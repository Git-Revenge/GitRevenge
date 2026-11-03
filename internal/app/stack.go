package app

import (
	"context"
	"slices"

	"github.com/risadams/gitrevenge/internal/git"
	"github.com/risadams/gitrevenge/internal/session"
)

// UnmergedBranch is a branch whose work isn't on any target, and so only it holds.
type UnmergedBranch struct {
	Name string `json:"name"`
	// Commits is how many of its commits aren't merged, or 0 if that isn't known.
	Commits int `json:"commits"`
}

// StackPlan is what deleting a stack would do.
type StackPlan struct {
	Delete []string `json:"delete"`
	// Keep is what stays: see deletableBranches.
	Keep []string `json:"keep"`
	// Unmerged is those in Delete whose work would be lost to all but the reflog and Undo.
	Unmerged []UnmergedBranch `json:"unmerged"`
}

// StackDeletePlan says what deleting these branches of a stack would do.
func (a *App) StackDeletePlan(path string, names []string) (StackPlan, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return StackPlan{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mergedBranchesTimeout)
	defer cancel()
	return a.stackDeletePlan(ctx, r, names)
}

// stackDeletePlan counts a branch as unmerged when the time runs out before it's been checked,
// which only means being asked.
func (a *App) stackDeletePlan(ctx context.Context, r *session.Repo, names []string) (StackPlan, error) {
	targets, err := a.mergeTargetsFor(ctx, r)
	if err != nil {
		return StackPlan{}, err
	}
	merged := r.Git.MergedBranches(ctx, names, targets)
	plan := StackPlan{Delete: a.deletableBranches(ctx, r, names), Keep: []string{}, Unmerged: []UnmergedBranch{}}
	for _, name := range names {
		if !slices.Contains(plan.Delete, name) {
			plan.Keep = append(plan.Keep, name)
		}
	}
	for _, name := range plan.Delete {
		if _, ok := merged[name]; !ok {
			plan.Unmerged = append(plan.Unmerged, UnmergedBranch{Name: name, Commits: r.Git.BranchMergeState(ctx, name, targets).Unmerged})
		}
	}
	return plan, nil
}

// DeleteStack deletes what StackDeletePlan allows of these branches, as one step to undo. It
// looks again first, and unless allowed to lose work, stops if anything isn't merged any more.
// It returns the names of those it deleted.
func (a *App) DeleteStack(path, root string, names []string, allowUnmerged bool) ([]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mergedBranchesTimeout)
	defer cancel()
	plan, err := a.stackDeletePlan(ctx, r, names)
	if err != nil {
		return nil, err
	}
	if len(plan.Unmerged) > 0 && !allowUnmerged {
		return nil, userError(plan.Unmerged[0].Name + " isn't merged any more, so nothing was deleted.")
	}
	refs, err := a.currentRefs(ctx, r, plan.Delete)
	if err != nil {
		return nil, err
	}
	gone := a.deleteTogether(ctx, r, refs, "Delete stack "+root, len(plan.Unmerged) == 0)
	if gone == nil {
		return []string{}, nil
	}
	r.ReloadGraph()
	return gone, nil
}

// currentRefs reads these local branches as they are now, not as last loaded, since undoing
// a delete has to put them back exactly where they were.
func (a *App) currentRefs(ctx context.Context, r *session.Repo, names []string) ([]git.Ref, error) {
	current, err := r.Git.Refs(ctx)
	if err != nil {
		return nil, err
	}
	var refs []git.Ref
	for _, ref := range current {
		if ref.Kind == git.LocalBranch && slices.Contains(names, ref.Short) {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

// Unstack stops treating these branches as stacked on anything. It returns what was set for
// them by hand before, for RestoreStack.
func (a *App) Unstack(path string, names []string) (map[string]string, error) {
	before := map[string]string{}
	err := a.updateStackParents(path, func(parents map[string]string) {
		for _, name := range names {
			if parent, ok := parents[name]; ok {
				before[name] = parent
			}
			parents[name] = ""
		}
	})
	if err != nil {
		return nil, err
	}
	return before, nil
}

// RestoreStack undoes Unstack: these branches go back to what was set by hand, or to being
// worked out from history.
func (a *App) RestoreStack(path string, names []string, before map[string]string) error {
	return a.updateStackParents(path, func(parents map[string]string) {
		for _, name := range names {
			if parent, ok := before[name]; ok {
				parents[name] = parent
			} else {
				delete(parents, name)
			}
		}
	})
}
