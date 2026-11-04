package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Worktrees lists the repository's main and linked worktrees.
func (a *App) Worktrees(path string) ([]git.Worktree, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	return r.Git.Worktrees(context.Background())
}

// AddWorktree checks out branch in a new linked worktree at dir. An existing local branch
// is checked out; any other name creates that branch at HEAD; an empty name detaches.
func (a *App) AddWorktree(path, dir, branch string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	dir, branch = strings.TrimSpace(dir), strings.TrimSpace(branch)
	if dir == "" || !filepath.IsAbs(dir) {
		return userError("Choose a full path for the new worktree.")
	}
	ctx := context.Background()
	existing := ""
	newBranch := branch
	if branch != "" {
		if _, err := r.Git.ReadString(ctx, "rev-parse", "--verify", "-q", "refs/heads/"+branch); err == nil {
			existing, newBranch = branch, ""
		}
	}
	return worktreeErr(r.Git.AddWorktree(ctx, dir, existing, newBranch))
}

// RemoveWorktree removes a linked worktree; force also removes one with uncommitted changes.
func (a *App) RemoveWorktree(path, dir string, force bool) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return worktreeErr(r.Git.RemoveWorktree(context.Background(), dir, force))
}

// worktreeErr shows git's own explanation ("already checked out at …", "contains modified
// or untracked files, use --force") rather than the command line.
func worktreeErr(err error) error {
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		return gitMessage(gerr.Stderr)
	}
	return writeErr(err)
}
