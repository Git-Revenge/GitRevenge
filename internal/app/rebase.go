package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// RebasePlan lists the commits after base (oldest first) for the interactive rebase editor.
func (a *App) RebasePlan(path, base string) ([]git.RebaseItem, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	items, err := r.Git.RebasePlan(context.Background(), base)
	if errors.Is(err, git.ErrMergesInRange) {
		return nil, userError("Those commits include merges, which an interactive rebase would flatten. Pick a starting point after the last merge.")
	}
	return items, err
}

// InteractiveRebase rewrites the commits after base as planned in the editor. It can be
// undone like any other history change.
func (a *App) InteractiveRebase(path, base string, items []git.RebaseItem) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	err = a.record(r, "Interactive rebase", kindMove, "", func() error {
		return r.Git.InteractiveRebase(context.Background(), base, items)
	})
	if errors.Is(err, git.ErrConflicts) {
		return userError("The rebase stopped on a conflict. Resolve it, then use Continue in the banner, or Abort to put everything back.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		return gitMessage(gerr.Stderr)
	}
	if err != nil && !errors.As(err, new(userError)) {
		// Validation errors from the plan are written for people already.
		return userError(err.Error())
	}
	return err
}
