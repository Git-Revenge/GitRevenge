package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Operation is a multi-step git operation that can be left in progress.
type Operation string

// Operations that can be in progress. None means the repository is idle.
const (
	None       Operation = ""
	Merge      Operation = "merge"
	Rebase     Operation = "rebase"
	CherryPick Operation = "cherry-pick"
	Revert     Operation = "revert"
	Bisect     Operation = "bisect"
	// Am is a patch series being applied with `git am`.
	Am Operation = "am"
)

// OperationState describes the operation in progress, if any.
type OperationState struct {
	Op Operation
	// Step and Total give rebase progress (1-based); both are zero when unknown.
	Step, Total int
	// Conflicts lists unmerged paths, relative to the work tree.
	Conflicts []string
	// Ours is the branch the operation is changing, or "" when HEAD was detached. Theirs is
	// the other side: what is being merged in, or what a rebase is onto. It is "" for a
	// cherry-pick or a revert, which have only the commit in hand.
	Ours, Theirs string
	// Commit is the commit in hand, with its Subject: the one a rebase stopped on, the one
	// being merged, cherry-picked or reverted.
	Commit, Subject string
	// OrigHead is where the branch was before the operation, which is where aborting returns it.
	OrigHead string
	// Written are the commits a rebase has written so far, newest first, up to a cap.
	Written []string
}

// DetectOperation reports which operation is in progress, by checking the state files git
// leaves in gitDir. It works whether the operation was started by the app or elsewhere.
func DetectOperation(gitDir string) (Operation, int, int) {
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(gitDir, name))
		return err == nil
	}
	switch {
	case exists("rebase-merge"):
		return Rebase, readInt(gitDir, "rebase-merge/msgnum"), readInt(gitDir, "rebase-merge/end")
	case exists("rebase-apply/applying"):
		return Am, readInt(gitDir, "rebase-apply/next"), readInt(gitDir, "rebase-apply/last")
	case exists("rebase-apply"):
		return Rebase, readInt(gitDir, "rebase-apply/next"), readInt(gitDir, "rebase-apply/last")
	case exists("MERGE_HEAD"):
		return Merge, 0, 0
	case exists("CHERRY_PICK_HEAD"):
		return CherryPick, 0, 0
	case exists("REVERT_HEAD"):
		return Revert, 0, 0
	case exists("BISECT_LOG"):
		return Bisect, 0, 0
	}
	return None, 0, 0
}

func readInt(gitDir, name string) int {
	b, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(name)))
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}

// OperationState returns the operation in progress and any conflicted paths.
func (r *Repo) OperationState(ctx context.Context, gitDir string) (OperationState, error) {
	op, step, total := DetectOperation(gitDir)
	st := OperationState{Op: op, Step: step, Total: total, Conflicts: []string{}}
	out, err := r.Read(ctx, "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return st, err
	}
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			st.Conflicts = append(st.Conflicts, p)
		}
	}
	r.describe(ctx, gitDir, &st)
	return st, nil
}

// Action is something the user can do to an in-progress operation.
type Action string

// Actions on an in-progress operation.
const (
	Continue Action = "continue"
	Abort    Action = "abort"
	Skip     Action = "skip"
)

// Actions lists what is valid for op.
func (op Operation) Actions() []Action {
	switch op {
	case Merge:
		return []Action{Continue, Abort}
	case Rebase, CherryPick, Revert, Am:
		return []Action{Continue, Skip, Abort}
	case Bisect:
		return []Action{Abort}
	}
	return nil
}

// ErrActionNotAllowed means the action does not apply to the operation in progress.
type ErrActionNotAllowed struct {
	Op     Operation
	Action Action
}

func (e *ErrActionNotAllowed) Error() string {
	if e.Op == None {
		return "no operation is in progress"
	}
	return "cannot " + string(e.Action) + " a " + string(e.Op)
}

// RunAction continues, skips or aborts the operation in progress. Continuing keeps git's
// prepared commit message as is (GIT_EDITOR=true): there is no in-app editor yet.
func (r *Repo) RunAction(ctx context.Context, gitDir string, action Action) error {
	op, _, _ := DetectOperation(gitDir)
	allowed := false
	for _, a := range op.Actions() {
		allowed = allowed || a == action
	}
	if !allowed {
		return &ErrActionNotAllowed{Op: op, Action: action}
	}

	var args []string
	switch {
	case op == Bisect:
		args = []string{"bisect", "reset"}
	case op == Merge && action == Continue:
		// `merge --continue` refuses to run if hooks or config would open an editor in
		// some versions; committing the prepared merge is equivalent and portable.
		args = []string{"commit", "--no-edit"}
	default:
		args = []string{string(op), "--" + string(action)}
	}
	_, err := r.Write(ctx, Cmd{Args: args, Env: []string{"GIT_EDITOR=true"}})
	return err
}
