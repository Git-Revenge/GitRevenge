package git

import (
	"context"
	"errors"
)

// ErrConflicts means the operation stopped with conflicts. The repository is left in the
// operation's in-progress state (see DetectOperation) for the user to resolve, continue
// or abort.
var ErrConflicts = errors.New("stopped with conflicts")

// MergeMode chooses how Merge records the result.
type MergeMode string

// Merge modes. MergeDefault follows the user's merge.ff config.
const (
	MergeDefault MergeMode = ""
	// MergeNoFF always creates a merge commit.
	MergeNoFF MergeMode = "no-ff"
	// MergeFFOnly refuses unless the current branch can simply move forward.
	MergeFFOnly MergeMode = "ff-only"
)

// Merge merges ref into the current branch, keeping git's default merge message.
func (r *Repo) Merge(ctx context.Context, ref string, mode MergeMode) error {
	args := []string{"merge", "--no-edit"}
	switch mode {
	case MergeNoFF:
		args = append(args, "--no-ff")
	case MergeFFOnly:
		args = append(args, "--ff-only")
	}
	args = append(args, ref)
	return r.conflictAware(ctx, Cmd{Args: args, Env: []string{"GIT_EDITOR=true"}})
}

// CherryPick applies commits (oldest first) onto the current branch.
func (r *Repo) CherryPick(ctx context.Context, commits ...string) error {
	return r.conflictAware(ctx, Cmd{Args: append([]string{"cherry-pick"}, commits...), Env: []string{"GIT_EDITOR=true"}})
}

// RebaseOnto moves the current branch's commits after upstream onto onto: `git rebase
// --onto onto upstream`. Restacking uses it to drop a merged parent's old commits.
func (r *Repo) RebaseOnto(ctx context.Context, onto, upstream string) error {
	return r.conflictAware(ctx, Cmd{Args: []string{"rebase", "--onto", onto, upstream}, Env: []string{"GIT_EDITOR=true"}})
}

// RebaseStack moves top's commits after upstream onto onto, and every branch among them along
// with it: `git rebase --update-refs --onto onto upstream top`. It leaves top checked out.
func (r *Repo) RebaseStack(ctx context.Context, onto, upstream, top string) error {
	return r.conflictAware(ctx, Cmd{Args: []string{"rebase", "--update-refs", "--onto", onto, upstream, top}, Env: []string{"GIT_EDITOR=true"}})
}

// Rebase replays the current branch's commits onto ref.
func (r *Repo) Rebase(ctx context.Context, onto string) error {
	return r.conflictAware(ctx, Cmd{Args: []string{"rebase", onto}, Env: []string{"GIT_EDITOR=true"}})
}

// conflictAware runs a merge-like command, reporting ErrConflicts when it stopped with
// unmerged paths rather than failing outright.
func (r *Repo) conflictAware(ctx context.Context, c Cmd) error {
	_, err := r.Write(ctx, c)
	if err == nil {
		return nil
	}
	var gerr *Error
	if errors.As(err, &gerr) {
		if st, serr := r.Status(ctx); serr == nil {
			for _, f := range st.Files {
				if f.Conflicted {
					return ErrConflicts
				}
			}
		}
	}
	return err
}
