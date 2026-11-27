package git

import (
	"context"
	"errors"
	"fmt"
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

// Diverged counts the commits each of a and b has that the other hasn't: `rev-list
// --left-right --count a...b`. In a shallow clone it counts what's here.
func (r *Repo) Diverged(ctx context.Context, a, b string) (onlyA, onlyB int, err error) {
	if err := CheckRevs(a, b); err != nil {
		return 0, 0, err
	}
	out, err := r.ReadString(ctx, "rev-list", "--left-right", "--count", a+"..."+b)
	if err != nil {
		return 0, 0, err
	}
	if _, err := fmt.Sscanf(out, "%d %d", &onlyA, &onlyB); err != nil {
		return 0, 0, fmt.Errorf("reading rev-list's counts %q: %w", out, err)
	}
	return onlyA, onlyB, nil
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
