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
	// Ours is the branch being changed ("" when HEAD was detached) and Theirs the other side:
	// what is merged in, or what a rebase is onto.
	Ours   string `json:"ours"`
	Theirs string `json:"theirs"`
	// Commit is the commit in hand, with its subject.
	Commit  string `json:"commit"`
	Subject string `json:"subject"`
	// OrigHead is where aborting puts the branch back.
	OrigHead string `json:"origHead"`
	// Written are the commits a rebase has written so far, newest first.
	Written []string `json:"written"`
	// Author, for git am, wrote the patch in hand (#1023). Applied counts the patches before it
	// that went in; the rest before it were skipped.
	Author  string `json:"author"`
	Applied int    `json:"applied"`
	// Stuck says git am stopped on a patch it couldn't apply at all, StuckReason why. Continue
	// isn't offered then: there's nothing to resolve, and nothing to commit until the change
	// is made by hand and staged.
	Stuck       bool   `json:"stuck"`
	StuckReason string `json:"stuckReason"`
	// MadeBranch names the branch the apply preview made for the series (#1022), which
	// aborting removes too, when nothing else is on it.
	MadeBranch string `json:"madeBranch"`
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
	out := RepoState{
		Operation: string(st.Op), Step: st.Step, Total: st.Total, Conflicts: st.Conflicts, Actions: []string{},
		Ours: st.Ours, Theirs: st.Theirs, Commit: st.Commit, Subject: st.Subject, OrigHead: st.OrigHead, Written: st.Written,
		Author: st.Author, Applied: st.Applied, Stuck: st.Stuck, StuckReason: st.StuckReason,
	}
	if out.Written == nil {
		out.Written = []string{}
	}
	for _, act := range st.Op.Actions() {
		if st.Stuck && act == git.Continue {
			continue
		}
		out.Actions = append(out.Actions, string(act))
	}
	if st.Op == git.Am {
		if b, ok := readSeriesBranch(r.GitDir); ok && b.name == st.Ours {
			out.MadeBranch = b.name
		}
	}
	return out, nil
}

// CurrentPatch is the patch git am has in hand, for Show the patch when one didn't apply.
func (a *App) CurrentPatch(path string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	text, err := r.Git.CurrentPatch(context.Background())
	if err != nil {
		return "", patchError(err)
	}
	return text, nil
}

// RunOperationAction continues, skips or aborts the operation in progress.
//
// Continuing or skipping to the end leaves an undo entry that puts the branch back where it
// was before the operation began. If it stops again on the next conflict, that isn't an error:
// the state says where it stopped.
func (a *App) RunOperationAction(path, action string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	r := m.Get(path)
	if r == nil {
		return userError("That repository isn't open.")
	}
	ctx := context.Background()
	before, _ := r.Git.OperationState(ctx, r.GitDir)
	err = r.Git.RunAction(ctx, r.GitDir, git.Action(action))
	var notAllowed *git.ErrActionNotAllowed
	if errors.As(err, &notAllowed) {
		return userError("Can't " + action + ": " + notAllowed.Error() + ".")
	}
	if before.Op == git.Am && git.Action(action) == git.Abort && err == nil {
		dropSeriesBranch(ctx, r.Git, r.GitDir)
	}
	if git.Action(action) == git.Abort || before.Op == git.Bisect {
		return err
	}
	after, serr := r.Git.OperationState(ctx, r.GitDir)
	if serr != nil {
		return err
	}
	if after.Op == git.None && before.Op == git.Am {
		// Every patch is in: the branch made for them stays, and isn't the preview's to remove.
		clearSeriesBranch(r.GitDir)
	}
	if after.Op != git.None {
		// Still going. Stopped on a later commit's conflicts: the strip shows them.
		if err != nil && len(after.Conflicts) > 0 && (after.Commit != before.Commit || len(before.Conflicts) == 0) {
			return nil
		}
		return err
	}
	if err == nil && before.OrigHead != "" {
		if head := r.Git.Head(ctx); head.Commit != before.OrigHead {
			a.undo.push(r.Path, undoEntry{Label: operationLabel(before), Kind: kindMove, Before: git.HeadState{Branch: head.Branch, Commit: before.OrigHead}, After: head})
			a.undoChanged(r.Path)
		}
	}
	return err
}

// operationLabel names a finished operation for Undo.
func operationLabel(st git.OperationState) string {
	switch st.Op {
	case git.Rebase:
		return "Rebase onto " + shortRef(st.Theirs)
	case git.Merge:
		return "Merge " + shortRef(st.Theirs)
	case git.CherryPick:
		return "Cherry-pick " + shortRef(st.Commit)
	case git.Revert:
		return "Revert " + shortRef(st.Commit)
	case git.Am:
		return "Apply patches"
	}
	return string(st.Op)
}
