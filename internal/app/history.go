package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// FileHistory lists commits that changed file, newest first, following renames. Asking for
// another file's history stops this one, which then answers with nothing.
func (a *App) FileHistory(path, file string) ([]git.FileCommit, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx, done := a.latest.start("history", r.Path)
	defer done()
	commits, err := r.Git.FileHistory(ctx, file)
	if errors.Is(err, context.Canceled) {
		return []git.FileCommit{}, nil // superseded, or the repository closed
	}
	return commits, err
}

// Blame attributes each line of file at rev ("" for the work tree) to a commit. Asking for
// another blame stops this one, which then answers with nothing.
func (a *App) Blame(path, file, rev string) ([]git.BlameLine, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx, done := a.latest.start("blame", r.Path)
	defer done()
	lines, err := r.Git.Blame(ctx, file, rev)
	if errors.Is(err, context.Canceled) {
		return []git.BlameLine{}, nil // superseded, or the repository closed
	}
	if lines == nil {
		lines = []git.BlameLine{}
	}
	return lines, err
}
