package app

import (
	"context"
	"slices"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/git"
)

// Thresholds decide when a branch signal is a flag. Zero means the default.
type Thresholds struct {
	// Stale and LongLived are days; Behind is commits; Size is lines changed.
	Stale     int `json:"stale"`
	LongLived int `json:"longLived"`
	Behind    int `json:"behind"`
	Size      int `json:"size"`
}

// withDefaults fills unset thresholds: stale after 7 days, long-lived after 14, drifting
// past 50 commits behind, a big batch over 500 lines.
func (t Thresholds) withDefaults() Thresholds {
	if t.Stale <= 0 {
		t.Stale = 7
	}
	if t.LongLived <= 0 {
		t.LongLived = 14
	}
	if t.Behind <= 0 {
		t.Behind = 50
	}
	if t.Size <= 0 {
		t.Size = 500
	}
	return t
}

// Branch flags.
const (
	flagStale     = "stale"
	flagLongLived = "long-lived"
	flagDrifting  = "drifting"
	flagBigBatch  = "big-batch"
)

// BranchSignal is an open branch measured at the as-of point.
type BranchSignal struct {
	git.OpenBranch
	// StaleDays is whole days from the last commit to the as-of point; AgeDays from the
	// first commit not on the target.
	StaleDays int `json:"staleDays"`
	AgeDays   int `json:"ageDays"`
	// Flags lists the thresholds the branch is past, in a fixed order.
	Flags []string `json:"flags"`
}

// AttentionResult is the Needs attention view's data for one repository. Conflict checks
// arrive afterwards, as "insights:conflict" events.
type AttentionResult struct {
	Window     InsightsWindow `json:"window"`
	Thresholds Thresholds     `json:"thresholds"`
	// Branches are every open branch measured, most recently active first.
	Branches []BranchSignal `json:"branches"`
}

// ConflictResult is the payload of the "insights:conflict" event: one branch's dry-run
// merge. The SHAs say which states it describes, so a stale result can be dropped.
type ConflictResult struct {
	Repo      string `json:"repo"`
	TargetSHA string `json:"targetSha"`
	Branch    string `json:"branch"`
	SHA       string `json:"sha"`
	git.Conflict
}

// daysBetween is whole days from then to asOf, never negative.
func daysBetween(then, asOf int64) int {
	if then == 0 || asOf <= then {
		return 0
	}
	return int((asOf - then) / day)
}

// signal measures b against the thresholds.
func signal(b git.OpenBranch, asOf int64, t Thresholds) BranchSignal {
	s := BranchSignal{OpenBranch: b, StaleDays: daysBetween(b.LastCommit, asOf), AgeDays: daysBetween(b.FirstCommit, asOf), Flags: []string{}}
	if s.StaleDays > t.Stale {
		s.Flags = append(s.Flags, flagStale)
	}
	if s.AgeDays > t.LongLived {
		s.Flags = append(s.Flags, flagLongLived)
	}
	if s.Behind > t.Behind {
		s.Flags = append(s.Flags, flagDrifting)
	}
	if s.Added+s.Removed > t.Size {
		s.Flags = append(s.Flags, flagBigBatch)
	}
	return s
}

// conflictChecks tracks the background dry-run merges, so a newer request for a
// repository cancels the older one's.
type conflictChecks struct {
	mu     sync.Mutex
	cancel map[string]context.CancelFunc
}

func (c *conflictChecks) start(repo string) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel == nil {
		c.cancel = map[string]context.CancelFunc{}
	}
	if stop := c.cancel[repo]; stop != nil {
		stop()
	}
	ctx, stop := context.WithCancel(context.Background())
	c.cancel[repo] = stop
	return ctx
}

// stopAll cancels every repository's checks, as the app shuts down.
func (c *conflictChecks) stopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, stop := range c.cancel {
		stop()
	}
}

// conflictWorkers is how many dry-run merges run at once for one repository.
const conflictWorkers = 2

// InsightsBranches measures the repository's open branches against the target at the
// as-of point, then checks each for conflicts in the background, sending an
// "insights:conflict" event as each check finishes.
func (a *App) InsightsBranches(path string, in InsightsInputs) (AttentionResult, error) {
	ctx := context.Background()
	g, root, err := a.repoGit(ctx, path)
	if err != nil {
		return AttentionResult{}, err
	}
	res, err := cachedInsight(ctx, a, g, root, "branches", in, func() (AttentionResult, error) {
		rules := a.branchRules(root)
		w, err := resolveInputs(ctx, g, in, rules)
		if err != nil {
			return AttentionResult{}, err
		}
		open, err := g.OpenBranches(ctx, w.Target, git.MaxSignalBranches, endStates(rules))
		if err != nil {
			return AttentionResult{}, err
		}
		t := in.Thresholds.withDefaults()
		res := AttentionResult{Window: w, Thresholds: t, Branches: make([]BranchSignal, 0, len(open))}
		for _, b := range open {
			res.Branches = append(res.Branches, signal(b, w.AsOf, t))
		}
		return res, nil
	})
	if err != nil {
		return AttentionResult{}, err
	}
	a.checkConflicts(a.conflicts.start(root), g, root, res)
	return res, nil
}

// checkConflicts reports each branch's dry-run merge in the background: at once for pairs
// of commits checked before, otherwise as each check finishes.
func (a *App) checkConflicts(ctx context.Context, g *git.Repo, root string, res AttentionResult) {
	target := res.Window.TargetSHA
	report := func(b git.OpenBranch, c git.Conflict) {
		a.emit("insights:conflict", ConflictResult{Repo: root, TargetSHA: target, Branch: b.Name, SHA: b.SHA, Conflict: c})
	}
	var todo []git.OpenBranch
	var known []git.OpenBranch
	for _, s := range res.Branches {
		if _, ok := a.insights.conflict(target, s.SHA); ok {
			known = append(known, s.OpenBranch)
		} else {
			todo = append(todo, s.OpenBranch)
		}
	}
	jobs := make(chan git.OpenBranch)
	for range conflictWorkers {
		go func() {
			for b := range jobs {
				c, err := g.WouldConflict(ctx, target, b.SHA)
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					continue
				}
				a.insights.putConflict(target, b.SHA, c)
				report(b, c)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, b := range known {
			if c, ok := a.insights.conflict(target, b.SHA); ok && ctx.Err() == nil {
				report(b, c)
			}
		}
		for _, b := range todo {
			select {
			case jobs <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
}

// HotFile is a file the window's shipped items changed, and how many open branches
// change it too.
type HotFile struct {
	git.HotFile
	OpenBranches int `json:"openBranches"`
}

// HotspotsResult is the Hotspots view's data for one repository.
type HotspotsResult struct {
	Window InsightsWindow `json:"window"`
	// Files are the most changed files, most first.
	Files []HotFile `json:"files"`
	// Rework is the reverts and fixup commits among the shipped items.
	Rework []git.ShippedItem `json:"rework"`
}

// maxHotFiles caps the hotspots list.
const maxHotFiles = 20

// InsightsHotspots lists the files changed most in the window, flagging those that open
// branches change too, and the window's rework.
func (a *App) InsightsHotspots(path string, in InsightsInputs) (HotspotsResult, error) {
	ctx := context.Background()
	g, root, err := a.repoGit(ctx, path)
	if err != nil {
		return HotspotsResult{}, err
	}
	rules := a.branchRules(root)
	return cachedInsight(ctx, a, g, root, "hotspots", in, func() (HotspotsResult, error) { return hotspots(ctx, g, in, rules) })
}

// endStates leaves Docs and Release branches out of branch signals and conflict checks.
// They are end states (a published site, a release being maintained), expected to drift
// from the target, not working copies waiting to merge (ADR 0009).
func endStates(rules branchkind.Rules) func(string) bool {
	return func(name string) bool {
		k := rules.Kind(name)
		return k == branchkind.Docs || k == branchkind.Release
	}
}

func hotspots(ctx context.Context, g *git.Repo, in InsightsInputs, rules branchkind.Rules) (HotspotsResult, error) {
	w, err := resolveInputs(ctx, g, in, rules)
	if err != nil {
		return HotspotsResult{}, err
	}
	q := w.query()
	files, err := g.HotFiles(ctx, q)
	if err != nil {
		return HotspotsResult{}, err
	}
	open, err := g.OpenBranches(ctx, w.Target, git.MaxSignalBranches, endStates(rules))
	if err != nil {
		return HotspotsResult{}, err
	}
	items, err := g.Shipped(ctx, q)
	if err != nil {
		return HotspotsResult{}, err
	}
	res := HotspotsResult{Window: w, Files: []HotFile{}, Rework: []git.ShippedItem{}}
	for _, f := range files[:min(len(files), maxHotFiles)] {
		h := HotFile{HotFile: f}
		for _, b := range open {
			if _, found := slices.BinarySearch(b.Paths, f.Path); found {
				h.OpenBranches++
			}
		}
		res.Files = append(res.Files, h)
	}
	for _, it := range items {
		if it.Revert || it.Fixup {
			res.Rework = append(res.Rework, it)
		}
	}
	return res, nil
}
