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
// another blame stops this one, which then answers with nothing. In a partial clone it
// downloads nothing: BlameNeeds says what it would need, and the view asks before
// BlameDownloading (#1003).
func (a *App) Blame(path, file, rev string) ([]git.BlameLine, error) {
	return a.blame(path, file, rev, false)
}

// BlameDownloading blames as Blame does, letting git download any version a partial clone
// still lacks: for after the view has asked, and the versions BlameNeeds named have come in
// one download.
func (a *App) BlameDownloading(path, file, rev string) ([]git.BlameLine, error) {
	return a.blame(path, file, rev, true)
}

func (a *App) blame(path, file, rev string, mayDownload bool) ([]git.BlameLine, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx, done := a.latest.start("blame", r.Path)
	defer done()
	lines, err := r.Git.Blame(ctx, file, rev, mayDownload)
	if errors.Is(err, context.Canceled) {
		return []git.BlameLine{}, nil // superseded, or the repository closed
	}
	if errors.Is(err, git.ErrNotDownloaded) {
		return nil, userError("Some versions of " + file + " were never downloaded to this partial clone, and blame doesn't download them by itself.")
	}
	if lines == nil {
		lines = []git.BlameLine{}
	}
	return lines, err
}
