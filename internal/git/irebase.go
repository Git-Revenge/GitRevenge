package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RebaseItem is one commit in an interactive rebase plan.
type RebaseItem struct {
	Commit  string `json:"commit"`
	Subject string `json:"subject"`
	// Message is the full message: the original when planning; the new one for reword and
	// squash when running.
	Message string `json:"message"`
	// Action is pick, reword, squash, fixup or drop.
	Action string `json:"action"`
}

// ErrMergesInRange means the commits to rebase include merges, which an interactive rebase
// would flatten.
var ErrMergesInRange = errors.New("range contains merge commits")

// RebasePlan lists the commits after base up to HEAD, oldest first, each set to pick.
func (r *Repo) RebasePlan(ctx context.Context, base string) ([]RebaseItem, error) {
	if err := CheckRevs(base); err != nil {
		return nil, err
	}
	if merges, err := r.ReadString(ctx, "rev-list", "--merges", base+"..HEAD"); err != nil {
		return nil, err
	} else if merges != "" {
		return nil, ErrMergesInRange
	}
	out, err := r.Read(ctx, "log", "--reverse", "--topo-order", "-z", "--format=%H%x1f%s%x1f%B", base+"..HEAD", "--")
	if err != nil {
		return nil, err
	}
	items := []RebaseItem{}
	for rec := range strings.SplitSeq(string(out), "\x00") {
		f := strings.SplitN(strings.TrimLeft(rec, "\n"), "\x1f", 3)
		if len(f) != 3 {
			continue
		}
		items = append(items, RebaseItem{Commit: f[0], Subject: f[1], Message: strings.TrimSpace(f[2]), Action: "pick"})
	}
	return items, nil
}

// InteractiveRebase rewrites the commits after base as described by items (oldest first).
//
// No editor is ever opened: the todo list is written by the app and copied into place via
// GIT_SEQUENCE_EDITOR, and new messages for reword/squash are applied with an
// "exec git commit --amend -F <file>" line after the commit, so commit signing still applies.
// If it stops with conflicts it returns ErrConflicts and the rebase stays in progress.
func (r *Repo) InteractiveRebase(ctx context.Context, base string, items []RebaseItem) error {
	if len(items) == 0 {
		return errors.New("nothing to rebase")
	}
	// Each commit becomes a line of the todo file, where a line break would start a line of
	// its own, and "exec" is a line git runs. Only whole commit ids go in.
	for _, it := range items {
		if !isObjectID(it.Commit) {
			return fmt.Errorf("%q isn't a commit id", it.Commit)
		}
	}
	if base == "" || strings.HasPrefix(base, "-") || strings.ContainsAny(base, " \t\r\n") {
		return fmt.Errorf("%q isn't a commit to rebase from", base)
	}
	dir, err := os.MkdirTemp("", "gitrevenge-rebase-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	var todo strings.Builder
	kept := false
	for i, it := range items {
		switch it.Action {
		case "drop":
			fmt.Fprintf(&todo, "drop %s\n", it.Commit)
			continue
		case "squash", "fixup":
			if !kept {
				return fmt.Errorf("%s can't be the first commit: there is nothing to combine it into", it.Action)
			}
		case "pick", "reword":
		default:
			return fmt.Errorf("unknown rebase action %q", it.Action)
		}
		verb := "pick"
		if it.Action == "squash" || it.Action == "fixup" {
			verb = "fixup"
		}
		fmt.Fprintf(&todo, "%s %s\n", verb, it.Commit)
		kept = true
		if it.Action == "reword" || it.Action == "squash" {
			if strings.TrimSpace(it.Message) == "" {
				return fmt.Errorf("%s of %.7s needs a message", it.Action, it.Commit)
			}
			msg := filepath.Join(dir, fmt.Sprintf("msg-%d", i))
			if err := os.WriteFile(msg, []byte(it.Message), 0o600); err != nil {
				return err
			}
			fmt.Fprintf(&todo, "exec git commit --amend --no-edit --allow-empty --cleanup=whitespace -F %s\n", shellQuote(msg))
		}
	}
	if !kept {
		return errors.New("every commit is dropped; use reset instead")
	}
	todoFile := filepath.Join(dir, "todo")
	if err := os.WriteFile(todoFile, []byte(todo.String()), 0o600); err != nil {
		return err
	}
	return r.conflictAware(ctx, Cmd{
		Args: []string{"rebase", "-i", base},
		Env: []string{
			// git runs this through sh (bundled with Git for Windows) as: <cmd> <todo-path>.
			"GIT_SEQUENCE_EDITOR=cp -- " + shellQuote(todoFile),
			"GIT_EDITOR=true",
		},
	})
}

// isObjectID reports whether s is a whole object id as git prints it: 40 lower-case hex digits
// (SHA-1) or 64 (SHA-256).
func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range []byte(s) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// shellQuote quotes a path for the sh that git uses to run editors and exec lines. Forward
// slashes keep Windows paths intact inside sh.
func shellQuote(p string) string {
	return "'" + strings.ReplaceAll(filepath.ToSlash(p), "'", `'\''`) + "'"
}
