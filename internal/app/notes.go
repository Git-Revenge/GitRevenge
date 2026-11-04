package app

import (
	"context"

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

// SetCommitNote adds or replaces the commit's note in the default notes ref
// (refs/notes/commits), or removes it when text is empty.
func (a *App) SetCommitNote(path, id, text string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return writeErr(r.Git.SetNote(context.Background(), git.DefaultNotesRef, id, text))
}
