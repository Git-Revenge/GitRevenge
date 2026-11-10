package git

import (
	"context"
	"strconv"
	"strings"
)

// A forecast for an interactive rebase as planned in the editor (issue #596): the commits in
// the plan's order, with the dropped ones left out.

// droppedFilesCap is the most files listed for one dropped commit.
const droppedFilesCap = 20

// DroppedCommit says what goes with a commit the plan drops.
type DroppedCommit struct {
	Commit string
	// Files are the files it changed, up to a cap; FileCount is how many in all.
	Files     []string
	FileCount int
	// Added and Deleted count its lines, leaving binary files out.
	Added, Deleted int
}

// PlanForecast is what running an interactive rebase plan would do.
type PlanForecast struct {
	// StopsAt is the first commit that wouldn't apply in this order, with the files in
	// conflict there, or "" when every commit tried applies.
	StopsAt   string
	Conflicts []ForecastConflict
	// Incomplete says not every commit could be tried: none of those that were conflicts.
	Incomplete bool
	// Pushed means some of the commits are already on the branch's upstream, so the rewritten
	// branch will need a force push. Upstream names it.
	Pushed   bool
	Upstream string
	Dropped  []DroppedCommit
}

// replay applies commits, oldest first, in memory on top of onto, each on the result of the
// one before, as a rebase does. It reports the first that doesn't apply and its conflicts.
// Nothing in the repository is touched: the commits it chains are loose objects.
func (r *Repo) replay(ctx context.Context, onto string, commits []string) (stopsAt string, conflicts []ForecastConflict, incomplete bool, err error) {
	tip, err := r.ReadString(ctx, "rev-parse", "--verify", onto+"^{commit}")
	if err != nil {
		return "", nil, false, err
	}
	for i, c := range commits {
		if i >= ForecastReplayCap {
			return "", nil, true, nil
		}
		tree, conflicted, err := r.mergeTree(ctx, c+"^", tip, c)
		if err != nil {
			return "", nil, true, nil
		}
		if len(conflicted) > 0 {
			return c, r.conflictsIn(ctx, tree, conflicted), false, nil
		}
		// The next commit applies on top of this result: give it a commit to stand on. It's a
		// loose object nothing points at, and git's own housekeeping removes it.
		next, err := r.runner.Output(ctx, Cmd{Dir: r.dir, Args: []string{"commit-tree", tree, "-p", tip, "-m", "forecast"}, Env: forecastIdentity})
		if err != nil {
			return "", nil, true, nil
		}
		tip = next
	}
	return "", nil, false, nil
}

// dropped reads what one commit changed, for the plan's account of what a drop throws away.
func (r *Repo) dropped(ctx context.Context, commit string) DroppedCommit {
	d := DroppedCommit{Commit: commit}
	out, err := r.Read(ctx, "show", "--numstat", "--format=", "-z", commit, "--")
	if err != nil {
		return d
	}
	// A rename's record has no path: its old and new names follow, each a record of its own.
	names := 0
	for rec := range strings.SplitSeq(string(out), "\x00") {
		f := strings.SplitN(strings.TrimLeft(rec, "\n"), "\t", 3)
		if len(f) != 3 {
			if names == 1 && len(d.Files) < droppedFilesCap {
				d.Files = append(d.Files, rec)
			}
			names--
			continue
		}
		d.FileCount++
		if f[2] == "" {
			names = 2
		} else if len(d.Files) < droppedFilesCap {
			d.Files = append(d.Files, f[2])
		}
		// Binary files count as "-".
		if n, err := strconv.Atoi(f[0]); err == nil {
			d.Added += n
		}
		if n, err := strconv.Atoi(f[1]); err == nil {
			d.Deleted += n
		}
	}
	return d
}

// ForecastPlan says what an interactive rebase of the commits after base would do, run as
// items plan it: in that order, without the dropped commits. A squash or a fixup changes the
// same files as a pick, so it's tried the same way.
func (r *Repo) ForecastPlan(ctx context.Context, base string, items []RebaseItem) (PlanForecast, error) {
	ctx, cancel := context.WithTimeout(ctx, ForecastTimeout)
	defer cancel()
	var f PlanForecast
	var order []string
	for _, it := range items {
		if it.Action == "drop" {
			f.Dropped = append(f.Dropped, r.dropped(ctx, it.Commit))
			continue
		}
		order = append(order, it.Commit)
	}

	if branch, err := r.ReadString(ctx, "symbolic-ref", "--short", "-q", "HEAD"); err == nil && branch != "" {
		if up, err := r.ReadString(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", branch+"@{upstream}"); err == nil && up != "" {
			f.Upstream = up
			rng := base + "..HEAD"
			f.Pushed = r.count(ctx, rng, "^"+up) < r.count(ctx, rng)
		}
	}

	var err error
	f.StopsAt, f.Conflicts, f.Incomplete, err = r.replay(ctx, base, order)
	if err != nil {
		// A base that can't be read, or a timeout: the plan is still worth showing.
		f.Incomplete = true
	}
	return f, nil
}
