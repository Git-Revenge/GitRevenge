package app

import (
	"context"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// maxPathSuggestions is how many folders and files the box under "Focus the timeline on a
// folder or file" offers at once.
const maxPathSuggestions = 12

// treePaths keeps the last repository's folders and files, for its commit: typing in the box
// asks on every key, and listing the Linux kernel's 90,000 paths takes a moment each time.
var treePaths struct {
	sync.Mutex
	repo, commit string
	paths        []string
}

// FocusSuggestions offers the folders and files at HEAD of the repository at path that match
// what's typed (#1060), folders ending in a slash, for focusing the timeline on one. Nothing
// typed offers the top-level folders.
func (a *App) FocusSuggestions(path, typed string) ([]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	head := r.Git.Head(ctx).Commit
	if head == "" {
		return []string{}, nil
	}
	treePaths.Lock()
	defer treePaths.Unlock()
	if treePaths.repo != r.Path || treePaths.commit != head {
		paths, err := r.Git.TreePaths(ctx, head)
		if err != nil {
			return nil, friendly(err)
		}
		treePaths.repo, treePaths.commit, treePaths.paths = r.Path, head, paths
	}
	return git.MatchPaths(treePaths.paths, typed, maxPathSuggestions), nil
}
