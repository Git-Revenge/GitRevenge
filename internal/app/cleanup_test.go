package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func localBranches(fx *gittest.Repo) []string {
	return strings.Fields(fx.Git("for-each-ref", "--format=%(refname:short)", "refs/heads"))
}

func TestCleanupOnPull(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := a.RepoPreferences(tab.Path)
	p.CleanupOnPull = true
	if err := a.SetRepoPreferences(tab.Path, p); err != nil {
		t.Fatal(err)
	}

	// Three pushed branches, then each one's remote branch is deleted (as a squash merge
	// on the host would). feature/b gets a commit that was never pushed; release/x is a
	// protected kind; feature/c is checked out in a worktree.
	for _, b := range []string{"feature/a", "feature/b", "release/x", "feature/c"} {
		local.Git("checkout", "-q", "-b", b, "main")
		local.CommitFile(strings.ReplaceAll(b, "/", "-")+".txt", b, b)
		local.Git("push", "-q", "-u", "origin", b)
	}
	local.CommitFile("unpushed.txt", "x", "unpushed")
	local.Git("checkout", "-q", "feature/b")
	local.CommitFile("b2.txt", "x", "not pushed")
	local.Git("checkout", "-q", "main")
	local.Git("worktree", "add", "-q", t.TempDir()+"/wt", "feature/c")
	remote.Git("branch", "-D", "feature/a", "feature/b", "release/x", "feature/c")

	// A fetch first prunes the upstreams; the pull after it still knows where they were.
	if err := a.Fetch(tab.Path, "origin"); err != nil {
		t.Fatal(err)
	}
	if err := a.Pull(tab.Path, ""); err != nil {
		t.Fatal(err)
	}
	got := localBranches(local)
	if slices.Contains(got, "feature/a") {
		t.Fatalf("feature/a should be cleaned up: %v", got)
	}
	for _, keep := range []string{"main", "feature/b", "release/x", "feature/c"} {
		if !slices.Contains(got, keep) {
			t.Fatalf("%s was deleted: %v", keep, got)
		}
	}

	info, _ := a.UndoState(tab.Path)
	if info.Undo != "Clean up 1 merged branch" {
		t.Fatalf("undo label = %q", info.Undo)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(localBranches(local), "feature/a") {
		t.Fatal("undo didn't bring feature/a back")
	}
}

func TestCleanupOffByDefault(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, _ := a.OpenRepo(local.Dir)
	local.Git("checkout", "-q", "-b", "feature/a", "main")
	local.CommitFile("a.txt", "a", "a")
	local.Git("push", "-q", "-u", "origin", "feature/a")
	local.Git("checkout", "-q", "main")
	remote.Git("branch", "-D", "feature/a")
	if err := a.Pull(tab.Path, ""); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(localBranches(local), "feature/a") {
		t.Fatal("cleanup ran with the setting off")
	}
}
