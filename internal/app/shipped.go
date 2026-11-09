package app

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/git"
)

// InsightsInputs are the choices an insights result depends on, besides the repository
// itself (ADR-0008). The same inputs on the same repository state give the same result.
type InsightsInputs struct {
	// Target is the branch to measure against, or "" to detect it (origin/HEAD, main, master).
	Target string `json:"target"`
	// Window is "7d", "14d", "month", "custom" or "releases".
	Window string `json:"window"`
	// Since and Until bound a custom window, Unix seconds, inclusive.
	Since int64 `json:"since"`
	Until int64 `json:"until"`
	// From and To are the tags of a releases window.
	From string `json:"from"`
	To   string `json:"to"`
	// AsOf is the point windows and ages are measured from, Unix seconds, or 0 for the
	// target's tip commit.
	AsOf int64 `json:"asOf"`
	// Thresholds decide which branch signals are flags.
	Thresholds Thresholds `json:"thresholds"`
}

// InsightsWindow is what inputs resolved to, printed with every result so it can be
// reproduced.
type InsightsWindow struct {
	Target    string `json:"target"`
	TargetSHA string `json:"targetSha"`
	AsOf      int64  `json:"asOf"`
	// Since and Until are the window's absolute bounds, Unix seconds (UTC), inclusive. For
	// a releases window they are the two tags' commit dates.
	Since int64 `json:"since"`
	Until int64 `json:"until"`
	// Range is "from..to" for a releases window, else "".
	Range string `json:"range"`
	// Points is the commit range a releases window measures, "sha..sha": the tags' commits,
	// or for a Release branch, where it forked from the target.
	Points string `json:"points,omitempty"`
	// Kinds are the branch kind patterns in use, an input since they decide which branches
	// are Docs (left out) and which items are Bot (grouped) (ADR 0009).
	Kinds map[string][]string `json:"kinds"`
	// References are the smart link rules that find ticket references, "name pattern" each,
	// or empty for the built-in PROJ-123 and #123. They decide how Shipped groups.
	References []string `json:"references"`
}

// query is the commit selection the window stands for.
func (w InsightsWindow) query() git.ShippedQuery {
	q := git.ShippedQuery{Target: w.Target, Since: w.Since, Until: w.Until, Range: w.Range}
	if w.Points != "" {
		q.Range = w.Points
	}
	return q
}

// ShippedResult is the Shipped view's data for one repository.
type ShippedResult struct {
	Window InsightsWindow    `json:"window"`
	Items  []git.ShippedItem `json:"items"`
}

const day = int64(24 * 60 * 60)

// resolveWindow turns inputs into absolute bounds measured from asOf. Day windows cover
// whole UTC days ending with asOf's day; a month runs from the 1st of asOf's UTC month.
func resolveWindow(in InsightsInputs, asOf int64) (since, until int64, err error) {
	startOfDay := asOf - ((asOf%day)+day)%day
	switch in.Window {
	case "7d", "":
		return startOfDay - 6*day, asOf, nil
	case "14d":
		return startOfDay - 13*day, asOf, nil
	case "month":
		t := time.Unix(asOf, 0).UTC()
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Unix(), asOf, nil
	case "custom":
		if in.Until < in.Since {
			return 0, 0, userError("The start of the window must be before its end.")
		}
		return in.Since, in.Until, nil
	}
	return 0, 0, userError("Unknown window " + in.Window + ".")
}

// resolveInputs fixes the target, as-of point and window for one repository.
func resolveInputs(ctx context.Context, g *git.Repo, in InsightsInputs, rules branchkind.Rules) (InsightsWindow, error) {
	target := in.Target
	if target == "" {
		t, err := g.InsightsTarget(ctx)
		if errors.Is(err, git.ErrNoTarget) {
			return InsightsWindow{}, userError("This repository has no origin/HEAD, main or master branch to measure against. Choose a branch.")
		}
		if err != nil {
			return InsightsWindow{}, err
		}
		target = t
	}
	tip, err := g.Tip(ctx, target)
	if err != nil {
		return InsightsWindow{}, userError("There's no branch named " + target + ".")
	}
	w := InsightsWindow{Target: target, TargetSHA: tip.SHA, AsOf: in.AsOf, Kinds: rules.KindPatterns()}
	if w.AsOf == 0 {
		w.AsOf = tip.Time
	}
	if in.Window == "releases" {
		if in.From == "" || in.To == "" {
			return InsightsWindow{}, userError("Choose the two releases to compare.")
		}
		from, errF := releasePoint(ctx, g, in.From, target, rules)
		to, errT := releasePoint(ctx, g, in.To, target, rules)
		if errF != nil || errT != nil {
			return InsightsWindow{}, userError("One of those releases no longer exists.")
		}
		w.Range, w.Since, w.Until = in.From+".."+in.To, from.Time, to.Time
		w.Points = from.SHA + ".." + to.SHA
		if in.AsOf == 0 {
			w.AsOf = to.Time
		}
		return w, nil
	}
	w.Since, w.Until, err = resolveWindow(in, w.AsOf)
	return w, err
}

// releasePoint is where a release sits on the target: a tag's commit, or for a Release
// branch (ADR 0009) the point where it forked from the target.
func releasePoint(ctx context.Context, g *git.Repo, name, target string, rules branchkind.Rules) (git.TipInfo, error) {
	if t, err := g.Tip(ctx, "refs/tags/"+name); err == nil {
		return t, nil
	}
	if rules.Kind(branchName(name)) == branchkind.Release {
		return g.ForkPoint(ctx, name, target)
	}
	return git.TipInfo{}, errors.New("not a release: " + name)
}

// branchName drops the remote from a remote-tracking branch name ("origin/release/1.4" to
// "release/1.4"); a local name stays as it is.
func branchName(name string) string {
	if rm, rest, ok := strings.Cut(name, "/"); ok && rm == "origin" {
		return rest
	}
	return name
}

// mergedBranch reads the branch a merge commit's subject names: GitHub's "Merge pull request
// #12 from owner/dependabot/npm/x", or git's "Merge branch 'x'" and "Merge remote-tracking
// branch 'origin/x'".
var mergedBranch = regexp.MustCompile(`^Merge (?:pull request #\d+ from [^/\s]+/(\S+)|(?:remote-tracking )?branch '(?:origin/)?([^']+)')`)

// markBots flags items a bot opened: the author is a GitHub App ("dependabot[bot]"), or the
// branch a merge brought in matches a Bot pattern.
func markBots(items []git.ShippedItem, rules branchkind.Rules) {
	for i, it := range items {
		bot := strings.HasSuffix(it.Author, "[bot]")
		if m := mergedBranch.FindStringSubmatch(it.Title); m != nil && !bot {
			bot = rules.Kind(m[1]+m[2]) == branchkind.Bot
		}
		items[i].Bot = bot
	}
}

// InsightsShipped lists what landed on the target branch in the window: its first-parent
// commits, with authors, sizes, ticket references and, for merges, cycle times.
func (a *App) InsightsShipped(path string, in InsightsInputs) (ShippedResult, error) {
	ctx := context.Background()
	g, root, err := a.repoGit(ctx, path)
	if err != nil {
		return ShippedResult{}, err
	}
	return cachedInsight(ctx, a, g, root, "shipped", in, func() (ShippedResult, error) {
		rules := a.branchRules(root)
		w, err := resolveInputs(ctx, g, in, rules)
		if err != nil {
			return ShippedResult{}, err
		}
		q := w.query()
		lc := a.linkConfig(ctx, g)
		q.Match = lc.References()
		w.References = lc.Patterns()
		items, err := g.Shipped(ctx, q)
		if err != nil {
			return ShippedResult{}, err
		}
		markBots(items, rules)
		return ShippedResult{Window: w, Items: items}, nil
	})
}

// ReleaseBranch is a Release-kind branch offered as a releases-window boundary.
type ReleaseBranch struct {
	Name string `json:"name"`
	// ForkTime is when it forked from the target branch, Unix seconds.
	ForkTime int64 `json:"forkTime"`
}

// InsightsReleaseBranches lists the Release branches (ADR 0009), newest fork first, for
// choosing a releases window alongside tags. target "" detects it, as Insights does.
func (a *App) InsightsReleaseBranches(path, target string) ([]ReleaseBranch, error) {
	ctx := context.Background()
	g, root, err := a.repoGit(ctx, path)
	if err != nil {
		return nil, err
	}
	if target == "" {
		if target, err = g.InsightsTarget(ctx); err != nil {
			return []ReleaseBranch{}, nil
		}
	}
	rules := a.branchRules(root)
	refs, err := g.Refs(ctx)
	if err != nil {
		return nil, err
	}
	out := []ReleaseBranch{}
	seen := map[string]bool{}
	for _, r := range refs {
		if r.Kind != git.LocalBranch && r.Kind != git.RemoteBranch {
			continue
		}
		name := r.Short
		if r.Kind == git.RemoteBranch {
			name = branchName(r.Short)
		}
		if seen[name] || rules.Kind(name) != branchkind.Release {
			continue
		}
		fork, err := g.ForkPoint(ctx, r.Short, target)
		if err != nil {
			continue
		}
		seen[name] = true
		out = append(out, ReleaseBranch{Name: r.Short, ForkTime: fork.Time})
	}
	slices.SortFunc(out, func(x, y ReleaseBranch) int {
		if x.ForkTime != y.ForkTime {
			return int(y.ForkTime - x.ForkTime)
		}
		return strings.Compare(x.Name, y.Name)
	})
	return out, nil
}

// InsightsTags lists a repository's tags newest first, for choosing a releases window.
func (a *App) InsightsTags(path string) ([]git.TagInfo, error) {
	ctx := context.Background()
	g, _, err := a.repoGit(ctx, path)
	if err != nil {
		return nil, err
	}
	return g.Tags(ctx)
}
