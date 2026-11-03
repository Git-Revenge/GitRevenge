package app

import (
	"context"
	"strconv"
	"sync"

	"github.com/risadams/gitrevenge/internal/branchkind"
	"github.com/risadams/gitrevenge/internal/fsutil"
	"github.com/risadams/gitrevenge/internal/git"
	"github.com/risadams/gitrevenge/internal/session"
)

// Cleanup on pull deletes local branches whose upstream was deleted and that have nothing
// unpushed. "Nothing unpushed" means the branch is exactly where its upstream last was, so
// that position has to be remembered before a fetch prunes the upstream away. Fetch prunes
// too, so every fetch and pull remembers it, for this session.

// upstreamMemory remembers, per repository, where each local branch's upstream last
// pointed.
type upstreamMemory struct {
	mu   sync.Mutex
	tips map[string]map[string]upstreamTip // repo path → local branch → upstream
}

type upstreamTip struct {
	upstream string // short name, e.g. origin/feature
	commit   string
}

func (u *upstreamMemory) repo(path string) map[string]upstreamTip {
	for p, m := range u.tips {
		if fsutil.SamePath(p, path) {
			return m
		}
	}
	if u.tips == nil {
		u.tips = map[string]map[string]upstreamTip{}
	}
	m := map[string]upstreamTip{}
	u.tips[path] = m
	return m
}

// remember records where the upstreams of path's local branches are now. Branches whose
// upstream is already gone keep what was remembered before.
func (u *upstreamMemory) remember(ctx context.Context, path string, g *git.Repo) {
	refs, err := g.Refs(ctx)
	if err != nil {
		return
	}
	remote := map[string]string{}
	for _, r := range refs {
		if r.Kind == git.RemoteBranch {
			remote[r.Short] = r.Commit
		}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	m := u.repo(path)
	for _, r := range refs {
		if r.Kind != git.LocalBranch || r.Upstream == "" {
			continue
		}
		if c, ok := remote[r.Upstream]; ok {
			m[r.Short] = upstreamTip{upstream: r.Upstream, commit: c}
		}
	}
}

// remembered returns where branch's upstream was last seen.
func (u *upstreamMemory) remembered(path, branch string) (upstreamTip, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	t, ok := u.repo(path)[branch]
	return t, ok
}

// forget drops what's remembered for branches that no longer exist.
func (u *upstreamMemory) forget(path string, branches []string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	m := u.repo(path)
	for _, b := range branches {
		delete(m, b)
	}
}

// protectedKinds are never cleaned up, even with their upstream gone.
var protectedKinds = map[branchkind.Kind]bool{
	branchkind.Trunk:       true,
	branchkind.Release:     true,
	branchkind.Environment: true,
	branchkind.Docs:        true,
}

// cleanupCandidates returns the local branches cleanup would delete: upstream gone, tip
// where the upstream last was, not checked out here or in a worktree, and not a protected
// kind.
func (a *App) cleanupCandidates(ctx context.Context, r *session.Repo) ([]git.Ref, error) {
	refs, err := r.Git.Refs(ctx)
	if err != nil {
		return nil, err
	}
	busy := map[string]bool{}
	if wts, err := r.Git.Worktrees(ctx); err == nil {
		for _, w := range wts {
			busy[w.Branch] = true
		}
	}
	rules := a.branchRules(r.Path)
	// A branch something is stacked on stays: its children still build on it.
	parentOf := map[string]bool{}
	for _, p := range session.Stacks(ctx, r.Git, refs, rules, a.repoSettings(r.Path).StackParents) {
		parentOf[p] = true
	}
	var out []git.Ref
	for _, ref := range refs {
		if ref.Kind != git.LocalBranch || !ref.Gone || ref.Head || busy[ref.Short] || protectedKinds[rules.Kind(ref.Short)] || parentOf[ref.Short] {
			continue
		}
		last, ok := a.upstreams.remembered(r.Path, ref.Short)
		if !ok || last.upstream != ref.Upstream || last.commit != ref.Commit {
			continue
		}
		out = append(out, ref)
	}
	return out, nil
}

// BranchesCleaned is the payload of the "branches:cleaned" event.
type BranchesCleaned struct {
	Path     string   `json:"path"`
	Branches []string `json:"branches"`
	// UndoLabel is the undo entry that restores them, so the UI can offer Undo only while
	// it's still the next one.
	UndoLabel string `json:"undoLabel"`
	// Merged says all of them were already merged.
	Merged bool `json:"merged"`
}

// cleanupAfterPull deletes the branches cleanupCandidates finds, when the setting is on.
func (a *App) cleanupAfterPull(ctx context.Context, r *session.Repo) {
	if !a.repoSettings(r.Path).CleanupOnPull {
		return
	}
	cands, err := a.cleanupCandidates(ctx, r)
	if err != nil || len(cands) == 0 {
		return
	}
	a.deleteMerged(ctx, r, cands)
}

// deleteMerged deletes branches already known to be merged, as one step to undo.
func (a *App) deleteMerged(ctx context.Context, r *session.Repo, refs []git.Ref) []string {
	return a.deleteTogether(ctx, r, refs, "Clean up "+plural(len(refs), "merged branch", "merged branches"), true)
}

// deleteTogether deletes branches, records one undo entry under label that restores them all,
// and tells the UI. Failures to delete one branch skip it. It returns the names of those it
// deleted.
func (a *App) deleteTogether(ctx context.Context, r *session.Repo, refs []git.Ref, label string, merged bool) []string {
	tracking := r.Git.BranchTrackings(ctx)
	asked := make([]string, len(refs))
	for i, ref := range refs {
		asked[i] = ref.Short
	}
	// One that can't be deleted stays, and shows as still here just below.
	_ = r.Git.DeleteBranches(ctx, asked)
	left, _ := r.Git.BranchNames(ctx)
	var gone []branchAt
	for _, ref := range refs {
		if !left[ref.Short] {
			gone = append(gone, branchAt{Name: ref.Short, Commit: ref.Commit, Tracking: tracking[ref.Short]})
		}
	}
	if len(gone) == 0 {
		return nil
	}
	names := make([]string, len(gone))
	for i, b := range gone {
		names[i] = b.Name
	}
	a.upstreams.forget(r.Path, names)
	h := r.Git.Head(ctx)
	a.undo.push(r.Path, undoEntry{Label: label, Kind: kindBranchesDelete, Branches: gone, Before: h, After: h})
	a.undoChanged(r.Path)
	a.emit("branches:cleaned", BranchesCleaned{Path: r.Path, Branches: names, UndoLabel: label, Merged: merged})
	return names
}

// branchAt is a branch, the commit it pointed at and its upstream, for undoing a delete.
type branchAt struct {
	Name     string
	Commit   string
	Tracking git.BranchTracking
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
