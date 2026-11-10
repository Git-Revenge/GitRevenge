package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Forecasts say what a merge or a rebase would do before it's run (issue #594). They are
// worked out with `git merge-tree`, which merges in memory: the working tree, the index and
// the branches are never touched. A forecast is advice. The operation itself still decides.

// ForecastConflict is a file a merge or rebase would stop on.
type ForecastConflict struct {
	Path string
	// Places is how many separate conflicts the file would have; 0 when it can't be counted,
	// such as a binary file or one deleted on one side and changed on the other.
	Places int
}

// ForecastCommit is a commit that would be merged in or written again.
type ForecastCommit struct {
	ID      string
	Subject string
	Author  string
	// FixupOf is the commit in the same list that this one is a fixup! or squash! for, or "".
	FixupOf string
}

// Caps on how much a forecast looks at, so it stays quick on large branches.
const (
	// ForecastListCap is the most commits a forecast lists.
	ForecastListCap = 200
	// ForecastReplayCap is the most commits a rebase forecast tries one by one.
	ForecastReplayCap = 100
	// ForecastTimeout bounds a whole forecast.
	ForecastTimeout = 10 * time.Second
	// forecastConflictCap is the most conflicted files whose places are counted.
	forecastConflictCap = 50
)

// MergeForecast is what merging one commit into another would do.
type MergeForecast struct {
	// AlreadyMerged means there is nothing to merge: ours already has everything in theirs.
	AlreadyMerged bool
	// FastForward means ours has nothing theirs doesn't, so ours can simply move to theirs.
	FastForward bool
	// Commits are the commits that would come in, newest first, up to ForecastListCap;
	// CommitCount is how many there are in all.
	Commits     []ForecastCommit
	CommitCount int
	// Files is how many files the merge would change in ours.
	Files     int
	Conflicts []ForecastConflict
	// Incomplete says the conflicts couldn't be worked out, and Conflicts means nothing.
	Incomplete bool
}

// RebaseForecast is what rebasing a branch onto another commit would do.
type RebaseForecast struct {
	// Commits are the commits that would be written again, oldest first, as git applies
	// them, up to ForecastListCap; CommitCount is how many there are in all.
	Commits     []ForecastCommit
	CommitCount int
	// UpToDate means the branch is already on top of onto: there is nothing to do.
	UpToDate bool
	// StopsAt is the commit the rebase would first stop on, with the files in conflict
	// there, or "" when every commit tried applies cleanly.
	StopsAt   string
	Conflicts []ForecastConflict
	// Incomplete says not every commit could be tried (too many, or it took too long): no
	// conflict was found in those that were.
	Incomplete bool
	// Pushed means some of these commits are already on the branch's upstream, so the
	// rebased branch will need a force push. Upstream names it.
	Pushed   bool
	Upstream string
}

// mergeTree merges two commits in memory. It returns the resulting tree, the conflicted
// paths, and whether git could do it at all. base, when set, is used as the merge base in
// place of the commits' own.
func (r *Repo) mergeTree(ctx context.Context, base, ours, theirs string) (tree string, conflicted []string, err error) {
	args := []string{"merge-tree", "--write-tree", "--name-only", "-z", "--no-messages"}
	if base != "" {
		args = append(args, "--merge-base="+base)
	}
	out, err := r.Read(ctx, append(args, ours, theirs)...)
	var gerr *Error
	// Exit 1 is a merge with conflicts: the output is still the tree and the paths.
	if err != nil && (!errors.As(err, &gerr) || gerr.ExitCode != 1) {
		return "", nil, err
	}
	parts := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return "", nil, fmt.Errorf("merge-tree gave no tree")
	}
	seen := map[string]bool{}
	for _, p := range parts[1:] {
		if p != "" && !seen[p] {
			seen[p] = true
			conflicted = append(conflicted, p)
		}
	}
	return parts[0], conflicted, nil
}

// conflictsIn counts the conflicts in each conflicted file of a merged tree, by the conflict
// markers git left in it.
func (r *Repo) conflictsIn(ctx context.Context, tree string, paths []string) []ForecastConflict {
	out := make([]ForecastConflict, 0, len(paths))
	for i, p := range paths {
		c := ForecastConflict{Path: p}
		if i < forecastConflictCap {
			if b, err := r.Read(ctx, "show", tree+":"+p); err == nil {
				for line := range strings.SplitSeq(string(b), "\n") {
					if strings.HasPrefix(line, "<<<<<<< ") {
						c.Places++
					}
				}
			}
		}
		out = append(out, c)
	}
	return out
}

// forecastCommits lists the commits in a range with their subjects and authors, in the
// order git log gives them.
func (r *Repo) forecastCommits(ctx context.Context, extra []string, revRange string) ([]ForecastCommit, error) {
	args := append([]string{"log", "--format=%H%x00%s%x00%an", fmt.Sprintf("--max-count=%d", ForecastListCap)}, extra...)
	out, err := r.Read(ctx, append(args, revRange)...)
	if err != nil {
		return nil, err
	}
	var list []ForecastCommit
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.SplitN(line, "\x00", 3)
		if len(f) == 3 {
			list = append(list, ForecastCommit{ID: f[0], Subject: f[1], Author: f[2]})
		}
	}
	return list, nil
}

func (r *Repo) count(ctx context.Context, args ...string) int {
	out, err := r.ReadString(ctx, append([]string{"rev-list", "--count"}, args...)...)
	if err != nil {
		return 0
	}
	var n int
	_, _ = fmt.Sscan(out, &n)
	return n
}

// ForecastMerge says what merging theirs into ours would do.
func (r *Repo) ForecastMerge(ctx context.Context, ours, theirs string) (MergeForecast, error) {
	ctx, cancel := context.WithTimeout(ctx, ForecastTimeout)
	defer cancel()
	var f MergeForecast
	if r.isAncestor(ctx, theirs, ours) {
		f.AlreadyMerged = true
		return f, nil
	}
	f.FastForward = r.isAncestor(ctx, ours, theirs)
	f.CommitCount = r.count(ctx, ours+".."+theirs)
	var err error
	if f.Commits, err = r.forecastCommits(ctx, nil, ours+".."+theirs); err != nil {
		return f, err
	}
	// What the merge brings to ours: the changes on theirs since the two parted.
	if out, err := r.Read(ctx, "diff", "--name-only", "-z", ours+"..."+theirs); err == nil {
		f.Files = strings.Count(string(out), "\x00")
	}
	if f.FastForward {
		return f, nil
	}
	tree, conflicted, err := r.mergeTree(ctx, "", ours, theirs)
	if err != nil {
		// Unrelated histories, a timeout, anything git couldn't merge in memory: say the
		// forecast isn't known rather than fail the preview.
		f.Incomplete = true
		return f, nil
	}
	f.Conflicts = r.conflictsIn(ctx, tree, conflicted)
	return f, nil
}

// fixupTarget returns the subject a "fixup! x" or "squash! x" commit is for, or "".
func fixupTarget(subject string) string {
	for _, p := range []string{"fixup! ", "squash! ", "amend! "} {
		if rest, ok := strings.CutPrefix(subject, p); ok {
			// A fixup of a fixup names the original.
			for {
				inner := fixupTarget(rest)
				if inner == "" {
					return rest
				}
				rest = inner
			}
		}
	}
	return ""
}

// markFixups sets FixupOf on each fixup!, squash! or amend! commit whose target is earlier in
// the list, which is oldest first.
func markFixups(commits []ForecastCommit) {
	for i := range commits {
		target := fixupTarget(commits[i].Subject)
		if target == "" {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if fixupTarget(commits[j].Subject) == "" && (commits[j].Subject == target || strings.HasPrefix(commits[j].ID, target)) {
				commits[i].FixupOf = commits[j].ID
				break
			}
		}
	}
}

// forecastIdentity lets the in-memory commits a rebase forecast chains together be written
// whatever the user's identity is, or isn't. They're never referenced by anything.
var forecastIdentity = []string{
	"GIT_AUTHOR_NAME=Git Revenge", "GIT_AUTHOR_EMAIL=forecast@gitrevenge.invalid",
	"GIT_COMMITTER_NAME=Git Revenge", "GIT_COMMITTER_EMAIL=forecast@gitrevenge.invalid",
}

// ForecastRebase says what rebasing branch onto another commit would do. It replays each of
// the branch's commits in memory on top of the one before, as a rebase would, and reports
// the first that doesn't apply. Merge commits are left out, as a plain rebase leaves them.
func (r *Repo) ForecastRebase(ctx context.Context, branch, onto string) (RebaseForecast, error) {
	ctx, cancel := context.WithTimeout(ctx, ForecastTimeout)
	defer cancel()
	var f RebaseForecast
	rng := onto + ".." + branch
	f.CommitCount = r.count(ctx, "--no-merges", rng)
	if f.CommitCount == 0 {
		f.UpToDate = true
		return f, nil
	}
	var err error
	if f.Commits, err = r.forecastCommits(ctx, []string{"--reverse", "--no-merges"}, rng); err != nil {
		return f, err
	}
	// --max-count with --reverse keeps the newest N; the list is still oldest first.
	markFixups(f.Commits)

	// Already on top of onto: every commit would be written back exactly as it is.
	f.UpToDate = r.isAncestor(ctx, onto, branch)

	if up, err := r.ReadString(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", branch+"@{upstream}"); err == nil && up != "" {
		f.Upstream = up
		// Some of the commits to be rewritten are already on the upstream.
		f.Pushed = r.count(ctx, "--no-merges", rng, "^"+up) < f.CommitCount
	}
	if f.UpToDate {
		return f, nil
	}

	if f.CommitCount > len(f.Commits) || len(f.Commits) > ForecastReplayCap {
		f.Incomplete = true
	}
	tip, err := r.ReadString(ctx, "rev-parse", "--verify", onto+"^{commit}")
	if err != nil {
		return f, err
	}
	for i, c := range f.Commits {
		if i >= ForecastReplayCap {
			break
		}
		tree, conflicted, err := r.mergeTree(ctx, c.ID+"^", tip, c.ID)
		if err != nil {
			f.Incomplete = true
			return f, nil
		}
		if len(conflicted) > 0 {
			f.StopsAt = c.ID
			f.Conflicts = r.conflictsIn(ctx, tree, conflicted)
			return f, nil
		}
		// The next commit applies on top of this result: give it a commit to stand on. It's a
		// loose object nothing points at, and git's own housekeeping removes it.
		next, err := r.runner.Output(ctx, Cmd{Dir: r.dir, Args: []string{"commit-tree", tree, "-p", tip, "-m", "forecast"}, Env: forecastIdentity})
		if err != nil {
			f.Incomplete = true
			return f, nil
		}
		tip = next
	}
	return f, nil
}
