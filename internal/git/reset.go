package git

import (
	"context"
	"fmt"
	"strconv"
)

// ResetMode chooses what Reset keeps.
type ResetMode string

// Reset modes.
const (
	// ResetSoft moves the branch only; changes stay staged.
	ResetSoft ResetMode = "soft"
	// ResetMixed moves the branch and unstages; changes stay in the work tree.
	ResetMixed ResetMode = "mixed"
	// ResetHard moves the branch and discards all local changes to tracked files.
	ResetHard ResetMode = "hard"
)

// Reset moves the current branch (or detached HEAD) to target.
func (r *Repo) Reset(ctx context.Context, target string, mode ResetMode) error {
	if err := CheckRevs(target); err != nil {
		return err
	}
	switch mode {
	case ResetSoft, ResetMixed, ResetHard:
	default:
		return fmt.Errorf("unknown reset mode %q", mode)
	}
	_, err := r.Write(ctx, Cmd{Args: []string{"reset", "-q", "--" + string(mode), target, "--"}})
	return err
}

// Revert creates a commit undoing commit. For a merge commit, mainline (1-based) picks the
// parent whose side is kept; it is ignored for ordinary commits.
func (r *Repo) Revert(ctx context.Context, commit string, mainline int) error {
	if err := CheckRevs(commit); err != nil {
		return err
	}
	args := []string{"revert", "--no-edit"}
	if mainline > 0 {
		args = append(args, "-m", strconv.Itoa(mainline))
	}
	args = append(args, commit)
	return r.conflictAware(ctx, Cmd{Args: args, Env: []string{"GIT_EDITOR=true"}})
}
