package app

import (
	"context"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// RepoStorage is how a repository keeps its objects, for the Repository panel (#937).
type RepoStorage struct {
	git.Storage
	// Bytes is everything the objects take on disk.
	Bytes int64 `json:"bytes"`
	// Due says git's own thresholds are passed (gc.auto, gc.autoPackLimit): the panel then
	// suggests maintenance, and otherwise only offers it.
	Due bool `json:"due"`
}

// RepoStorage reads how the repository keeps its objects, and whether git maintenance has it
// on its schedule.
func (a *App) RepoStorage(path string) (RepoStorage, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return RepoStorage{}, err
	}
	s, err := r.Git.Storage(context.Background())
	if err != nil {
		return RepoStorage{}, friendly(err)
	}
	return RepoStorage{Storage: s, Bytes: s.Bytes(), Due: s.Due()}, nil
}

// RunMaintenance runs git maintenance run in the background, as a network operation does: the
// toolbar shows it with Stop, and nothing else of the kind starts meanwhile. A prefetch task
// talks to the remotes, so it can ask for credentials as a fetch would.
func (a *App) RunMaintenance(path string) error {
	return a.runNetwork(path, "maintenance", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		return r.Git.RunMaintenance(ctx, o)
	})
}

// ScheduleMaintenance puts the repository on git maintenance's schedule (git maintenance
// start) or takes it off (git maintenance unregister).
func (a *App) ScheduleMaintenance(path string, on bool) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return friendly(r.Git.ScheduleMaintenance(context.Background(), on))
}
