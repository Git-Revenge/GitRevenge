package git

import (
	"context"
	"errors"
	"strings"
)

// RevertAll reverts commits one after another in the order given, each with its own revert
// commit: `git revert --no-edit A B C`. Newest first undoes them cleanly. A merge needs to be
// told which parent to keep, so merges go through Revert one at a time.
func (r *Repo) RevertAll(ctx context.Context, commits ...string) error {
	if len(commits) == 0 {
		return errors.New("no commits to revert")
	}
	if err := CheckRevs(commits...); err != nil {
		return err
	}
	return r.conflictAware(ctx, Cmd{Args: append([]string{"revert", "--no-edit"}, commits...), Env: []string{"GIT_EDITOR=true"}})
}

// FilesByCommit returns the files each commit changes against its parent, by commit ID, from
// one diff-tree for them all. It reads trees only, so a partial clone downloads nothing.
// Renames show as a delete and an add. Merges, root commits and a shallow clone's edge
// commits have no entry, and neither has a commit that changes nothing.
func (r *Repo) FilesByCommit(ctx context.Context, commits []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(commits) == 0 {
		return out, nil
	}
	if err := CheckRevs(commits...); err != nil {
		return nil, err
	}
	raw, err := r.readCmd(ctx, Cmd{
		Args:     []string{"diff-tree", "--stdin", "-r", "-z", "--name-only", "--no-renames", "--format=%x01%H"},
		Stdin:    strings.NewReader(strings.Join(commits, "\n") + "\n"),
		Env:      noLazyFetch,
		ReadOnly: true,
	})
	if err != nil {
		return nil, err
	}
	// Each commit is \x01, its ID, then its files, each ended by NUL.
	for chunk := range strings.SplitSeq(string(raw), "\x01") {
		end := strings.IndexAny(chunk, "\x00\n")
		if end <= 0 {
			continue
		}
		id := chunk[:end]
		for f := range strings.SplitSeq(strings.TrimLeft(chunk[end:], "\x00\n"), "\x00") {
			if f != "" {
				out[id] = append(out[id], f)
			}
		}
	}
	return out, nil
}
