package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// LFSInfo is what the LFS section of the sidebar shows.
type LFSInfo struct {
	// Available is false when git-lfs isn't installed.
	Available bool          `json:"available"`
	Status    git.LFSStatus `json:"status"`
}

// LFS returns tracked patterns and LFS files, or Available=false without git-lfs.
func (a *App) LFS(path string) (LFSInfo, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return LFSInfo{}, err
	}
	st, err := r.Git.LFS(context.Background())
	if errors.Is(err, git.ErrNoLFS) {
		return LFSInfo{}, nil
	}
	return LFSInfo{Available: err == nil, Status: st}, err
}

// LFSAction changes LFS tracking or locks: track (pattern, lockable "true"/""),
// untrack (pattern), lock (path), unlock (path, force "true"/""). Locks talk to the server.
func (a *App) LFSAction(path, action string, args []string) error {
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	switch action {
	case "track", "untrack":
		r, err := a.openRepo(path)
		if err != nil {
			return err
		}
		if action == "track" {
			return writeErr(r.Git.LFSTrack(context.Background(), arg(0), arg(1) == "true"))
		}
		return writeErr(r.Git.LFSUntrack(context.Background(), arg(0)))
	case "lock":
		return a.runNetwork(path, "lfs", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
			return r.Git.LFSLock(ctx, arg(0), o)
		})
	case "unlock":
		return a.runNetwork(path, "lfs", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
			return r.Git.LFSUnlock(ctx, arg(0), arg(1) == "true", o)
		})
	}
	return userError("Unknown LFS action " + action + ".")
}

// LFSLocks asks the LFS server which files are locked.
func (a *App) LFSLocks(path string) ([]git.LFSLock, error) {
	var locks []git.LFSLock
	err := a.runNetwork(path, "lfs", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		var err error
		locks, err = r.Git.LFSLocks(ctx, o)
		return err
	})
	if locks == nil {
		locks = []git.LFSLock{}
	}
	return locks, err
}

// Submodules lists the repository's submodules (recursively).
func (a *App) Submodules(path string) ([]git.Submodule, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	return r.Git.Submodules(context.Background())
}

// UpdateSubmodules initializes and checks out every submodule at its recorded commit.
func (a *App) UpdateSubmodules(path string) error {
	return a.runNetwork(path, "submodules", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		return r.Git.UpdateSubmodules(ctx, o)
	})
}
