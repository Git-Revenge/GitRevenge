package app

import (
	"context"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// DroppedCommit says what goes with a commit the plan drops.
type DroppedCommit struct {
	Commit string `json:"commit"`
	// Files are the files it changed, up to a cap; FileCount is how many in all.
	Files     []string `json:"files"`
	FileCount int      `json:"fileCount"`
	Added     int      `json:"added"`
	Deleted   int      `json:"deleted"`
}

// RebasePlanPreview is what running the rebase editor's plan would do (issue #596).
type RebasePlanPreview struct {
	// StopsAt is the first commit that wouldn't apply in this order, or "".
	StopsAt   string             `json:"stopsAt"`
	Conflicts []ForecastConflict `json:"conflicts"`
	// Incomplete means not every commit could be tried; none of those tried conflicts.
	Incomplete bool `json:"incomplete"`
	// Pushed means some of the commits are already on Upstream, so the rewritten branch will
	// need a force push.
	Pushed   bool            `json:"pushed"`
	Upstream string          `json:"upstream"`
	Dropped  []DroppedCommit `json:"dropped"`
}

// PreviewRebasePlan forecasts the plan in the rebase editor: whether it would stop on
// conflicts in this order, and what each dropped commit takes with it. Nothing is changed.
func (a *App) PreviewRebasePlan(path, base string, items []git.RebaseItem) (RebasePlanPreview, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return RebasePlanPreview{}, err
	}
	f, err := r.Git.ForecastPlan(context.Background(), base, items)
	if err != nil {
		return RebasePlanPreview{}, friendly(err)
	}
	out := RebasePlanPreview{
		StopsAt: f.StopsAt, Conflicts: forecastConflicts(f.Conflicts), Incomplete: f.Incomplete,
		Pushed: f.Pushed, Upstream: f.Upstream, Dropped: []DroppedCommit{},
	}
	for _, d := range f.Dropped {
		files := d.Files
		if files == nil {
			files = []string{}
		}
		out.Dropped = append(out.Dropped, DroppedCommit{Commit: d.Commit, Files: files, FileCount: d.FileCount, Added: d.Added, Deleted: d.Deleted})
	}
	return out, nil
}
