package app

import (
	"context"
	"slices"
	"time"

	"github.com/risadams/gitrevenge/internal/branchkind"
	"github.com/risadams/gitrevenge/internal/git"
	"github.com/risadams/gitrevenge/internal/session"
)

// branchMergeTimeout bounds the check when deleting a branch. An old branch on a huge
// history can take a while, and asking first is a fine answer when it does.
const branchMergeTimeout = 5 * time.Second

// maxMergeTargets is how many branches a branch is compared with.
const maxMergeTargets = 6

// BranchMergeState says whether a local branch's work is already on a Trunk branch or the
// checked-out branch, so deleting it needn't ask when it is.
func (a *App) BranchMergeState(path, name string) (git.BranchMerge, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.BranchMerge{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), branchMergeTimeout)
	defer cancel()
	refs, err := r.Git.Refs(ctx)
	if err != nil {
		return git.BranchMerge{}, err
	}
	remotes, err := r.Git.Remotes(ctx)
	if err != nil {
		return git.BranchMerge{}, err
	}
	names := make([]string, len(remotes))
	for i, rm := range remotes {
		names[i] = rm.Name
	}
	return r.Git.BranchMergeState(ctx, name, mergeTargets(refs, a.branchRules(path), names, name)), nil
}

// mergedBranchesTimeout bounds checking every branch that needs care.
const mergedBranchesTimeout = 30 * time.Second

// MergedBranches says which branches that need care already have their work on a Trunk branch
// or the checked-out branch, by branch, with the branch holding it.
func (a *App) MergedBranches(path string) (map[string]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mergedBranchesTimeout)
	defer cancel()
	merged, err := a.mergedBranches(ctx, r)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(merged))
	for name, state := range merged {
		out[name] = state.Target
	}
	return out, nil
}

// mergeTargetsFor lists what to compare branches with, in general rather than for one branch.
func (a *App) mergeTargetsFor(ctx context.Context, r *session.Repo) ([]string, error) {
	remotes, err := r.Git.Remotes(ctx)
	if err != nil {
		return nil, err
	}
	remoteNames := make([]string, len(remotes))
	for i, rm := range remotes {
		remoteNames[i] = rm.Name
	}
	return mergeTargets(r.Refs().All, a.branchRules(r.Path), remoteNames, ""), nil
}

func (a *App) mergedBranches(ctx context.Context, r *session.Repo) (map[string]git.BranchMerge, error) {
	refs := r.Refs().All
	targets, err := a.mergeTargetsFor(ctx, r)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, ref := range refs {
		if ref.Kind == git.LocalBranch && (ref.Gone || ref.StackParentMerged) && !slices.Contains(targets, ref.Short) {
			names = append(names, ref.Short)
		}
	}
	return r.Git.MergedBranches(ctx, names, targets), nil
}

// mergeTargets lists what to compare a branch with: local Trunk branches, then their remote
// counterparts (a squash merged on the server reaches those first), then the checked-out
// branch.
func mergeTargets(refs []git.Ref, rules branchkind.Rules, remotes []string, name string) []string {
	var local, remote, current []string
	for _, r := range refs {
		switch r.Kind {
		case git.LocalBranch:
			if r.Short == name {
				continue
			}
			if rules.Kind(r.Short) == branchkind.Trunk {
				local = append(local, r.Short)
			}
			if r.Head {
				current = append(current, r.Short)
			}
		case git.RemoteBranch:
			if rules.Kind(branchkind.Strip(r.Short, remotes)) == branchkind.Trunk {
				remote = append(remote, r.Short)
			}
		}
	}
	var out []string
	for _, t := range slices.Concat(local, remote, current) {
		if !slices.Contains(out, t) && len(out) < maxMergeTargets {
			out = append(out, t)
		}
	}
	return out
}

// DeletableBranches narrows merged branches to those Delete all may take: not checked out here
// or in a worktree, not hidden, not a protected kind, and with nothing left that builds on them.
func (a *App) DeletableBranches(path string, names []string) ([]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	return a.deletableBranches(context.Background(), r, names), nil
}

func (a *App) deletableBranches(ctx context.Context, r *session.Repo, names []string) []string {
	refs := r.Refs().All
	busy := map[string]bool{}
	if wts, err := r.Git.Worktrees(ctx); err == nil {
		for _, w := range wts {
			busy[w.Branch] = true
		}
	}
	rules := a.branchRules(r.Path)
	safe := map[string]bool{}
	for _, ref := range refs {
		if ref.Kind == git.LocalBranch && slices.Contains(names, ref.Short) && !ref.Head && !ref.Hidden && !busy[ref.Short] && !protectedKinds[rules.Kind(ref.Short)] {
			safe[ref.Short] = true
		}
	}
	// A branch stays while one that stays builds on it, which can run all the way down a stack.
	for changed := true; changed; {
		changed = false
		for _, ref := range refs {
			if ref.Kind == git.LocalBranch && safe[ref.StackParent] && !safe[ref.Short] {
				delete(safe, ref.StackParent)
				changed = true
			}
		}
	}
	return slices.DeleteFunc(slices.Clone(names), func(name string) bool { return !safe[name] })
}

// DeleteMergedBranches deletes those of names that are still merged and safe to delete, as one
// step to undo. What the list showed may be out of date by now, so it checks again first. It
// returns the names of those it deleted.
func (a *App) DeleteMergedBranches(path string, names []string) ([]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mergedBranchesTimeout)
	defer cancel()
	merged, err := a.mergedBranches(ctx, r)
	if err != nil {
		return nil, err
	}
	still := slices.DeleteFunc(slices.Clone(names), func(name string) bool {
		_, ok := merged[name]
		return !ok
	})
	safe := a.deletableBranches(ctx, r, still)
	refs, err := a.currentRefs(ctx, r, safe)
	if err != nil {
		return nil, err
	}
	gone := a.deleteMerged(ctx, r, refs)
	if gone == nil {
		return []string{}, nil
	}
	r.ReloadGraph()
	return gone, nil
}
