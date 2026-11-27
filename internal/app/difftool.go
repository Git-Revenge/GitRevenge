package app

import (
	"context"
	"errors"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// DiffTool says which tool Open in diff tool opens (#939), and where it's set: diff.guitool,
// diff.tool or merge.tool. Its Name is "" when none is, and the app offers nothing.
func (a *App) DiffTool(path string) (git.DiffTool, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.DiffTool{}, err
	}
	return r.Git.DiffToolSet(context.Background()), nil
}

// OpenDiffTool opens one file's comparison, the one the diff view shows, in the user's diff
// tool with git difftool, and returns when they close it.
func (a *App) OpenDiffTool(path string, t git.DiffToolTarget) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	err = r.Git.RunDiffTool(ctx, t)
	if errors.Is(err, git.ErrNoDiffTool) {
		return userError("No diff tool is set. Set diff.tool in your git config to use one.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) {
		tool := r.Git.DiffToolSet(ctx).Name
		said := strings.TrimSpace(gerr.Stderr)
		if i := strings.IndexByte(said, '\n'); i >= 0 {
			said = said[:i]
		}
		if said == "" {
			return userError(tool + " closed with an error.")
		}
		return userError(tool + " didn't open: " + strings.TrimPrefix(said, "fatal: "))
	}
	return err
}
