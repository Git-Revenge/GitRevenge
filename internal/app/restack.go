package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// Restack moves a stacked branch whose parent was merged onto the Trunk: with a squash
// merge, the branch still carries the parent's old commits, so `git rebase --onto <trunk>
// <parent's tip>` replays only its own. The branch is checked out first if it isn't
// already; each step is recorded for Undo. Conflicts enter the normal rebase flow.
func (a *App) Restack(path, branch string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	refs, err := r.Git.Refs(ctx)
	if err != nil {
		return err
	}
	rules := a.branchRules(r.Path)
	parents := session.Stacks(ctx, r.Git, refs, rules, a.repoSettings(r.Path).StackParents)
	parent := parents[branch]
	if parent == "" {
		return userError(branch + " isn't stacked on another branch.")
	}
	// The Trunk's upstream holds the merge; the local branch may not be pulled yet.
	onto, upstream := trunkOf(refs, rules)
	if upstream != "" {
		onto = upstream
	}
	var parentTip string
	for _, ref := range refs {
		if ref.Kind == git.LocalBranch && ref.Short == parent {
			parentTip = ref.Commit
		}
	}
	if onto == "" {
		return userError("There's no Trunk branch to restack onto.")
	}
	if parentTip == "" {
		return userError(parent + " is gone, so there's no record of which commits were its.")
	}
	if r.Git.Head(ctx).Branch != branch {
		if err := a.record(r, "Checkout "+branch, kindCheckout, "", func() error { return r.Git.Checkout(ctx, branch) }); err != nil {
			return branchErr(err)
		}
	}
	err = a.record(r, "Restack "+branch+" onto "+onto, kindMove, "", func() error { return r.Git.RebaseOnto(ctx, onto, parentTip) })
	if errors.Is(err, git.ErrConflicts) {
		return userError("Restacking " + branch + " hit conflicts. Fix the conflicted files, stage them, then use Continue in the banner, or Abort to undo.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		return gitMessage(gerr.Stderr)
	}
	return writeErr(err)
}
