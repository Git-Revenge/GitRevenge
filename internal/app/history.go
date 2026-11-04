package app

import (
	"context"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// FileHistory lists commits that changed file, newest first, following renames.
func (a *App) FileHistory(path, file string) ([]git.FileCommit, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	return r.Git.FileHistory(context.Background(), file)
}

// Blame attributes each line of file at rev ("" for the work tree) to a commit.
func (a *App) Blame(path, file, rev string) ([]git.BlameLine, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	lines, err := r.Git.Blame(context.Background(), file, rev)
	if lines == nil {
		lines = []git.BlameLine{}
	}
	return lines, err
}
