package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// RepoState describes an in-progress operation for the banner.
type RepoState struct {
	// Operation is "", "merge", "rebase", "cherry-pick", "revert", "bisect" or "am".
	Operation string `json:"operation"`
	Step      int    `json:"step"`
	Total     int    `json:"total"`
	// Actions lists the buttons to show: "continue", "skip", "abort".
	Actions   []string `json:"actions"`
	Conflicts []string `json:"conflicts"`
}

// RepoState returns the operation in progress in the repository at path, if any.
func (a *App) RepoState(path string) (RepoState, error) {
	m, err := a.sessions()
	if err != nil {
		return RepoState{}, err
	}
	r := m.Get(path)
	if r == nil {
		return RepoState{}, userError("That repository isn't open.")
	}
	st, err := r.Git.OperationState(context.Background(), r.GitDir)
	if err != nil {
		return RepoState{}, err
	}
	out := RepoState{Operation: string(st.Op), Step: st.Step, Total: st.Total, Conflicts: st.Conflicts, Actions: []string{}}
	for _, act := range st.Op.Actions() {
		out.Actions = append(out.Actions, string(act))
	}
	return out, nil
}

// RunOperationAction continues, skips or aborts the operation in progress.
func (a *App) RunOperationAction(path, action string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	r := m.Get(path)
	if r == nil {
		return userError("That repository isn't open.")
	}
	err = r.Git.RunAction(context.Background(), r.GitDir, git.Action(action))
	var notAllowed *git.ErrActionNotAllowed
	if errors.As(err, &notAllowed) {
		return userError("Can't " + action + ": " + notAllowed.Error() + ".")
	}
	return err
}
