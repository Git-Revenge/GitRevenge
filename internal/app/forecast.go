package app

import (
	"context"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// ForecastConflict is a file a merge or rebase would stop on.
type ForecastConflict struct {
	Path string `json:"path"`
	// Places is how many separate conflicts the file would have; 0 when that can't be
	// counted, such as a binary file.
	Places int `json:"places"`
}

// ForecastCommit is a commit a merge would bring in or a rebase would write again.
type ForecastCommit struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
	// FixupOf is the commit in the same list this one is a fixup! or squash! for, or "".
	FixupOf string `json:"fixupOf"`
}

// MergePreview is what merging a branch, tag or commit into the checked-out branch would do
// (issue #594). Nothing is changed to work it out.
type MergePreview struct {
	// Into is the checked-out branch, or "" when HEAD is detached.
	Into string `json:"into"`
	// AlreadyMerged means there is nothing to merge.
	AlreadyMerged bool `json:"alreadyMerged"`
	// FastForward means the checked-out branch can simply move forward: no merge commit is
	// needed, and nothing can conflict.
	FastForward bool `json:"fastForward"`
	// Commits come in newest first, up to a cap; CommitCount is how many there are in all.
	Commits     []ForecastCommit   `json:"commits"`
	CommitCount int                `json:"commitCount"`
	Files       int                `json:"files"`
	Conflicts   []ForecastConflict `json:"conflicts"`
	// Unknown means the conflicts couldn't be worked out; Conflicts then says nothing.
	Unknown bool `json:"unknown"`
}

// RebasePreview is what rebasing the checked-out branch onto another commit would do.
type RebasePreview struct {
	// Branch is the checked-out branch, or "" when HEAD is detached.
	Branch string `json:"branch"`
	// Commits are written again oldest first, up to a cap; CommitCount is how many in all.
	Commits     []ForecastCommit `json:"commits"`
	CommitCount int              `json:"commitCount"`
	// Fixups is how many of them are fixup! or squash! commits for another in the list.
	Fixups int `json:"fixups"`
	// UpToDate means the branch is already on top of the target.
	UpToDate bool `json:"upToDate"`
	// StopsAt is the first commit that wouldn't apply, with its subject and the files in
	// conflict there; "" when every commit tried applies.
	StopsAt        string             `json:"stopsAt"`
	StopsAtSubject string             `json:"stopsAtSubject"`
	StopsAtNumber  int                `json:"stopsAtNumber"`
	Conflicts      []ForecastConflict `json:"conflicts"`
	// Incomplete means not every commit could be tried; none of those tried conflicts.
	Incomplete bool `json:"incomplete"`
	// Pushed means some of these commits are already on Upstream, so a force push will be
	// needed afterwards.
	Pushed   bool   `json:"pushed"`
	Upstream string `json:"upstream"`
}

func forecastConflicts(in []git.ForecastConflict) []ForecastConflict {
	out := make([]ForecastConflict, 0, len(in))
	for _, c := range in {
		out = append(out, ForecastConflict{Path: c.Path, Places: c.Places})
	}
	return out
}

func forecastCommits(in []git.ForecastCommit) []ForecastCommit {
	out := make([]ForecastCommit, 0, len(in))
	for _, c := range in {
		out = append(out, ForecastCommit{ID: c.ID, Subject: c.Subject, Author: c.Author, FixupOf: c.FixupOf})
	}
	return out
}

// currentBranch returns the checked-out branch's name, or "" when HEAD is detached.
func currentBranch(ctx context.Context, g *git.Repo) string {
	name, err := g.ReadString(ctx, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return ""
	}
	return name
}

// PreviewMerge says what merging source into the checked-out branch would do.
func (a *App) PreviewMerge(path, source string) (MergePreview, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return MergePreview{}, err
	}
	if strings.TrimSpace(source) == "" {
		return MergePreview{}, userError("Choose what to merge.")
	}
	ctx := context.Background()
	f, err := r.Git.ForecastMerge(ctx, "HEAD", source)
	if err != nil {
		return MergePreview{}, friendly(err)
	}
	return MergePreview{
		Into: currentBranch(ctx, r.Git), AlreadyMerged: f.AlreadyMerged, FastForward: f.FastForward,
		Commits: forecastCommits(f.Commits), CommitCount: f.CommitCount, Files: f.Files,
		Conflicts: forecastConflicts(f.Conflicts), Unknown: f.Incomplete,
	}, nil
}

// PreviewRebase says what rebasing the checked-out branch onto another commit would do.
func (a *App) PreviewRebase(path, onto string) (RebasePreview, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return RebasePreview{}, err
	}
	if strings.TrimSpace(onto) == "" {
		return RebasePreview{}, userError("Choose what to rebase onto.")
	}
	ctx := context.Background()
	branch := currentBranch(ctx, r.Git)
	// A named branch has an upstream to check; a detached HEAD is just a commit.
	tip := branch
	if tip == "" {
		tip = "HEAD"
	}
	f, err := r.Git.ForecastRebase(ctx, tip, onto)
	if err != nil {
		return RebasePreview{}, friendly(err)
	}
	p := RebasePreview{
		Branch: branch, Commits: forecastCommits(f.Commits), CommitCount: f.CommitCount, UpToDate: f.UpToDate,
		StopsAt: f.StopsAt, Conflicts: forecastConflicts(f.Conflicts), Incomplete: f.Incomplete, Pushed: f.Pushed, Upstream: f.Upstream,
	}
	for i, c := range p.Commits {
		if c.FixupOf != "" {
			p.Fixups++
		}
		if c.ID == p.StopsAt {
			p.StopsAtSubject, p.StopsAtNumber = c.Subject, i+1
		}
	}
	return p, nil
}
