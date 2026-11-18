package git

import "context"

// HeadState is where HEAD is: on a branch at a commit, or detached at a commit.
type HeadState struct {
	// Branch is the checked-out branch, or "" when detached.
	Branch string `json:"branch"`
	// Commit is HEAD's commit, or "" before the first commit.
	Commit string `json:"commit"`
}

// Head reads the current HEAD state.
func (r *Repo) Head(ctx context.Context) HeadState {
	var h HeadState
	h.Branch, _ = r.ReadString(ctx, "symbolic-ref", "--short", "-q", "HEAD")
	h.Commit, _ = r.ReadString(ctx, "rev-parse", "-q", "--verify", "HEAD")
	return h
}

// ResetKeep moves HEAD to target like a hard reset, but refuses if that would overwrite
// local changes in files that differ between HEAD and target.
func (r *Repo) ResetKeep(ctx context.Context, target string) error {
	if err := CheckRevs(target); err != nil {
		return err
	}
	_, err := r.Write(ctx, Cmd{Args: []string{"reset", "-q", "--keep", target, "--"}})
	return err
}
