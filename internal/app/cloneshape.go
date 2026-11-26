package app

import (
	"context"
	"errors"
	"slices"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// This clone, in the Repository panel (#1005): how a clone differs from a whole one, and the
// way to the rest. Each action runs as a fetch does, with progress in the toolbar, Stop, and
// the graph and lists updating when it lands.

// CloneShape says how the repository's clone differs from a whole one: shallow, partial,
// sparse or one branch, whoever made it.
func (a *App) CloneShape(path string) (git.CloneShape, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.CloneShape{}, err
	}
	s, err := r.Git.CloneShape(context.Background())
	return s, friendly(err)
}

// FetchMoreHistory fetches more of a shallow clone's history: more.Commits more, back to
// more.Since, or all of it.
func (a *App) FetchMoreHistory(path string, more git.MoreHistory) error {
	err := a.runNetwork(path, "fetch", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		return r.Git.FetchMoreHistory(ctx, cloneRemote(ctx, r.Git), more, o)
	})
	if errors.Is(err, git.ErrNotShallow) {
		return userError("This clone already has all of its history.")
	}
	return err
}

// BackfillCost says what downloading every file would take: the clone's size now, and how
// many versions it would download. Counting walks the whole history, so it can take a while;
// asking again, or StopBackfillCost, stops the count before.
func (a *App) BackfillCost(path string) (git.BackfillCost, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.BackfillCost{}, err
	}
	ctx, done := a.latest.start("backfill-cost", r.Path)
	defer done()
	cost, err := r.Git.BackfillCost(ctx)
	if ctx.Err() != nil {
		return git.BackfillCost{}, nil // stopped: nobody is waiting for it
	}
	return cost, friendly(err)
}

// StopBackfillCost stops a count BackfillCost is making, for a dialog closed before it ended.
func (a *App) StopBackfillCost(path string) {
	if r, err := a.openRepo(path); err == nil {
		a.latest.stop("backfill-cost", r.Path)
	}
}

// Backfill downloads every version of every file in the checked-out branch's history that a
// partial clone hasn't got (in a sparse checkout, its folders'), in batches.
func (a *App) Backfill(path string) error {
	err := a.runNetwork(path, "download", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		return r.Git.Backfill(ctx, o)
	})
	if errors.Is(err, git.ErrNotPartial) {
		return userError("This clone isn't partial: it has every file already.")
	}
	return err
}

// SetSparseFolders changes which folders a sparse checkout writes, or writes every folder
// when folders is empty. In a partial clone the files it adds are downloaded.
func (a *App) SetSparseFolders(path string, folders []string) error {
	err := a.runNetwork(path, "download", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		return r.Git.SetSparseFolders(ctx, folders, o)
	})
	if errors.Is(err, git.ErrCloneOptions) {
		return userError("Choose folders inside the repository.")
	}
	return err
}

// IsShallowEdge says whether a commit is at a shallow clone's edge (#1006): its parents
// weren't fetched, so a history that reaches it stops there, not where the project began.
func (a *App) IsShallowEdge(path, id string) bool {
	r, err := a.openRepo(path)
	if err != nil {
		return false
	}
	return r.Git.IsShallowEdge(context.Background(), id)
}

// RepoFolders lists the checked-out commit's folders with how many files each holds, for
// changing a sparse checkout's folders. It reads the folder listings only.
func (a *App) RepoFolders(path string) ([]git.FolderCount, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	counts, err := r.Git.FolderCounts(context.Background(), "HEAD")
	return counts, friendly(err)
}

// FetchEveryBranch has a one-branch remote fetch every branch, and fetches them.
func (a *App) FetchEveryBranch(path, remote string) error {
	return a.runNetwork(path, "fetch", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		return r.Git.FetchEveryBranch(ctx, remote, o)
	})
}

// cloneRemote is the remote a clone's missing history comes from: the one it fetches one
// branch from, else its partial clone's, else the one the current branch pulls from.
func cloneRemote(ctx context.Context, g *git.Repo) string {
	if s, err := g.CloneShape(ctx); err == nil {
		if s.OneBranch != nil {
			return s.OneBranch.Remote
		}
		if s.Partial != nil {
			return s.Partial.Remote
		}
	}
	remote := pullRemote(ctx, g)
	if rs, err := g.Remotes(ctx); err == nil && len(rs) > 0 && !slices.ContainsFunc(rs, func(r git.Remote) bool { return r.Name == remote }) {
		return rs[0].Name
	}
	return remote
}
