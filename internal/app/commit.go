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
	// Author and Committer are who the mailmap says wrote and committed it: the names the
	// graph shows. They're the commit's own (Commit.Author, Commit.Committer) unless a
	// mailmap line reads them as someone else.
	Author    git.Identity `json:"author"`
	Committer git.Identity `json:"committer"`
	// Edge means the commit is at a shallow clone's edge (#1006): its parents weren't fetched,
	// so Files are every file it has, as for a first commit.
	Edge bool `json:"edge,omitempty"`
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
	// The commit was read as an object, so its names are as written. Ask the mailmap beside
	// the file list, which is the slower of the two. Without an answer the names stand.
	written := []git.Identity{{Name: c.Author.Name, Email: c.Author.Email}, {Name: c.Committer.Name, Email: c.Committer.Email}}
	shown := make(chan []git.Identity, 1)
	go func() {
		ids, _ := r.Git.Mailmap(context.Background(), written)
		shown <- ids
	}()
	// At a shallow clone's edge there's no parent to compare with: the commit's files are all
	// its own, as a first commit's are.
	compared := c
	edge := len(c.Parents) > 0 && r.Git.IsShallowEdge(context.Background(), c.ID)
	if edge {
		compared.Parents = nil
	}
	files, err := r.Git.ChangedFiles(context.Background(), compared)
	ids := <-shown
	if err != nil {
		return CommitInfo{}, err
	}
	return CommitInfo{Commit: c, Files: files, Author: ids[0], Committer: ids[1], Edge: edge}, nil
}
