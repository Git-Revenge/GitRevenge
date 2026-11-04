package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// BranchAction runs a branch or tag operation from the UI. Arguments by action:
//
//	create:   name, start ("" = HEAD), checkout ("true"/"")
//	rename:   name, newName
//	delete:   name, force ("true"/"")
//	checkout: ref (branch, remote branch, tag or commit)
//	upstream: name, upstream ("" to unset)
//	tag:      name, target ("" = HEAD), message ("" = lightweight)
//	untag:    name
func (a *App) BranchAction(path, action string, args []string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	ctx := context.Background()
	g := r.Git
	switch action {
	case "create":
		if arg(2) == "true" {
			// Creating and switching moves HEAD: undo switches back and deletes the branch.
			err = a.record(r, "Create and check out "+arg(0), kindCheckout, "", func() error {
				return g.CreateBranch(ctx, arg(0), arg(1), true)
			})
		} else if err = g.CreateBranch(ctx, arg(0), arg(1), false); err == nil {
			commit, _ := g.ReadString(ctx, "rev-parse", "refs/heads/"+arg(0))
			a.recordBranch(r, "Create branch "+arg(0), kindBranchCreate, arg(0), commit)
		}
	case "rename":
		err = g.RenameBranch(ctx, arg(0), arg(1))
	case "delete":
		commit, _ := g.ReadString(ctx, "rev-parse", "refs/heads/"+arg(0))
		if err = g.DeleteBranch(ctx, arg(0), arg(1) == "true"); err == nil && commit != "" {
			a.recordBranch(r, "Delete branch "+arg(0), kindBranchDelete, arg(0), commit)
		}
	case "checkout":
		err = a.record(r, "Check out "+shortRef(arg(0)), kindCheckout, "", func() error { return g.Checkout(ctx, arg(0)) })
	case "upstream":
		err = g.SetUpstream(ctx, arg(0), arg(1))
	case "tag":
		err = g.CreateTag(ctx, arg(0), arg(1), arg(2))
	case "untag":
		err = g.DeleteTag(ctx, arg(0))
	default:
		return userError("Unknown action " + action + ".")
	}
	return branchErr(err)
}

func branchErr(err error) error {
	switch {
	case errors.Is(err, git.ErrInvalidName):
		return userError("That name isn't allowed by git. Avoid spaces, '..', '~', '^', ':', '?', '*', '[' and a leading '-'.")
	case errors.Is(err, git.ErrNotMerged):
		return userError("This branch has commits that aren't merged anywhere. Delete it anyway to discard them.")
	case errors.Is(err, git.ErrCurrentBranch):
		return userError("This branch is checked out, here or in another worktree. Switch to another branch there first.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		// e.g. "a branch named 'x' already exists", or local changes blocking a checkout.
		return gitMessage(gerr.Stderr)
	}
	return writeErr(err)
}
