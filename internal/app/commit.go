package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// CommitInfo is everything the commit detail panel shows.
type CommitInfo struct {
	Commit git.CommitDetail `json:"commit"`
	Files  []git.FileChange `json:"files"`
}

// CommitDetail returns a commit's metadata, message and changed files.
func (a *App) CommitDetail(path, id string) (CommitInfo, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return CommitInfo{}, err
	}
	c, err := git.ReadCommit(r.Objects, id)
	if errors.Is(err, git.ErrObjectMissing) {
		return CommitInfo{}, userError("That commit no longer exists.")
	}
	if err != nil {
		return CommitInfo{}, err
	}
	files, err := r.Git.ChangedFiles(context.Background(), c)
	if err != nil {
		return CommitInfo{}, err
	}
	return CommitInfo{Commit: c, Files: files}, nil
}
