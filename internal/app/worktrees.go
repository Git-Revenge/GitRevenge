package app

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/fsutil"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
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
	if err := worktreeErr(r.Git.AddWorktree(ctx, dir, existing, newBranch)); err != nil {
		return err
	}
	// The branch now shows as being in a worktree.
	r.ReloadGraph()
	return nil
}

// RemoveWorktree removes a linked worktree; force also removes one with uncommitted changes.
// It returns the folder if that had to be left behind (see removeWorktree).
func (a *App) RemoveWorktree(path, dir string, force bool) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	leftover, err := a.removeWorktree(context.Background(), r, dir, force)
	if err != nil {
		return "", err
	}
	r.ReloadGraph()
	return leftover, nil
}

// removeWorktree removes the linked worktree at dir from r. A tab open on it holds the folder on
// Windows, so that closes first, unless git is about to refuse over uncommitted changes. If git
// still refuses, the tab comes back.
//
// When something else holds the folder, git drops the worktree but can't delete the folder. The
// worktree is gone all the same, so that isn't a failure: it returns the folder to mention.
func (a *App) removeWorktree(ctx context.Context, r *session.Repo, dir string, force bool) (string, error) {
	m, err := a.sessions()
	if err != nil {
		return "", err
	}
	reopen := ""
	if tab := m.Get(dir); tab != nil && tab != r {
		status, err := tab.Git.ReadString(ctx, "status", "--porcelain")
		if force || (err == nil && strings.TrimSpace(status) == "") {
			reopen = tab.Path
			if err := a.CloseRepo(reopen); err != nil {
				return "", writeErr(err)
			}
		}
	}
	removeErr := r.Git.RemoveWorktree(ctx, dir, force)
	if removeErr == nil {
		return "", nil
	}
	if wts, err := r.Git.Worktrees(ctx); err == nil && !slices.ContainsFunc(wts, func(w git.Worktree) bool {
		return fsutil.SamePath(filepath.FromSlash(w.Path), filepath.FromSlash(dir))
	}) {
		return dir, nil
	}
	if reopen != "" {
		// Best effort: the error that matters is git's, below.
		if _, err := a.OpenRepo(reopen); err == nil {
			_ = m.SetActive(r.Path)
		}
	}
	return "", worktreeErr(removeErr)
}

// DeleteBranchAndWorktree deletes a local branch along with the linked worktree it's checked
// out in. discardChanges allows uncommitted changes in the worktree to go too, and
// discardCommits allows commits that aren't merged. Anything that would refuse is found before
// the worktree goes, so it's never left half done. It returns the worktree's folder if that had
// to be left behind (see removeWorktree).
func (a *App) DeleteBranchAndWorktree(path, name string, discardChanges, discardCommits bool) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	wts, err := r.Git.Worktrees(ctx)
	if err != nil {
		return "", writeErr(err)
	}
	var tree *git.Worktree
	for i := range wts {
		if wts[i].Branch == name {
			tree = &wts[i]
		}
	}
	if tree != nil && tree.Main {
		return "", userError(name + " is checked out in the main worktree at " + tree.Path + ", which can't be removed. Switch to another branch there first.")
	}
	if tree != nil && tree.Current {
		return "", userError(name + " is checked out in the worktree open here. Delete it from another worktree's tab.")
	}
	if !discardCommits && !r.Git.BranchFullyMerged(ctx, name) {
		return "", branchErr(git.ErrNotMerged)
	}
	leftover := ""
	if tree != nil {
		if leftover, err = a.removeWorktree(ctx, r, tree.Path, discardChanges); err != nil {
			return "", err
		}
	}
	commit, _ := r.Git.ReadString(ctx, "rev-parse", "refs/heads/"+name)
	// Merged or allowed, as checked above.
	if err := r.Git.DeleteBranch(ctx, name, true); err != nil {
		r.ReloadGraph()
		return leftover, branchErr(err)
	}
	if commit != "" {
		a.recordBranch(r, "Delete branch "+name, kindBranchDelete, name, commit)
	}
	r.ReloadGraph()
	return leftover, nil
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
