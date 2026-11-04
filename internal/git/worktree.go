package git

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/fsutil"
)

// Worktree is a working tree of the repository: the main one or a linked one.
type Worktree struct {
	Path string `json:"path"`
	// Head is the checked-out commit; empty for a bare entry.
	Head string `json:"head"`
	// Branch is the checked-out branch's short name, or "" when HEAD is detached.
	Branch   string `json:"branch"`
	Main     bool   `json:"main"`
	Locked   bool   `json:"locked"`
	Prunable bool   `json:"prunable"`
	// Current is the worktree this repository is open in.
	Current bool `json:"current"`
}

// Worktrees lists the main worktree and every linked one.
func (r *Repo) Worktrees(ctx context.Context) ([]Worktree, error) {
	out, err := r.ReadString(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	// Compare with git's own spelling of this worktree's path: r.dir may differ from it by
	// a symlink (macOS /var → /private/var) or a Windows short name (RUNNER~1).
	top, err := r.ReadString(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		top = r.dir
	}
	return parseWorktrees(out, filepath.FromSlash(strings.TrimSpace(top))), nil
}

// parseWorktrees reads `git worktree list --porcelain`: blank-line separated records of
// "worktree <path>", "HEAD <sha>", "branch <ref>" or "detached", and optional "locked",
// "prunable" and "bare" lines. The first record is the main worktree.
func parseWorktrees(out, current string) []Worktree {
	list := []Worktree{}
	for i, rec := range strings.Split(strings.TrimSpace(strings.ReplaceAll(out, "\r\n", "\n")), "\n\n") {
		var w Worktree
		for line := range strings.SplitSeq(rec, "\n") {
			key, val, _ := strings.Cut(line, " ")
			switch key {
			case "worktree":
				w.Path = val
			case "HEAD":
				w.Head = val
			case "branch":
				w.Branch = strings.TrimPrefix(val, "refs/heads/")
			case "locked":
				w.Locked = true
			case "prunable":
				w.Prunable = true
			}
		}
		if w.Path == "" {
			continue
		}
		w.Main = i == 0
		w.Current = fsutil.SamePath(filepath.FromSlash(w.Path), current)
		list = append(list, w)
	}
	return list
}

// AddWorktree checks out a new linked worktree at path: newBranch (created at HEAD) when
// set, else the existing branch, else a detached HEAD.
func (r *Repo) AddWorktree(ctx context.Context, path, branch, newBranch string) error {
	args := []string{"worktree", "add"}
	switch {
	case newBranch != "":
		args = append(args, "-b", newBranch, "--", path)
	case branch != "":
		args = append(args, "--", path, branch)
	default:
		args = append(args, "--detach", "--", path)
	}
	_, err := r.Write(ctx, Cmd{Args: args})
	return err
}

// RemoveWorktree removes a linked worktree; force also removes one with local changes.
func (r *Repo) RemoveWorktree(ctx context.Context, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	_, err := r.Write(ctx, Cmd{Args: append(args, "--", path)})
	return err
}
