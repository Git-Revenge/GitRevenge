package app

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/risadams/gitrevenge/internal/branchkind"
	"github.com/risadams/gitrevenge/internal/git"
	"github.com/risadams/gitrevenge/internal/session"
)

// UnmergedBranch is a branch whose work isn't on any target, and so only it holds.
type UnmergedBranch struct {
	Name string `json:"name"`
	// Commits is how many of its commits aren't merged, or 0 if that isn't known.
	Commits int `json:"commits"`
}

// StackPlan is what deleting a stack would do.
type StackPlan struct {
	Delete []string `json:"delete"`
	// Keep is what stays: see deletableBranches.
	Keep []string `json:"keep"`
	// Unmerged is those in Delete whose work would be lost to all but the reflog and Undo.
	Unmerged []UnmergedBranch `json:"unmerged"`
}

// StackDeletePlan says what deleting these branches of a stack would do.
func (a *App) StackDeletePlan(path string, names []string) (StackPlan, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return StackPlan{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mergedBranchesTimeout)
	defer cancel()
	return a.stackDeletePlan(ctx, r, names)
}

// stackTargets lists what to compare a stack's branches with. The checked-out branch is one,
// but if it's in the stack, its own lower branches being inside it doesn't make them merged.
func (a *App) stackTargets(ctx context.Context, r *session.Repo, names []string) ([]string, error) {
	targets, err := a.mergeTargetsFor(ctx, r)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(targets, func(t string) bool { return slices.Contains(names, t) }), nil
}

// stackDeletePlan counts a branch as unmerged when the time runs out before it's been checked,
// which only means being asked. Only the merge checks are on the clock, so running out can't
// fail the plan.
func (a *App) stackDeletePlan(ctx context.Context, r *session.Repo, names []string) (StackPlan, error) {
	targets, err := a.stackTargets(context.Background(), r, names)
	if err != nil {
		return StackPlan{}, err
	}
	states := r.Git.BranchStates(ctx, names, targets)
	plan := StackPlan{Delete: a.deletableBranches(context.Background(), r, names), Keep: []string{}, Unmerged: []UnmergedBranch{}}
	for _, name := range names {
		if !slices.Contains(plan.Delete, name) {
			plan.Keep = append(plan.Keep, name)
		}
	}
	for _, name := range plan.Delete {
		if states[name].How == "" {
			plan.Unmerged = append(plan.Unmerged, UnmergedBranch{Name: name, Commits: states[name].Unmerged})
		}
	}
	return plan, nil
}

// DeleteStack deletes what StackDeletePlan allows of these branches, as one step to undo. Unless
// allowed to lose work, it looks again first and stops if anything isn't merged any more. It
// returns the names of those it deleted.
func (a *App) DeleteStack(path, root string, names []string, allowUnmerged bool) ([]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	safe := a.deletableBranches(context.Background(), r, names)
	if !allowUnmerged {
		ctx, cancel := context.WithTimeout(context.Background(), mergedBranchesTimeout)
		defer cancel()
		plan, err := a.stackDeletePlan(ctx, r, names)
		if err != nil {
			return nil, err
		}
		if len(plan.Unmerged) > 0 {
			return nil, userError(plan.Unmerged[0].Name + " doesn't read as merged any more, so nothing was deleted.")
		}
		safe = plan.Delete
	}
	ctx := context.Background()
	refs, err := a.currentRefs(ctx, r, safe)
	if err != nil {
		return nil, err
	}
	gone := a.deleteTogether(ctx, r, refs, "Delete stack "+root, !allowUnmerged)
	if gone == nil {
		return []string{}, nil
	}
	r.ReloadGraph()
	return gone, nil
}

// currentRefs reads these local branches as they are now, not as last loaded, since undoing
// a delete has to put them back exactly where they were.
func (a *App) currentRefs(ctx context.Context, r *session.Repo, names []string) ([]git.Ref, error) {
	current, err := r.Git.Refs(ctx)
	if err != nil {
		return nil, err
	}
	var refs []git.Ref
	for _, ref := range current {
		if ref.Kind == git.LocalBranch && slices.Contains(names, ref.Short) {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

// Unstack stops treating these branches as stacked on anything. It returns what was set for
// them by hand before, for RestoreStack.
func (a *App) Unstack(path string, names []string) (map[string]string, error) {
	before := map[string]string{}
	err := a.updateStackParents(path, func(parents map[string]string) {
		for _, name := range names {
			if parent, ok := parents[name]; ok {
				before[name] = parent
			}
			parents[name] = ""
		}
	})
	if err != nil {
		return nil, err
	}
	return before, nil
}

// RestoreStack undoes Unstack: these branches go back to what was set by hand, or to being
// worked out from history.
func (a *App) RestoreStack(path string, names []string, before map[string]string) error {
	return a.updateStackParents(path, func(parents map[string]string) {
		for _, name := range names {
			if parent, ok := before[name]; ok {
				parents[name] = parent
			} else {
				delete(parents, name)
			}
		}
	})
}

// branchMove is a branch that moved, and from and to where.
type branchMove struct {
	Name string
	From string
	To   string
}

// applyStackMove puts every branch of a moved stack back, or forward again, along with the
// checkout. It changes nothing unless they're all still where the move left them.
func applyStackMove(ctx context.Context, g *git.Repo, e undoEntry, expect, target git.HeadState, undo bool) error {
	if g.Head(ctx) != expect {
		return errMoved
	}
	refs, err := g.Refs(ctx)
	if err != nil {
		return friendlyUndo(err)
	}
	now := map[string]string{}
	for _, ref := range refs {
		if ref.Kind == git.LocalBranch {
			now[ref.Short] = ref.Commit
		}
	}
	type step struct{ from, to string }
	steps := map[string]step{}
	for _, m := range e.Moves {
		s := step{m.To, m.From}
		if !undo {
			s = step{m.From, m.To}
		}
		if now[m.Name] != s.from {
			return errMoved
		}
		steps[m.Name] = s
	}
	// The checked-out branch moves with its working tree, the rest as plain refs.
	for name, s := range steps {
		if name == expect.Branch {
			if err := g.ResetKeep(ctx, s.to); err != nil {
				return friendlyUndo(err)
			}
		} else if err := g.MoveBranch(ctx, name, s.from, s.to); err != nil {
			return friendlyUndo(err)
		}
	}
	if target.Branch != expect.Branch {
		ref := target.Branch
		if ref == "" {
			ref = target.Commit
		}
		return friendlyUndo(g.Checkout(ctx, ref))
	}
	return nil
}

// checkLine refuses a stack that forks: --update-refs moves one line of commits, and rebasing
// each fork in turn would duplicate what they share. names runs from the top down.
func checkLine(ctx context.Context, g *git.Repo, names []string) error {
	if len(names) == 0 {
		return userError("There's nothing in this stack.")
	}
	for i := 1; i < len(names); i++ {
		if _, err := g.ReadString(ctx, "merge-base", "--is-ancestor", "refs/heads/"+names[i], "refs/heads/"+names[i-1]); err != nil {
			return userError("This stack forks at " + names[i] + ", so it can't move as one. Restack each branch on its own.")
		}
	}
	return nil
}

// moveStack rebases top's commits after upstream onto onto, recording one step to undo.
func (a *App) moveStack(ctx context.Context, r *session.Repo, names []string, onto, upstream string) error {
	before := r.Git.Head(ctx)
	tips, err := a.currentRefs(ctx, r, names)
	if err != nil {
		return err
	}
	r.Quietly(func() { err = r.Git.RebaseStack(ctx, onto, upstream, names[0]) })
	if errors.Is(err, git.ErrConflicts) {
		return userError("Rebasing the stack hit conflicts. Fix the conflicted files, stage them, then use Continue in the banner. Abort there puts everything back. Undo won't be offered for it afterwards.")
	}
	if err != nil {
		var gerr *git.Error
		if errors.As(err, &gerr) && gerr.Stderr != "" {
			return gitMessage(gerr.Stderr)
		}
		return writeErr(err)
	}
	after, err := a.currentRefs(ctx, r, names)
	if err != nil {
		return err
	}
	var moves []branchMove
	for _, ref := range tips {
		for _, now := range after {
			if now.Short == ref.Short && now.Commit != ref.Commit {
				moves = append(moves, branchMove{Name: ref.Short, From: ref.Commit, To: now.Commit})
			}
		}
	}
	if len(moves) == 0 {
		return nil
	}
	h := r.Git.Head(ctx)
	a.undo.push(r.Path, undoEntry{Label: "Rebase stack onto " + shortRef(onto), Kind: kindStackMove, Moves: moves, Before: before, After: h})
	a.undoChanged(r.Path)
	return nil
}

// RebaseStack moves a whole stack onto onto, keeping every branch in place on it. names runs
// from the top down. Conflicts enter the normal rebase flow.
func (a *App) RebaseStack(path string, names []string, onto string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := checkLine(ctx, r.Git, names); err != nil {
		return err
	}
	return a.moveStack(ctx, r, names, onto, onto)
}

// trunkOf finds the local Trunk branch, and its upstream if it has one that exists.
func trunkOf(refs []git.Ref, rules branchkind.Rules) (local, upstream string) {
	for _, p := range rules.Patterns(branchkind.Trunk) {
		for _, ref := range refs {
			if ref.Kind == git.LocalBranch && local == "" && branchkind.Match(p, ref.Short) {
				local = ref.Short
				if ref.Upstream != "" && !ref.Gone {
					upstream = ref.Upstream
				}
			}
		}
	}
	return local, upstream
}

// UpdateStack brings a stack up to date after its lower branches merge: it fetches Trunk, moves
// the branches not yet merged onto it, and returns those that are merged, which are left where
// they are for deleting. names runs from the top down. Trunk's upstream is what they move onto
// if it has one, since that holds the merge whether or not Trunk has been pulled.
func (a *App) UpdateStack(path string, names []string) ([]string, error) {
	merged := []string{}
	err := a.runNetwork(path, "pull", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		if err := checkLine(ctx, r.Git, names); err != nil {
			return err
		}
		refs, err := r.Git.Refs(ctx)
		if err != nil {
			return err
		}
		trunk, upstream := trunkOf(refs, a.branchRules(r.Path))
		if trunk == "" {
			return userError("There's no Trunk branch to update onto.")
		}
		onto, targets := trunk, []string{trunk}
		if upstream != "" {
			remote, _, _ := strings.Cut(upstream, "/")
			a.upstreams.remember(ctx, r.Path, r.Git)
			if err := r.Git.Fetch(ctx, remote, o); err != nil {
				return err
			}
			onto, targets = upstream, []string{upstream, trunk}
		}
		states := r.Git.BranchStates(ctx, names, targets)
		// Everything from the first merged branch down is on Trunk already.
		k := slices.IndexFunc(names, func(n string) bool { return states[n].How != "" })
		if k < 0 {
			return a.moveStack(ctx, r, names, onto, onto)
		}
		merged = names[k:]
		if k == 0 {
			return nil
		}
		return a.moveStack(ctx, r, names[:k], onto, names[k])
	})
	if err != nil {
		return nil, err
	}
	return merged, nil
}

// BranchLoss is what force pushing a branch would really lose from the remote.
type BranchLoss struct {
	Branch string `json:"branch"`
	SharedCommits
}

// StackPush is what Push stack would do.
type StackPush struct {
	// Push is the branches it would send.
	Push []string `json:"push"`
	// Merged is those it leaves alone: there's nothing to send, and it would bring a remote
	// branch back that was deleted on merging.
	Merged []string     `json:"merged"`
	Losses []BranchLoss `json:"losses"`
}

// StackPushPlan says what pushing these branches of a stack would do.
func (a *App) StackPushPlan(path string, names []string) (StackPush, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return StackPush{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mergedBranchesTimeout)
	defer cancel()
	targets, err := a.stackTargets(context.Background(), r, names)
	if err != nil {
		return StackPush{}, err
	}
	states := r.Git.BranchStates(ctx, names, targets)
	plan := StackPush{Push: []string{}, Merged: []string{}, Losses: []BranchLoss{}}
	bg := context.Background()
	for _, name := range names {
		if states[name].How != "" {
			plan.Merged = append(plan.Merged, name)
			continue
		}
		plan.Push = append(plan.Push, name)
		upstream := r.Git.UpstreamOf(bg, name)
		if upstream == "" {
			continue
		}
		commits, total, err := r.Git.CommitsOnlyOn(bg, "refs/heads/"+name, upstream, maxListedCommits)
		if err == nil && total > 0 {
			plan.Losses = append(plan.Losses, BranchLoss{Branch: name, SharedCommits: SharedCommits{Upstream: upstream, Commits: commits, Total: total}})
		}
	}
	return plan, nil
}

// PushStack force pushes these branches with a lease, each to where it tracks, or to the only
// remote or "origin" if it tracks nothing. A remote takes all of its branches or none.
func (a *App) PushStack(path string, names []string) error {
	return a.runNetwork(path, "push", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		remotes, err := r.Git.Remotes(ctx)
		if err != nil {
			return err
		}
		fallback := "origin"
		if len(remotes) == 1 {
			fallback = remotes[0].Name
		}
		if len(remotes) == 0 {
			return userError("This repository has no remotes. Add one first.")
		}
		tracking := r.Git.BranchTrackings(ctx)
		byRemote := map[string][]string{}
		for _, name := range names {
			remote, merge := fallback, "refs/heads/"+name
			if t := tracking[name]; t.Remote != "" && t.Remote != "." && t.Merge != "" {
				remote, merge = t.Remote, t.Merge
			}
			byRemote[remote] = append(byRemote[remote], name+":"+merge)
		}
		for remote, refspecs := range byRemote {
			if err := r.Git.PushBranches(ctx, remote, refspecs, o); err != nil {
				return err
			}
		}
		return nil
	})
}
