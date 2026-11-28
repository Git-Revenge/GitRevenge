package git

import (
	"context"
	"slices"
	"strings"
)

// TreePaths lists every folder and file at rev, folders ending in a slash, from its listing
// alone: a partial clone downloads nothing. It's what a box that suggests the repository's
// folders and files reads (#1060).
func (r *Repo) TreePaths(ctx context.Context, rev string) ([]string, error) {
	if err := CheckRevs(rev); err != nil {
		return nil, err
	}
	if rev == "" {
		rev = "HEAD"
	}
	out, err := r.readCmd(ctx, Cmd{Args: []string{"ls-tree", "-r", "-t", "-z", "--name-only", rev}, Env: noLazyFetch})
	if err != nil {
		return nil, err
	}
	// -t names each folder before what's in it; a folder is known by having something under it.
	all := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	paths := make([]string, 0, len(all))
	for i, p := range all {
		if p == "" {
			continue
		}
		if i+1 < len(all) && strings.HasPrefix(all[i+1], p+"/") {
			p += "/"
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// MatchPaths picks up to limit of paths for what's typed, ignoring case: a path that starts
// with it first, then one with a folder or name that does, then any that holds it. Within
// each, folders before files and shallower before deeper. Nothing typed offers the top-level
// folders.
func MatchPaths(paths []string, typed string, limit int) []string {
	q := strings.ToLower(strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(typed), `\`, "/"), "/"))
	type hit struct {
		path        string
		rank, depth int
		folder      bool
	}
	var hits []hit
	for _, p := range paths {
		folder := strings.HasSuffix(p, "/")
		depth := strings.Count(strings.TrimSuffix(p, "/"), "/")
		if q == "" {
			if folder && depth == 0 {
				hits = append(hits, hit{p, 0, 0, true})
			}
			continue
		}
		low := strings.ToLower(p)
		rank := -1
		switch {
		case strings.HasPrefix(low, q):
			rank = 0
		case strings.Contains(low, "/"+q):
			rank = 1
		case strings.Contains(low, q):
			rank = 2
		}
		if rank >= 0 {
			hits = append(hits, hit{p, rank, depth, folder})
		}
	}
	slices.SortFunc(hits, func(a, b hit) int {
		if a.rank != b.rank {
			return a.rank - b.rank
		}
		if a.folder != b.folder {
			if a.folder {
				return -1
			}
			return 1
		}
		if a.depth != b.depth {
			return a.depth - b.depth
		}
		return strings.Compare(a.path, b.path)
	})
	out := make([]string, 0, min(limit, len(hits)))
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, h.path)
	}
	return out
}
