package session

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
)

// DefaultReadAhead is how many rows the loader reads past the furthest row the UI has
// asked for before it pauses. Memory then grows with how far someone has scrolled, not
// with the size of the repository.
const DefaultReadAhead = 30000

// GraphProgress reports loading progress for a repository's commit graph.
type GraphProgress struct {
	Rows int
	Done bool
	// Paused means the reader stopped ahead of the viewport; scrolling further, jumping to
	// a commit or searching reads on.
	Paused bool
	// Estimate is about how many rows the whole history has (from git rev-list --count),
	// or 0 until it's known. Once Done, it equals Rows.
	Estimate int
	// Reset means a reloaded graph replaced the displayed one, so cached rows are stale.
	Reset bool
	// RefsReady is set on the update sent when refs finish loading during a first load,
	// so the UI can re-fetch rows to pick up their labels.
	RefsReady bool
	Err       error
}

// graphLoader owns a repository's commit graph and reloads it on demand. It streams
// `git log` into the graph and pauses the reader once it is ahead of what the UI needs;
// blocking the reader blocks git on its output pipe, so nothing more is read or parsed.
type graphLoader struct {
	repo       *git.Repo
	onProgress func(GraphProgress)
	ahead      int
	// rules returns the repository's current branch rules (hide patterns, kinds); nil means
	// the defaults.
	rules func() branchkind.Rules
	// stackOverrides returns the repository's stack parent overrides; nil skips stacks.
	stackOverrides func() map[string]string

	mu sync.Mutex
	// showHidden puts hidden branches back in the graph, for this session only.
	showHidden bool
	// focus narrows the graph to one branch or stack, for this session only.
	focus    Focus
	current  *graph.Graph
	refs     *RefIndex
	loading  bool
	paused   bool
	estimate int
	// want is the furthest row the UI has asked for.
	want int
	// unbounded counts callers that need the reader to keep going (finding a commit,
	// searching), so it doesn't pause while they wait.
	unbounded int
	// wake nudges a paused reader to check whether it may go on.
	wake chan struct{}
	// changed is closed and replaced whenever the graph advances, waking waiters.
	changed chan struct{}
	cancel  context.CancelFunc
	// wg tracks load goroutines so stop can wait for their git processes to exit.
	wg sync.WaitGroup
}

func newGraphLoader(repo *git.Repo, ahead int, onProgress func(GraphProgress)) *graphLoader {
	return &graphLoader{
		repo:       repo,
		onProgress: onProgress,
		ahead:      ahead,
		wake:       make(chan struct{}, 1),
		changed:    make(chan struct{}),
	}
}

// RefIndex holds a repository's refs, indexed by the commit they point at.
type RefIndex struct {
	All      []git.Ref
	ByCommit map[string][]git.Ref
}

func newRefIndex(refs []git.Ref) *RefIndex {
	idx := &RefIndex{All: refs, ByCommit: make(map[string][]git.Ref, len(refs))}
	for _, r := range refs {
		idx.ByCommit[r.Commit] = append(idx.ByCommit[r.Commit], r)
	}
	return idx
}

// Focus narrows the graph to one branch, or to the whole stack it's in: the opposite of hiding
// a branch. As with hiding, the name has no remote, so its remote-tracking copies come too. The
// zero Focus is no focus.
type Focus struct {
	Branch string `json:"branch"`
	Stack  bool   `json:"stack"`
}

// annotate sets each branch's kind and whether the rules hide it. Unless they're being
// shown, hidden branches stay in All (the sidebar counts them) but get no label in the
// graph. A focused branch (focused holds full ref names) always gets its label.
func (idx *RefIndex) annotate(rules branchkind.Rules, remotes []string, show bool, focused map[string]bool) {
	idx.ByCommit = make(map[string][]git.Ref, len(idx.All))
	for i, r := range idx.All {
		name := r.Short
		switch r.Kind {
		case git.RemoteBranch:
			name = branchkind.Strip(r.Short, remotes)
			fallthrough
		case git.LocalBranch:
			idx.All[i].Hidden = rules.Hidden(name)
			idx.All[i].BranchKind = string(rules.Kind(name))
		}
		idx.All[i].Focused = focused[r.Name]
		if !idx.All[i].Hidden || show || idx.All[i].Focused {
			idx.ByCommit[r.Commit] = append(idx.ByCommit[r.Commit], idx.All[i])
		}
	}
}

// SetShowHidden shows or hides the hidden branches again, and reloads the graph.
func (l *graphLoader) SetShowHidden(show bool) {
	l.mu.Lock()
	changed := l.showHidden != show
	l.showHidden = show
	l.mu.Unlock()
	if changed {
		l.reload()
	}
}

// ShowingHidden reports whether hidden branches are being shown.
func (l *graphLoader) ShowingHidden() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.showHidden
}

// SetFocus narrows the graph to focus, replacing any focus it had, and reloads it.
func (l *graphLoader) SetFocus(focus Focus) {
	l.mu.Lock()
	changed := l.focus != focus
	l.focus = focus
	l.mu.Unlock()
	if changed {
		l.reload()
	}
}

// Focus returns what the graph is focused on.
func (l *graphLoader) Focus() Focus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.focus
}

// focusedRefs returns the full names of the refs focus covers, which is none once its branch
// has gone.
func (l *graphLoader) focusedRefs(ctx context.Context, focus Focus, rules branchkind.Rules, remotes []string) (map[string]bool, error) {
	names := []string{focus.Branch}
	if focus.Stack && l.stackOverrides != nil {
		list, err := l.repo.Refs(ctx)
		if err != nil {
			return nil, err
		}
		names = StackMembers(focus.Branch, Stacks(ctx, l.repo, list, rules, l.stackOverrides()))
	}
	wanted := map[string]bool{}
	for _, n := range names {
		wanted["refs/heads/"+n] = true
		for _, rm := range remotes {
			wanted["refs/remotes/"+rm+"/"+n] = true
		}
	}
	args := []string{"for-each-ref", "--format=%(refname)"}
	for ref := range wanted {
		args = append(args, ref)
	}
	out, err := l.repo.ReadString(ctx, args...)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	// A pattern also matches what's under it, so refs/heads/a would bring in refs/heads/a/b.
	for ref := range strings.Lines(out) {
		if ref = strings.TrimSpace(ref); wanted[ref] {
			found[ref] = true
		}
	}
	return found, nil
}

// refResult is a load's refs, or the error reading them.
type refResult struct {
	idx *RefIndex
	err error
}

// Refs returns the refs matching the displayed graph.
func (l *graphLoader) Refs() *RefIndex {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.refs == nil {
		return newRefIndex(nil)
	}
	return l.refs
}

// Graph returns the graph to display and whether more of it may still load (it is reading
// or paused). The graph may be partial; it is nil only before the first load starts.
func (l *graphLoader) Graph() (*graph.Graph, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.current, l.loading
}

// Status reports whether the reader is paused ahead of the viewport, and the estimated
// size of the whole history (0 until known).
func (l *graphLoader) Status() (paused bool, estimate int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.paused, l.estimate
}

// Demand records that the UI wants rows up to row, so a paused reader reads on.
func (l *graphLoader) Demand(row int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if row > l.want {
		l.want = row
		l.nudgeLocked()
	}
}

// LoadUntil keeps the reader going past the read-ahead limit until found reports true for
// the displayed graph or the history ends, and returns that graph. A background reload
// keeps the old graph displayed, so found may be satisfied by it.
func (l *graphLoader) LoadUntil(ctx context.Context, found func(*graph.Graph) bool) (*graph.Graph, error) {
	l.mu.Lock()
	l.unbounded++
	l.nudgeLocked()
	defer func() {
		l.mu.Lock()
		l.unbounded--
		l.mu.Unlock()
	}()
	for {
		g, loading, changed := l.current, l.loading, l.changed
		if g != nil && (!loading || found(g)) {
			l.mu.Unlock()
			return g, nil
		}
		l.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		l.mu.Lock()
	}
}

// nudgeLocked wakes a paused reader, if there is one.
func (l *graphLoader) nudgeLocked() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// notifyLocked wakes everything waiting in LoadUntil.
func (l *graphLoader) notifyLocked() {
	close(l.changed)
	l.changed = make(chan struct{})
}

func (l *graphLoader) report(p GraphProgress) {
	if l.onProgress != nil {
		l.onProgress(p)
	}
}

// reload starts loading the graph, cancelling any load in flight. The first load streams
// into the visible graph so the first screen appears quickly; later loads build a fresh
// graph off-screen and swap it in once it reaches the read-ahead limit or ends, so a
// refresh never blanks the view.
func (l *graphLoader) reload() {
	l.mu.Lock()
	if l.cancel != nil {
		l.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	g := graph.New()
	if l.current == nil {
		l.current = g
	}
	l.loading = true
	show := l.showHidden
	focus := l.focus
	l.mu.Unlock()
	rules := branchkind.New(nil, nil)
	if l.rules != nil {
		rules = l.rules()
	}

	l.wg.Go(func() {
		var remotes []string
		if rs, err := l.repo.Remotes(ctx); err == nil {
			for _, r := range rs {
				remotes = append(remotes, r.Name)
			}
		}

		var focused map[string]bool
		if focus != (Focus{}) {
			var err error
			focused, err = l.focusedRefs(ctx, focus, rules, remotes)
			if ctx.Err() != nil {
				return // superseded or closed; the newer load reports
			}
			if err == nil && len(focused) == 0 {
				// Its branch has gone, so there's nothing left to focus on.
				l.mu.Lock()
				if l.focus == focus {
					l.focus = Focus{}
				}
				l.mu.Unlock()
			}
		}

		// Read refs alongside the log rather than before it: with thousands of refs they can
		// take longer than the first screen of history.
		refsCh := make(chan refResult, 1)
		go func() {
			list, err := l.repo.Refs(ctx)
			if err == nil && l.stackOverrides != nil {
				annotateStacks(list, Stacks(ctx, l.repo, list, rules, l.stackOverrides()))
			}
			idx := newRefIndex(list)
			idx.annotate(rules, remotes, show, focused)
			first := false
			l.mu.Lock()
			if l.refs == nil && ctx.Err() == nil {
				l.refs = idx
				first = true
			}
			l.mu.Unlock()
			if first {
				l.report(GraphProgress{Rows: g.Len(), RefsReady: true})
			}
			refsCh <- refResult{idx, err}
		}()
		var refs *refResult
		awaitRefs := func() refResult {
			if refs == nil {
				r := <-refsCh
				refs = &r
			}
			return *refs
		}

		// Stashes appear as single rows on their base commit. A stash commit's other parents
		// (its index and untracked-files snapshots) are internal: drop those edges and skip
		// the commits, which date order (children before parents) always yields after the stash itself.
		stashIDs, _ := l.repo.StashIDs(ctx)
		stashes := map[string]bool{}
		for _, id := range stashIDs {
			stashes[id] = true
		}
		hidden := map[string]bool{}
		pins, kinds := KindTips(ctx, l.repo, rules, remotes, show)
		g.Pin(pins)
		g.SetTipKinds(kinds)
		opts := git.LogOptions{Extra: stashIDs}
		if len(focused) > 0 {
			// Only what the focus reaches: not HEAD, tags or stashes.
			opts = git.LogOptions{}
			for ref := range focused {
				opts.Revs = append(opts.Revs, ref)
			}
			slices.Sort(opts.Revs)
		} else if !show {
			opts.Exclude, opts.Remotes = rules.Hide(), remotes
		}

		// The size of the whole history, for the scrollbar while the reader is paused. It
		// slightly overcounts (stash snapshots are hidden), so the UI calls it approximate.
		var counted atomic.Int64
		go func() {
			n, err := l.repo.CountCommits(ctx, opts)
			if err != nil || ctx.Err() != nil {
				return
			}
			counted.Store(int64(n))
			l.mu.Lock()
			shown := l.current == g && l.loading
			if shown {
				l.estimate = n
			}
			paused := l.paused
			l.mu.Unlock()
			if shown {
				l.report(GraphProgress{Rows: g.Len(), Paused: paused, Estimate: n})
			}
		}()

		// swapLocked shows g in place of the graph it is reloading. The caller holds l.mu.
		swapLocked := func(r refResult) {
			l.current = g
			l.refs = r.idx
			l.estimate = int(counted.Load())
			l.notifyLocked()
		}

		const every = 100 * time.Millisecond
		last := time.Now()
		err := l.repo.Log(ctx, opts, func(c git.LogCommit) error {
			if hidden[c.ID] {
				return nil
			}
			if stashes[c.ID] && len(c.Parents) > 1 {
				for _, p := range c.Parents[1:] {
					hidden[p] = true
				}
				c.Parents = c.Parents[:1]
			}
			if err := g.Add(c); err != nil {
				return err
			}
			n := g.Len()
			if time.Since(last) >= every {
				last = time.Now()
				l.mu.Lock()
				shown := l.current == g
				if shown {
					l.notifyLocked()
				}
				est := l.estimate
				l.mu.Unlock()
				if shown {
					l.report(GraphProgress{Rows: n, Estimate: est})
				}
			}
			return l.pauseIfAhead(ctx, g, n, awaitRefs, swapLocked)
		})
		r := awaitRefs()
		if ctx.Err() != nil {
			return // superseded or closed; the newer load reports
		}
		if err == nil {
			err = r.err
		}
		l.mu.Lock()
		l.current = g
		l.refs = r.idx
		l.loading = false
		l.paused = false
		l.estimate = g.Len()
		l.cancel = nil
		l.notifyLocked()
		l.mu.Unlock()
		l.report(GraphProgress{Rows: g.Len(), Done: true, Estimate: g.Len(), Err: err})
	})
}

// pauseIfAhead blocks the reader while g has n rows and is far enough ahead of what the
// UI and any waiters need. An off-screen reload that gets this far is swapped in first.
func (l *graphLoader) pauseIfAhead(ctx context.Context, g *graph.Graph, n int, awaitRefs func() refResult, swapLocked func(refResult)) error {
	for {
		l.mu.Lock()
		if l.unbounded > 0 || n < l.want+l.ahead {
			l.mu.Unlock()
			return nil
		}
		reset := false
		if l.current != g {
			l.mu.Unlock()
			r := awaitRefs() // before taking the lock: the refs goroutine needs it
			l.mu.Lock()
			swapLocked(r)
			reset = true
		}
		l.paused = true
		l.notifyLocked()
		est := l.estimate
		l.mu.Unlock()
		l.report(GraphProgress{Rows: n, Paused: true, Estimate: est, Reset: reset})
		select {
		case <-l.wake:
		case <-ctx.Done():
			return ctx.Err()
		}
		l.mu.Lock()
		l.paused = false
		l.mu.Unlock()
	}
}

// stop cancels any load and waits for it to finish, so no git process is left holding
// files in the repository (on Windows that would block deleting or moving it). Anything
// waiting in LoadUntil is released.
func (l *graphLoader) stop() {
	l.mu.Lock()
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	l.mu.Unlock()
	l.wg.Wait()
	l.mu.Lock()
	l.loading = false
	l.paused = false
	l.notifyLocked()
	l.mu.Unlock()
}
