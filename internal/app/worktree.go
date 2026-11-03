package app

import (
	"context"
	"errors"
	"strings"

	"github.com/risadams/gitrevenge/internal/git"
)

// WorkingTree is what the changes panel shows.
type WorkingTree struct {
	Status   git.Status   `json:"status"`
	Signing  git.Signing  `json:"signing"`
	Identity git.Identity `json:"identity"`
}

// WorkingTree returns the current status, commit signing setup and commit identity.
func (a *App) WorkingTree(path string) (WorkingTree, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return WorkingTree{}, err
	}
	ctx := context.Background()
	st, err := r.Git.Status(ctx)
	if err != nil {
		return WorkingTree{}, err
	}
	r.Git.CountLines(ctx, &st)
	return WorkingTree{Status: st, Signing: r.Git.CommitSigning(ctx), Identity: r.Git.CommitIdentity(ctx)}, nil
}

// StageFiles stages the given paths; with no paths it stages everything.
func (a *App) StageFiles(path string, files []string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return writeErr(r.Git.StageAll(context.Background()))
	}
	return writeErr(r.Git.Stage(context.Background(), files))
}

// UnstageFiles unstages the given paths; with no paths it unstages everything.
func (a *App) UnstageFiles(path string, files []string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return writeErr(r.Git.UnstageAll(context.Background()))
	}
	return writeErr(r.Git.Unstage(context.Background(), files))
}

// DiscardFiles throws away unstaged changes to the given paths and deletes untracked ones.
// What to do with each path is decided from a fresh status, not from the caller, since
// deleting is irreversible.
func (a *App) DiscardFiles(path string, files []string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	st, err := r.Git.Status(ctx)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, f := range files {
		want[f] = true
	}
	var targets []git.FileStatus
	for _, f := range st.Files {
		if want[f.Path] {
			targets = append(targets, f)
		}
	}
	err = r.Git.Discard(ctx, targets)
	if errors.Is(err, git.ErrConflicted) {
		return userError("Conflicted files can't be discarded here. Resolve the conflict or abort the operation instead.")
	}
	return writeErr(err)
}

// CommitChanges commits the staged changes (or amends the last commit) and returns the new
// commit ID.
func (a *App) CommitChanges(path, message string, amend bool) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	summary, _, _ := strings.Cut(strings.TrimSpace(message), "\n")
	label := "Commit “" + summary + "”"
	if amend {
		label = "Amend “" + summary + "”"
	}
	var sha string
	err = a.record(r, label, kindCommit, "", func() error {
		var cerr error
		sha, cerr = r.Git.Commit(context.Background(), message, amend)
		return cerr
	})
	return sha, commitErr(err)
}

// LastCommitMessage returns HEAD's message, to prefill an amend.
func (a *App) LastCommitMessage(path string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	return r.Git.LastCommitMessage(context.Background())
}

// writeErr turns a held git lock into a message the user can act on.
func writeErr(err error) error {
	var locked *git.LockedError
	if errors.As(err, &locked) {
		return userError("Another git process is running in this repository. If none is, delete " + locked.Path + " and try again.")
	}
	return err
}

// commitErr explains common commit failures; anything else (such as hook output) is shown
// as git reported it.
func commitErr(err error) error {
	if errors.Is(err, git.ErrNothingStaged) {
		return userError("Nothing is staged. Stage some changes first, or use Amend to reword the last commit.")
	}
	var gerr *git.Error
	if !errors.As(err, &gerr) {
		return writeErr(err)
	}
	switch s := gerr.Stderr; {
	case strings.Contains(s, "failed to sign"), strings.Contains(s, "gpg failed"):
		return userError("Signing the commit failed. Check that your signing key is available (gpg-agent or ssh-agent) and that commit.gpgsign / user.signingkey are set correctly.\n\n" + s)
	case strings.Contains(s, "Please tell me who you are"):
		return userError("Git doesn't know who you are. Set user.name and user.email in your git config, then try again.")
	}
	if s := strings.TrimSpace(gerr.Stderr); s != "" {
		// Usually a hook rejecting the commit: its own output is the most useful message.
		return gitMessage(s)
	}
	return err
}
