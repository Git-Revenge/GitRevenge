package app

import (
	"context"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// maxArrived is how many arrived commits the graph marks. A pull that brings more still says
// how many there were.
const maxArrived = 500

// Arrived is what a branch gained when its tip moved.
type Arrived struct {
	// Commits are the IDs of the commits the branch has now and didn't before, newest first,
	// at most maxArrived of them.
	Commits []string `json:"commits"`
	// Total counts them all.
	Total int `json:"total"`
}

// HeadCommit returns the checked-out commit and the branch it's on: the branch is empty when
// HEAD is detached, and the commit before a repository's first commit.
func (a *App) HeadCommit(path string) (git.HeadState, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.HeadState{}, err
	}
	h := r.Git.Head(context.Background())
	h.Branch, h.Commit = strings.TrimSpace(h.Branch), strings.TrimSpace(h.Commit)
	return h, nil
}

// OwnMoveAge returns how many milliseconds ago this app last ran something that can move a
// branch in the repository at path (a commit, pull, checkout, merge, rebase, reset, undo and the
// like), or -1 if it hasn't this session. The graph follows a branch that moved only when the
// move was the person's own doing here, not a change made under them by another tool.
func (a *App) OwnMoveAge(path string) (int, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return -1, err
	}
	ago, ok := a.undo.movedAgo(r.Path)
	if !ok {
		return -1, nil
	}
	return int(ago.Milliseconds()), nil
}

// ArrivedCommits says what a branch gained when its tip moved from one commit to another: the
// commits reachable from the new tip and not from the old one. A branch moved backwards gained
// nothing; a rebased one gained every commit that was rewritten.
func (a *App) ArrivedCommits(path, from, to string) (Arrived, error) {
	none := Arrived{Commits: []string{}}
	if !isCommitID(from) || !isCommitID(to) {
		return none, userError("That isn't a commit.")
	}
	r, err := a.openRepo(path)
	if err != nil {
		return none, err
	}
	lines, total, err := r.Git.CommitsBetween(context.Background(), from, to, maxArrived)
	if err != nil {
		return none, err
	}
	out := Arrived{Commits: make([]string, 0, len(lines)), Total: total}
	for _, l := range lines {
		out.Commits = append(out.Commits, l.ID)
	}
	return out, nil
}

// isCommitID reports whether s is a full or abbreviated commit ID and nothing else, so it can
// be handed to git as a revision without being read as an option or a range.
func isCommitID(s string) bool {
	if len(s) < 4 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
