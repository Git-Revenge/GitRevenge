package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// ConflictFile returns the base, ours, theirs and merged versions of a conflicted file.
func (a *App) ConflictFile(path, file string) (git.ConflictFile, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.ConflictFile{}, err
	}
	return r.Git.Conflict(context.Background(), file)
}

// ResolveConflict saves content as the file's resolution and marks it resolved.
func (a *App) ResolveConflict(path, file, content string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	fp := fingerprintBefore(ctx, r.Git, file)
	if err := r.Git.ResolveWith(ctx, file, content); err != nil {
		return writeErr(err)
	}
	a.noteResolution(r, fp)
	return nil
}

// TakeConflictSide resolves a file with one whole side: "ours" or "theirs".
func (a *App) TakeConflictSide(path, file, side string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	fp := fingerprintBefore(ctx, r.Git, file)
	err = r.Git.TakeSide(ctx, file, git.ConflictSide(side))
	if errors.Is(err, git.ErrNotConflicted) {
		return userError(file + " is no longer conflicted.")
	}
	if err != nil {
		return writeErr(err)
	}
	a.noteResolution(r, fp)
	return nil
}
