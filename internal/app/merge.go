package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Integrate merges, cherry-picks, rebases, resets or reverts on the current branch:
//
//	merge:       ref, mode ("" / "no-ff" / "ff-only" / "squash")
//	cherry-pick: commit IDs, oldest first
//	rebase:      onto, "autosquash" to fold fixup! and squash! commits into the ones they name
//	reset:       target, mode ("soft" / "mixed" / "hard")
//	revert:      commit, mainline parent for merges ("" for ordinary commits)
//
// When the operation stops with conflicts, the in-progress banner takes over.
func (a *App) Integrate(path, action string, args []string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	switch action {
	case "merge":
		err = a.record(r, "Merge "+shortRef(arg(0)), kindMove, "", func() error { return r.Git.Merge(ctx, arg(0), git.MergeMode(arg(1))) })
	case "cherry-pick":
		if len(args) == 0 {
			return userError("Choose at least one commit to cherry-pick.")
		}
		err = a.record(r, "Cherry-pick "+shortRef(args[0]), kindMove, "", func() error { return r.Git.CherryPick(ctx, args...) })
	case "rebase":
		err = a.record(r, "Rebase onto "+shortRef(arg(0)), kindMove, "", func() error {
			if arg(1) == "autosquash" {
				return r.Git.RebaseAutosquash(ctx, arg(0))
			}
			return r.Git.Rebase(ctx, arg(0))
		})
	case "reset":
		err = a.record(r, "Reset to "+shortRef(arg(0))+" ("+arg(1)+")", kindReset, git.ResetMode(arg(1)), func() error {
			return r.Git.Reset(ctx, arg(0), git.ResetMode(arg(1)))
		})
	case "revert":
		mainline, _ := strconv.Atoi(arg(1))
		err = a.record(r, "Revert "+shortRef(arg(0)), kindMove, "", func() error { return r.Git.Revert(ctx, arg(0), mainline) })
	default:
		return userError("Unknown action " + action + ".")
	}
	if errors.Is(err, git.ErrConflicts) {
		return userError("There are conflicts to resolve. Fix the conflicted files, stage them, then use Continue in the banner, or Abort to undo.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		return gitMessage(gerr.Stderr)
	}
	return writeErr(err)
}
