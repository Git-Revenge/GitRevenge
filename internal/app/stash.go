package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// StashSave stashes local changes. includeUntracked also stashes untracked files.
func (a *App) StashSave(path, message string, includeUntracked bool) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return stashErr(r.Git.StashSave(context.Background(), message, includeUntracked))
}

// StashAction applies, pops or drops the stash with the given commit ID.
func (a *App) StashAction(path, id, action string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch action {
	case "apply":
		err = r.Git.StashApply(ctx, id)
	case "pop":
		err = r.Git.StashPop(ctx, id)
	case "drop":
		err = r.Git.StashDrop(ctx, id)
	default:
		return userError("Unknown stash action " + action + ".")
	}
	return stashErr(err)
}

func stashErr(err error) error {
	switch {
	case errors.Is(err, git.ErrNothingToStash):
		return userError("There are no changes to stash.")
	case errors.Is(err, git.ErrStashConflict):
		return userError("The stash was applied with conflicts. Resolve the conflicted files; the stash has been kept.")
	case errors.Is(err, git.ErrStashNotFound):
		return userError("That stash no longer exists.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		// e.g. "Your local changes to the following files would be overwritten".
		return gitMessage(gerr.Stderr)
	}
	return writeErr(err)
}
