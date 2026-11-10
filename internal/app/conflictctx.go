package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// ConflictContext is what the conflict tool shows around the file being resolved (issue #597).
type ConflictContext struct {
	// MergeTool is the user's merge.tool, or "" when none is set.
	MergeTool string `json:"mergeTool"`
	// Clean are the files the operation changed without a conflict, up to a cap.
	Clean []string `json:"clean"`
}

// ConflictContext returns what surrounds the conflicted files of the operation in progress.
func (a *App) ConflictContext(path string) (ConflictContext, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return ConflictContext{}, err
	}
	ctx := context.Background()
	out := ConflictContext{MergeTool: r.Git.MergeToolName(ctx), Clean: r.Git.MergedCleanly(ctx)}
	if out.Clean == nil {
		out.Clean = []string{}
	}
	return out, nil
}

// ConflictOrigin names the commit that last changed lines start to end (1-based, inclusive)
// of file on one side of the conflict: "ours", "base" or "theirs". Its Commit is "" when
// there's nothing to name.
func (a *App) ConflictOrigin(path, file, side string, start, end int) (git.PlaceOrigin, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.PlaceOrigin{}, err
	}
	return r.Git.PlaceOrigin(context.Background(), r.GitDir, file, side, start, end)
}

// OpenMergeTool opens file in the user's own merge tool and returns when they close it. A
// result the tool reports as good is staged, which resolves the file.
func (a *App) OpenMergeTool(path, file string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	err = r.Git.RunMergeTool(context.Background(), file)
	if errors.Is(err, git.ErrNoMergeTool) {
		return userError("No merge tool is set. Set merge.tool in your git config to use one.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) {
		// The tool was closed without saving, or failed to start: the file is as it was.
		return userError("The merge tool closed without resolving " + file + ".")
	}
	return err
}
