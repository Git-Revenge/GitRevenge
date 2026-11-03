package app

import (
	"context"
	"slices"
	"time"

	"github.com/risadams/gitrevenge/internal/branchkind"
	"github.com/risadams/gitrevenge/internal/git"
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
