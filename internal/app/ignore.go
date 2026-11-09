package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// IgnoreAdded is a rule just written, and the files it stopped tracking, so it can be
// taken back.
type IgnoreAdded struct {
	Rule git.IgnoreLine `json:"rule"`
	// Hidden counts the files the rule hid from the changes list.
	Hidden    int      `json:"hidden"`
	Untracked []string `json:"untracked"`
}

// Ignored is the list of ignored paths in a work tree.
type Ignored struct {
	Files []git.IgnoredFile `json:"files"`
	// Truncated means there were more than the list holds.
	Truncated bool `json:"truncated"`
}

// IgnoreFiles names the ignore files outside the work tree, for saying where a rule goes.
type IgnoreFiles struct {
	Global string `json:"global"`
}

// IgnoreFileNames returns where the global excludes file is.
func (a *App) IgnoreFileNames(path string) (IgnoreFiles, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return IgnoreFiles{}, err
	}
	g, err := r.Git.GlobalExcludesFile(context.Background())
	return IgnoreFiles{Global: g}, err
}

// PreviewIgnore says which files a rule would hide, and which tracked files it matches,
// without writing it.
func (a *App) PreviewIgnore(path string, target git.IgnoreTarget, pattern string) (git.IgnorePreview, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.IgnorePreview{}, err
	}
	return r.Git.PreviewIgnore(context.Background(), target, pattern)
}

// AddIgnore writes an ignore rule and stops tracking the given files, keeping them on disk.
func (a *App) AddIgnore(path string, target git.IgnoreTarget, pattern string, untrack []string) (IgnoreAdded, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return IgnoreAdded{}, err
	}
	ctx := context.Background()
	pv, err := r.Git.PreviewIgnore(ctx, target, pattern)
	if err != nil {
		return IgnoreAdded{}, err
	}
	rule, err := r.Git.AddIgnoreRule(ctx, target, pattern)
	if err != nil {
		return IgnoreAdded{}, err
	}
	if err := r.Git.StopTracking(ctx, untrack); err != nil {
		return IgnoreAdded{Rule: rule}, writeErr(err)
	}
	if untrack == nil {
		untrack = []string{}
	}
	return IgnoreAdded{Rule: rule, Hidden: len(pv.Hidden), Untracked: untrack}, nil
}

// UndoIgnore takes back AddIgnore: the rule is removed and untracked files are tracked
// again.
func (a *App) UndoIgnore(path string, added IgnoreAdded) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := ruleErr(r.Git.RemoveIgnoreRule(ctx, added.Rule)); err != nil {
		return err
	}
	return writeErr(r.Git.Unstage(ctx, added.Untracked))
}

// IgnoredFiles lists ignored paths with the rule behind each.
func (a *App) IgnoredFiles(path string) (Ignored, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return Ignored{}, err
	}
	files, truncated, err := r.Git.IgnoredFiles(context.Background())
	return Ignored{Files: files, Truncated: truncated}, err
}

// StopIgnoring brings an ignored path back: with except, by adding a "!" rule for it;
// otherwise by removing the rule that ignores it, which brings back whatever else that
// rule matched. It returns the rule it added or removed.
func (a *App) StopIgnoring(path, file string, rule git.IgnoreLine, except bool) (git.IgnoreLine, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.IgnoreLine{}, err
	}
	ctx := context.Background()
	if except {
		return r.Git.ExceptIgnored(ctx, file, rule)
	}
	return rule, ruleErr(r.Git.RemoveIgnoreRule(ctx, rule))
}

// RemoveIgnoreRule deletes one rule, such as an exception being taken back.
func (a *App) RemoveIgnoreRule(path string, rule git.IgnoreLine) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return ruleErr(r.Git.RemoveIgnoreRule(context.Background(), rule))
}

func ruleErr(err error) error {
	if errors.Is(err, git.ErrRuleMoved) {
		return userError("The ignore file changed since this was shown, so nothing was removed. Look again and retry.")
	}
	return err
}

// RestoreIgnoreRule puts back a rule StopIgnoring removed, on its old line.
func (a *App) RestoreIgnoreRule(path string, rule git.IgnoreLine) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return r.Git.RestoreIgnoreRule(context.Background(), rule)
}
