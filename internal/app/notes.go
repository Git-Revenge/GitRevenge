package app

import (
	"context"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// CommitNotes returns the commit's git notes from every notes ref, the default first.
func (a *App) CommitNotes(path, id string) ([]git.Note, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	return r.Git.Notes(context.Background(), id)
}

// NotedCommits lists the commits that have a note in any notes ref, for the graph's note
// marks. Notes can sit on commits the graph doesn't show; those are simply never drawn.
func (a *App) NotedCommits(path string) ([]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ids, err := r.Git.NotedCommits(context.Background())
	if err != nil {
		return nil, err
	}
	return nonNil(ids), nil
}

// isNotesQuery reports whether a graph search asks for the commits that have a note.
func isNotesQuery(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	return q == "has:note" || q == "has:notes"
}

// SetCommitNote adds or replaces the commit's note in the default notes ref
// (refs/notes/commits), or removes it when text is empty.
func (a *App) SetCommitNote(path, id, text string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return writeErr(r.Git.SetNote(context.Background(), git.DefaultNotesRef, id, text))
}
