package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/risadams/gitrevenge/internal/fsutil"
	"github.com/risadams/gitrevenge/internal/git"
	"github.com/risadams/gitrevenge/internal/session"
)

// An undo entry records one operation that moved history, so it can be reversed.
//
// Undo is only offered while the repository is exactly where the operation left it: if
// anything moved since (another operation, or git in a terminal), undo refuses rather than
// guess. The reflog keeps every previous state either way, so nothing is ever lost.
type undoEntry struct {
	Label  string
	Kind   undoKind
	Before git.HeadState
	After  git.HeadState
	// Mode is the reset mode for kindReset.
	Mode git.ResetMode
	// Ref and Commit are the branch created or deleted and where it pointed, for
	// kindBranchCreate / kindBranchDelete.
	Ref    string
	Commit string
	// From is where Ref was before kindBranchMove, which left it at Commit.
	From string
	// Branches are the branches deleted together, for kindBranchesDelete.
	Branches []branchAt
}

type undoKind string

const (
	// kindCommit covers commit and amend: undo brings the changes back staged.
	kindCommit undoKind = "commit"
	// kindMove covers merge, cherry-pick, revert, rebase and pull: the branch moved.
	kindMove         undoKind = "move"
	kindReset        undoKind = "reset"
	kindCheckout     undoKind = "checkout"
	kindBranchCreate undoKind = "branch-create"
	kindBranchDelete undoKind = "branch-delete"
	// kindBranchesDelete is cleanup on pull: several branches deleted at once.
	kindBranchesDelete undoKind = "branches-delete"
	// kindBranchMove is pulling a branch that isn't checked out: only that branch moved.
	kindBranchMove undoKind = "branch-move"
)

// undoStacks holds undo and redo history per repository.
type undoStacks struct {
	mu   sync.Mutex
	undo map[string][]undoEntry
	redo map[string][]undoEntry
}

// maxUndo bounds history kept per repository.
const maxUndo = 50

func (u *undoStacks) push(path string, e undoEntry) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.undo == nil {
		u.undo, u.redo = map[string][]undoEntry{}, map[string][]undoEntry{}
	}
	u.undo[path] = append(u.undo[path], e)
	if n := len(u.undo[path]); n > maxUndo {
		u.undo[path] = u.undo[path][n-maxUndo:]
	}
	// A new operation makes anything previously undone unreachable by redo.
	delete(u.redo, path)
}

// forget drops a repository's undo and redo history, as when its tab closes: the entries
// describe a session with it that has ended, and offering them later would only mislead.
func (u *undoStacks) forget(path string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, m := range []map[string][]undoEntry{u.undo, u.redo} {
		for p := range m {
			if fsutil.SamePath(p, path) {
				delete(m, p)
			}
		}
	}
}

// record runs op and, if it succeeded and changed HEAD, remembers how to undo it.
func (a *App) record(r *session.Repo, label string, kind undoKind, mode git.ResetMode, op func() error) error {
	ctx := context.Background()
	before := r.Git.Head(ctx)
	if err := op(); err != nil {
		return err
	}
	after := r.Git.Head(ctx)
	if before == after {
		return nil
	}
	a.undo.push(r.Path, undoEntry{Label: label, Kind: kind, Before: before, After: after, Mode: mode})
	a.undoChanged(r.Path)
	return nil
}

// UndoChange is the payload of the "undo:changed" event.
type UndoChange struct {
	Path string `json:"path"`
}

// undoChanged tells the UI a repository's undo or redo history changed. The file watcher
// can report an operation's effects before record has pushed its entry, so the toolbar
// can't rely on that alone to refresh its labels.
func (a *App) undoChanged(path string) {
	a.emit("undo:changed", UndoChange{Path: path})
}

// recordBranch remembers creating or deleting a branch at commit.
func (a *App) recordBranch(r *session.Repo, label string, kind undoKind, branch, commit string) {
	h := r.Git.Head(context.Background())
	a.undo.push(r.Path, undoEntry{Label: label, Kind: kind, Ref: branch, Commit: commit, Before: h, After: h})
	a.undoChanged(r.Path)
}

// UndoInfo describes what Undo and Redo would do, for the toolbar.
type UndoInfo struct {
	Undo string `json:"undo"`
	Redo string `json:"redo"`
}

// UndoState returns the labels of the next undo and redo, or "" when unavailable.
func (a *App) UndoState(path string) (UndoInfo, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return UndoInfo{}, err
	}
	a.undo.mu.Lock()
	defer a.undo.mu.Unlock()
	var info UndoInfo
	if s := a.undo.undo[r.Path]; len(s) > 0 {
		info.Undo = s[len(s)-1].Label
	}
	if s := a.undo.redo[r.Path]; len(s) > 0 {
		info.Redo = s[len(s)-1].Label
	}
	return info, nil
}

var errMoved = userError("The repository changed since that operation (perhaps from another tool), so it can't be undone safely. Its earlier state is still in the reflog.")

// Undo reverses the most recent recorded operation.
func (a *App) Undo(path string) error { return a.step(path, true) }

// Redo reapplies the most recently undone operation.
func (a *App) Redo(path string) error { return a.step(path, false) }

func (a *App) step(path string, undo bool) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	u := &a.undo
	u.mu.Lock()
	from, to := u.undo, u.redo
	if !undo {
		from, to = u.redo, u.undo
	}
	stack := from[r.Path]
	if len(stack) == 0 {
		u.mu.Unlock()
		if undo {
			return userError("Nothing to undo.")
		}
		return userError("Nothing to redo.")
	}
	e := stack[len(stack)-1]
	u.mu.Unlock()

	if err := apply(context.Background(), r.Git, e, undo); err != nil {
		return err
	}
	u.mu.Lock()
	from[r.Path] = from[r.Path][:len(from[r.Path])-1]
	to[r.Path] = append(to[r.Path], e)
	u.mu.Unlock()
	a.undoChanged(r.Path)
	return nil
}

// apply undoes (or redoes) e, after checking the repository is where e left it.
func apply(ctx context.Context, g *git.Repo, e undoEntry, undo bool) error {
	expect, target := e.After, e.Before
	if !undo {
		expect, target = e.Before, e.After
	}
	switch e.Kind {
	case kindBranchesDelete:
		// Undo recreates every branch; redo deletes them again. A branch that came back some
		// other way in between is left alone.
		here, err := g.BranchNames(ctx)
		if err != nil {
			return friendlyUndo(err)
		}
		if !undo {
			head := g.Head(ctx).Branch
			var names []string
			for _, b := range e.Branches {
				if here[b.Name] && b.Name != head {
					names = append(names, b.Name)
				}
			}
			return friendlyUndo(g.DeleteBranches(ctx, names))
		}
		// The upstreams go first, so the branches arrive complete, in one step.
		tips := map[string]string{}
		for _, b := range e.Branches {
			if !here[b.Name] {
				if err := g.SetBranchTracking(ctx, b.Name, b.Tracking); err != nil {
					return friendlyUndo(err)
				}
				tips[b.Name] = b.Commit
			}
		}
		return friendlyUndo(g.CreateBranchesAt(ctx, tips))
	case kindBranchMove:
		from, to := e.Commit, e.From
		if !undo {
			from, to = e.From, e.Commit
		}
		if g.Head(ctx).Branch == e.Ref {
			return userError("Can't move " + e.Ref + " while it is checked out.")
		}
		err := g.MoveBranch(ctx, e.Ref, from, to)
		if errors.Is(err, git.ErrBranchMoved) {
			return errMoved
		}
		return friendlyUndo(err)
	case kindBranchCreate, kindBranchDelete:
		create := (e.Kind == kindBranchCreate) != undo // redo create, or undo delete
		if create {
			return friendlyUndo(g.CreateBranch(ctx, e.Ref, e.Commit, false))
		}
		if cur := g.Head(ctx); cur.Branch == e.Ref {
			return userError("Can't remove " + e.Ref + " while it is checked out.")
		}
		return friendlyUndo(g.DeleteBranch(ctx, e.Ref, true))
	}

	if g.Head(ctx) != expect {
		return errMoved
	}
	var err error
	switch e.Kind {
	case kindCheckout:
		ref := target.Branch
		if ref == "" {
			ref = target.Commit
		}
		err = g.Checkout(ctx, ref)
	case kindCommit:
		// Undo keeps the commit's changes staged; redo re-points the branch at the commit,
		// whose tree is exactly what is staged.
		err = g.Reset(ctx, target.Commit, git.ResetSoft)
	case kindReset:
		// Reverse a soft or mixed reset the same way, so uncommitted changes stay put; a
		// hard reset left a clean tree, so --keep is safe and refuses to clobber anything.
		if e.Mode == git.ResetHard {
			err = g.ResetKeep(ctx, target.Commit)
		} else {
			err = g.Reset(ctx, target.Commit, e.Mode)
		}
	default:
		err = g.ResetKeep(ctx, target.Commit)
	}
	return friendlyUndo(err)
}

func friendlyUndo(err error) error {
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		return userError(fmt.Sprintf("Couldn't complete that without touching your uncommitted changes:\n\n%s", gitMessage(gerr.Stderr)))
	}
	return err
}

// shortRef abbreviates full commit IDs for labels; names are kept as they are.
func shortRef(ref string) string {
	if len(ref) >= 40 {
		return ref[:7]
	}
	return ref
}
