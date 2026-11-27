package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DiffTool is the tool git difftool would open (#939), and the config key that names it.
type DiffTool struct {
	// Name is the tool, such as "meld" or "vscode"; "" when none is set.
	Name string `json:"name"`
	// Key is where it's set: diff.guitool, diff.tool or merge.tool.
	Key string `json:"key"`
}

// DiffToolSet reads which tool git difftool opens, as git chooses it for an app with a window:
// diff.guitool, else diff.tool, else merge.tool, which difftool falls back to as well.
func (r *Repo) DiffToolSet(ctx context.Context) DiffTool {
	for _, key := range []string{"diff.guitool", "diff.tool", "merge.tool"} {
		if name, _ := r.ReadString(ctx, "config", "--get", key); name != "" {
			return DiffTool{Name: name, Key: key}
		}
	}
	return DiffTool{}
}

// ErrNoDiffTool means no diff tool is set.
var ErrNoDiffTool = errors.New("no diff tool is configured")

// DiffToolTarget is the comparison to open: the one the diff view shows.
type DiffToolTarget struct {
	// Mode is "unstaged" (the index against the working tree), "staged" (HEAD against the
	// index), "commit" (a commit against its first parent) or "compare" (Commit is
	// "FROM..TO"; FROM "" is nothing).
	Mode    string `json:"mode"`
	File    string `json:"file"`
	OldPath string `json:"oldPath"`
	Commit  string `json:"commit"`
}

// RunDiffTool opens one file's comparison in the user's diff tool and waits for it to close.
// A commit with no parent here (a first commit, or a shallow clone's edge) is compared with the
// empty tree, as the diff view shows it. It never prompts.
func (r *Repo) RunDiffTool(ctx context.Context, t DiffToolTarget) error {
	tool := r.DiffToolSet(ctx)
	if tool.Name == "" {
		return ErrNoDiffTool
	}
	if t.File == "" {
		return errors.New("no file to compare")
	}
	args := []string{"difftool", "--no-prompt", "--tool=" + tool.Name}
	switch t.Mode {
	case "unstaged":
	case "staged":
		args = append(args, "--cached")
	case "commit":
		if err := CheckRevs(t.Commit); err != nil {
			return err
		}
		from, err := r.parentOrEmpty(ctx, t.Commit)
		if err != nil {
			return err
		}
		args = append(args, from, t.Commit)
	case "compare":
		from, to, ok := strings.Cut(t.Commit, "..")
		if !ok || to == "" {
			return fmt.Errorf("%q isn't two commits", t.Commit)
		}
		if err := CheckRevs(from, to); err != nil {
			return err
		}
		if from == "" {
			var err error
			if from, err = r.emptyTree(ctx); err != nil {
				return err
			}
		}
		args = append(args, from, to)
	default:
		return fmt.Errorf("difftool can't open a %q comparison", t.Mode)
	}
	// Both names of a renamed file, so git pairs them as it does in the view.
	args = append(args, "--", t.File)
	if t.OldPath != "" && t.OldPath != t.File {
		args = append(args, t.OldPath)
	}
	_, err := r.runner.Run(ctx, Cmd{Dir: r.dir, Args: args})
	return err
}

// parentOrEmpty is commit's first parent, or the empty tree for a commit without one here.
func (r *Repo) parentOrEmpty(ctx context.Context, commit string) (string, error) {
	if parent, err := r.ReadString(ctx, "rev-parse", "--verify", "-q", commit+"^1"); err == nil && parent != "" {
		// A shallow clone's edge names a parent it hasn't got.
		if _, err := r.ReadString(ctx, "cat-file", "-e", parent+"^{commit}"); err == nil {
			return parent, nil
		}
	}
	return r.emptyTree(ctx)
}

// emptyTree is the id of the tree with nothing in it, in this repository's hash: git knows it
// without its being stored.
func (r *Repo) emptyTree(ctx context.Context) (string, error) {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"hash-object", "-t", "tree", "--stdin"}, Stdin: strings.NewReader("")})
	return strings.TrimSpace(string(out)), err
}
