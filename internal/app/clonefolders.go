package app

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Choosing folders at clone time (#1004). A clone with Options › Folders › "Only the ones I
// choose" brings the history and folder listings and writes no file. The dialog's second step
// lists the folders (CloneFolders), then writes the ones chosen (CloneCheckout), or takes
// the clone away again (CloneAbandon). Only a clone this app made that way can be finished
// or taken away: a path from the window is never trusted to be one.

// halfClone is a clone waiting for its folders.
type halfClone struct {
	// existed says the folder was there, empty, before the clone: taking the clone away
	// empties it rather than removing it.
	existed bool
	// submodules says the clone was asked to bring them, which needs files written first.
	submodules bool
}

type halfClones struct {
	mu sync.Mutex
	m  map[string]halfClone
}

func (h *halfClones) add(dir string, c halfClone) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.m == nil {
		h.m = map[string]halfClone{}
	}
	h.m[filepath.Clean(dir)] = c
}

func (h *halfClones) get(dir string) (halfClone, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.m[filepath.Clean(dir)]
	return c, ok
}

func (h *halfClones) remove(dir string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.m, filepath.Clean(dir))
}

// notHalfCloned is the answer for a folder this app didn't clone without its files.
const notHalfCloned = userError("That clone isn't waiting for its folders.")

// CloneFolders lists the folders of a clone waiting for them, with how many files each holds,
// from its folder listings: nothing is downloaded or written.
func (a *App) CloneFolders(dir string) ([]git.FolderCount, error) {
	if _, ok := a.halfCloned.get(dir); !ok {
		return nil, notHalfCloned
	}
	runner, err := a.runner()
	if err != nil {
		return nil, err
	}
	return git.OpenRepo(runner, dir).FolderCounts(context.Background(), "")
}

// CloneCheckout writes the chosen folders of a clone waiting for them, in cone mode, or every
// folder when none are named, then brings its submodules if it was asked to.
func (a *App) CloneCheckout(dir string, folders []string) error {
	c, ok := a.halfCloned.get(dir)
	if !ok {
		return notHalfCloned
	}
	runner, err := a.runner()
	if err != nil {
		return err
	}
	repo := git.OpenRepo(runner, dir)
	if err := repo.CheckOutFolders(context.Background(), folders); err != nil {
		return err
	}
	a.halfCloned.remove(dir)
	if !c.submodules {
		return nil
	}
	// A clone with no files has no submodules to clone yet: they come now, from the folders
	// written, with the clone's own prompts.
	hosts := func() []string { return remoteHosts(context.Background(), repo) }
	return a.track(cloneKey, dir, "clone", hosts, "A clone is already running.", func(ctx context.Context, o git.NetOptions) error {
		return repo.UpdateSubmodules(ctx, o)
	})
}

// CloneAbandon takes away a clone waiting for its folders: the dialog was closed before they
// were chosen, so nothing it made is left behind.
func (a *App) CloneAbandon(dir string) error {
	c, ok := a.halfCloned.get(dir)
	if !ok {
		return notHalfCloned
	}
	a.halfCloned.remove(dir)
	removeClone(dir, c.existed)
	return nil
}
