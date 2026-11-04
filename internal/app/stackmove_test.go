package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// threeBranchStack makes a stack of s1, s2, s3 on main, two commits each, with main moved on since. It
// leaves main checked out.
func threeBranchStack(t *testing.T, local *gittest.Repo) []string {
	t.Helper()
	from := "main"
	for _, b := range []string{"s1", "s2", "s3"} {
		local.Git("checkout", "-q", "-b", b, from)
		local.CommitFile(b+"a.txt", b, b+" first")
		local.CommitFile(b+"b.txt", b, b+" second")
		from = b
	}
	local.Git("checkout", "-q", "main")
	return []string{"s3", "s2", "s1"}
}

func tipOf(fx *gittest.Repo, ref string) string { return strings.TrimSpace(fx.Git("rev-parse", ref)) }

func isAncestor(fx *gittest.Repo, a, b string) bool {
	_, err := fx.TryGit("merge-base", "--is-ancestor", a, b)
	return err == nil
}

// #475
func TestRebaseStack(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, _ := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	names := threeBranchStack(t, local)
	local.Git("branch", "base", "main")
	local.Git("checkout", "-q", "base")
	local.CommitFile("base.txt", "base", "base moves")
	local.Git("checkout", "-q", "main")
	was := map[string]string{}
	for _, n := range names {
		was[n] = tipOf(local, n)
	}

	if err := a.RebaseStack(tab.Path, names, "base"); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"base", "s1"}, {"s1", "s2"}, {"s2", "s3"}} {
		if !isAncestor(local, pair[0], pair[1]) {
			t.Errorf("%s should be under %s", pair[0], pair[1])
		}
	}
	for _, n := range names {
		if tipOf(local, n) == was[n] {
			t.Errorf("%s didn't move", n)
		}
	}
	moved := map[string]string{}
	for _, n := range names {
		moved[n] = tipOf(local, n)
	}
	info, _ := a.UndoState(tab.Path)
	if info.Undo != "Rebase stack onto base" {
		t.Errorf("undo: %q", info.Undo)
	}

	// One Undo puts every branch back, and the checkout.
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if tipOf(local, n) != was[n] {
			t.Errorf("%s not back", n)
		}
	}
	if got := strings.TrimSpace(local.Git("branch", "--show-current")); got != "main" {
		t.Errorf("checked out %q after undo", got)
	}

	// And Redo moves them all again.
	if err := a.Redo(tab.Path); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if tipOf(local, n) != moved[n] {
			t.Errorf("%s not redone", n)
		}
	}
	if got := strings.TrimSpace(local.Git("branch", "--show-current")); got != "s3" {
		t.Errorf("checked out %q after redo", got)
	}

	// Undo refuses once a branch has moved on, and moves nothing.
	local.Git("branch", "-f", "s2", "s1")
	if err := a.Undo(tab.Path); err == nil {
		t.Error("should refuse")
	}
	if tipOf(local, "s3") != moved["s3"] {
		t.Error("s3 moved anyway")
	}
}

func TestRebaseStackRefusesAFork(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, _ := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	threeBranchStack(t, local)
	local.Git("checkout", "-q", "-b", "fork", "s1")
	local.CommitFile("fork.txt", "fork", "fork")
	local.Git("checkout", "-q", "main")
	was := tipOf(local, "s3")
	err = a.RebaseStack(tab.Path, []string{"s3", "fork", "s2", "s1"}, "main")
	if err == nil || !strings.Contains(err.Error(), "forks") {
		t.Errorf("got %v", err)
	}
	if tipOf(local, "s3") != was {
		t.Error("s3 moved")
	}
}

func TestUpdateStack(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	names := threeBranchStack(t, local)
	s1, s2 := tipOf(local, "s1"), tipOf(local, "s2")

	// Someone merges the bottom branch on the host, as a squash.
	host := gittest.Clone(t, remote)
	local.Git("push", "-q", "origin", "s1")
	host.Git("fetch", "-q", "origin")
	host.Git("merge", "-q", "--squash", "origin/s1")
	host.Commit("squash s1")
	host.Git("push", "-q", "origin", "main")

	merged, err := a.UpdateStack(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(merged, []string{"s1"}) {
		t.Errorf("merged: %v", merged)
	}
	if tipOf(local, "s1") != s1 {
		t.Error("the merged branch should stay where it is")
	}
	if tipOf(local, "s2") == s2 {
		t.Error("s2 should have moved")
	}
	for _, pair := range [][2]string{{"origin/main", "s2"}, {"s2", "s3"}} {
		if !isAncestor(local, pair[0], pair[1]) {
			t.Errorf("%s should be under %s", pair[0], pair[1])
		}
	}
	// Only their own commits came along: s1's were dropped, not replayed.
	if got := strings.TrimSpace(local.Git("rev-list", "--count", "origin/main..s3")); got != "4" {
		t.Errorf("s3 is %s commits above main", got)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if tipOf(local, "s2") != s2 {
		t.Error("undo should put s2 back")
	}
}

func TestUpdateStackAllMerged(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, _ := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	names := threeBranchStack(t, local)
	s3 := tipOf(local, "s3")
	local.Git("merge", "-q", "--squash", "s3")
	local.Commit("squash all")
	local.Git("push", "-q", "origin", "main")
	merged, err := a.UpdateStack(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(merged, names) || tipOf(local, "s3") != s3 {
		t.Errorf("merged: %v", merged)
	}
}

func TestPushStack(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	names := threeBranchStack(t, local)

	// New branches go up, and track what they went to.
	if err := a.PushStack(tab.Path, names); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if tipOf(remote, n) != tipOf(local, n) {
			t.Errorf("%s not pushed", n)
		}
		if got := strings.TrimSpace(local.Git("rev-parse", "--abbrev-ref", n+"@{upstream}")); got != "origin/"+n {
			t.Errorf("%s tracks %q", n, got)
		}
	}

	// After a rebase, nothing is really lost: the same changes are still there.
	local.Git("checkout", "-q", "main")
	local.CommitFile("moved.txt", "moved", "main moves")
	if err := a.RebaseStack(tab.Path, names, "main"); err != nil {
		t.Fatal(err)
	}
	plan, err := a.StackPushPlan(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Push, names) || len(plan.Losses) != 0 {
		t.Errorf("after a rebase: %+v", plan)
	}
	if err := a.PushStack(tab.Path, plan.Push); err != nil {
		t.Fatal(err)
	}
	if tipOf(remote, "s3") != tipOf(local, "s3") {
		t.Error("s3 not force pushed")
	}

	// With the top checked out, its lower branches being inside it doesn't make them merged.
	refsAfterReload(t, a, tab.Path, func(rs []git.Ref) bool {
		for _, r := range rs {
			if r.Short == "s3" {
				return r.Head
			}
		}
		return false
	})
	plan, err = a.StackPushPlan(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Push, names) {
		t.Errorf("with the top checked out: %+v", plan)
	}

	// Someone else's commit on a remote branch is what's at stake, and the plan names it.
	host := gittest.Clone(t, remote)
	host.Git("fetch", "-q", "origin")
	host.Git("checkout", "-q", "-b", "s3", "origin/s3")
	host.CommitFile("theirs.txt", "theirs", "their work")
	host.Git("push", "-q", "origin", "s3")
	local.Git("fetch", "-q", "origin")
	plan, err = a.StackPushPlan(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Losses) != 1 || plan.Losses[0].Branch != "s3" || plan.Losses[0].Commits[0].Subject != "their work" {
		t.Errorf("with their commit: %+v", plan)
	}

	// The lease stops a push when the remote moved since the last fetch.
	host.CommitFile("more.txt", "more", "more of their work")
	host.Git("push", "-q", "origin", "s3")
	onRemote := tipOf(remote, "s3")
	if err := a.PushStack(tab.Path, names); err == nil {
		t.Error("the lease should refuse")
	}
	if tipOf(remote, "s3") != onRemote {
		t.Error("s3 was overwritten")
	}
}
