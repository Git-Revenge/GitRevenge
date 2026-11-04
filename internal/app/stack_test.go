package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// #474
func TestDeleteStack(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, _ := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"s3", "s2", "s1"}
	for i, b := range []string{"s1", "s2", "s3"} {
		from := "main"
		if i > 0 {
			from = "s" + string(rune('0'+i))
		}
		local.Git("checkout", "-q", "-b", b, from)
		local.CommitFile(b+".txt", b, b)
		if i > 0 {
			if err := a.SetStackParent(tab.Path, b, from, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	local.Git("checkout", "-q", "main")
	reloaded := func(ok func([]git.Ref) bool) { refsAfterReload(t, a, tab.Path, ok) }
	reloaded(func(rs []git.Ref) bool {
		for _, r := range rs {
			if r.Short == "s3" {
				return r.StackParent == "s2"
			}
		}
		return false
	})

	plan, err := a.StackDeletePlan(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Delete, names) || len(plan.Keep) != 0 || len(plan.Unmerged) != 3 {
		t.Fatalf("not merged: %+v", plan)
	}
	if plan.Unmerged[0].Name != "s3" || plan.Unmerged[0].Commits != 3 {
		t.Errorf("s3 first, with 3 commits: %+v", plan.Unmerged)
	}

	// Nothing goes without leave to lose work.
	if _, err := a.DeleteStack(tab.Path, "s1", names, false); err == nil {
		t.Error("should refuse unmerged work")
	}
	if got := localBranches(local); !slices.Contains(got, "s3") {
		t.Fatalf("deleted anyway: %v", got)
	}
	tip := strings.TrimSpace(local.Git("rev-parse", "s3"))
	gone, err := a.DeleteStack(tab.Path, "s1", names, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(gone) != 3 {
		t.Fatalf("deleted %v", gone)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(local.Git("rev-parse", "s3")); got != tip {
		t.Errorf("s3 came back at %s, was at %s", got, tip)
	}

	// Once merged, it needs no leave.
	local.Git("merge", "-q", "--squash", "s3")
	local.Commit("squash the stack")
	plan, err = a.StackDeletePlan(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Unmerged) != 0 {
		t.Errorf("merged: %+v", plan)
	}
	if gone, err = a.DeleteStack(tab.Path, "s1", names, false); err != nil || len(gone) != 3 {
		t.Fatalf("got %v, %v", gone, err)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}

	// The checked-out branch stays, and so does what it builds on.
	local.Git("checkout", "-q", "s2")
	reloaded(func(rs []git.Ref) bool {
		for _, r := range rs {
			if r.Short == "s2" {
				return r.Head
			}
		}
		return false
	})
	plan, err = a.StackDeletePlan(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Delete, []string{"s3"}) || !slices.Equal(plan.Keep, []string{"s2", "s1"}) {
		t.Errorf("with s2 checked out: %+v", plan)
	}
}

func TestUnstack(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, _ := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetStackParent(tab.Path, "byhand", "main", false); err != nil {
		t.Fatal(err)
	}
	parents := func() map[string]string { return a.repoSettings(tab.Path).StackParents }

	names := []string{"byhand", "worked-out"}
	before, err := a.Unstack(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := parents()["worked-out"]; !ok || got != "" {
		t.Errorf("worked-out: %q, %v", got, ok)
	}
	if parents()["byhand"] != "" {
		t.Errorf("byhand: %q", parents()["byhand"])
	}
	if len(before) != 1 || before["byhand"] != "main" {
		t.Errorf("before: %v", before)
	}

	if err := a.RestoreStack(tab.Path, names, before); err != nil {
		t.Fatal(err)
	}
	if parents()["byhand"] != "main" {
		t.Errorf("byhand back at %q", parents()["byhand"])
	}
	if _, ok := parents()["worked-out"]; ok {
		t.Error("worked-out should be worked out from history again")
	}
}

// Deciding what's safe to delete can't wait for the graph to reload. Before, a branch that had
// just come back from Undo wasn't there yet, so deleting it quietly did nothing, and a branch
// just checked out wasn't yet known to be.
func TestDeletableBranchesReadsGitNotTheGraph(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, _ := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	local.Git("branch", "other")
	refsAfterReload(t, a, tab.Path, func(rs []git.Ref) bool {
		return slices.ContainsFunc(rs, func(r git.Ref) bool { return r.Short == "other" })
	})

	// Straight after making it, well inside the watcher's debounce.
	local.Git("branch", "fresh")
	safe, err := a.DeletableBranches(tab.Path, []string{"fresh"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(safe, []string{"fresh"}) {
		t.Errorf("a branch just made: %v", safe)
	}

	local.Git("checkout", "-q", "other")
	safe, err = a.DeletableBranches(tab.Path, []string{"other", "main"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(safe, "other") {
		t.Errorf("the branch just checked out: %v", safe)
	}
}
