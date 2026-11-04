package app

import (
	"slices"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// refsAfterReload waits for the refs to match want after a settings change reloads them.
func refsAfterReload(t *testing.T, a *App, path string, ok func([]git.Ref) bool) []git.Ref {
	t.Helper()
	var refs []git.Ref
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		loadedPage(t, a, path)
		if refs, _ = a.Refs(path); ok(refs) {
			return refs
		}
	}
	t.Fatal("refs never matched")
	return refs
}

func stackOf(refs []git.Ref, name string) git.Ref {
	for _, r := range refs {
		if r.Kind == git.LocalBranch && r.Short == name {
			return r
		}
	}
	return git.Ref{}
}

func TestStacksInRefs(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "base")
	fx.Branch("one")
	fx.CommitFile("one.txt", "1", "one")
	fx.Branch("two")
	fx.CommitFile("two.txt", "1", "two")
	fx.Checkout("main")
	fx.Branch("other")
	fx.CommitFile("other.txt", "1", "other")
	fx.Checkout("main")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	refs := refsAfterReload(t, a, tab.Path, func(rs []git.Ref) bool { return stackOf(rs, "two").StackParent == "one" })
	if one := stackOf(refs, "one"); one.StackParent != "" || one.StackDepth != 1 || one.StackSize != 2 {
		t.Fatalf("one = %+v, want the bottom of a 2-branch stack", one)
	}
	if two := stackOf(refs, "two"); two.StackDepth != 2 || two.StackSize != 2 {
		t.Fatalf("two = %+v, want 2 of 2", two)
	}
	if o := stackOf(refs, "other"); o.StackDepth != 0 || stackOf(refs, "main").StackDepth != 0 {
		t.Fatal("branches outside a stack should carry no stack")
	}

	// Override: other is stacked on two by hand; then two is set to not stacked.
	if err := a.SetStackParent(tab.Path, "other", "two", false); err != nil {
		t.Fatal(err)
	}
	refsAfterReload(t, a, tab.Path, func(rs []git.Ref) bool {
		o := stackOf(rs, "other")
		return o.StackParent == "two" && o.StackDepth == 3 && stackOf(rs, "one").StackSize == 3
	})
	if err := a.SetStackParent(tab.Path, "two", "", false); err != nil {
		t.Fatal(err)
	}
	refsAfterReload(t, a, tab.Path, func(rs []git.Ref) bool {
		return stackOf(rs, "two").StackParent == "" && stackOf(rs, "other").StackDepth == 2
	})
	// Auto: back to the inferred parent.
	if err := a.SetStackParent(tab.Path, "two", "", true); err != nil {
		t.Fatal(err)
	}
	refsAfterReload(t, a, tab.Path, func(rs []git.Ref) bool { return stackOf(rs, "two").StackParent == "one" })
	if err := a.SetStackParent(tab.Path, "two", "two", false); err == nil {
		t.Fatal("a branch stacked on itself was accepted")
	}
}

func TestCleanupKeepsStackParents(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, _ := a.OpenRepo(local.Dir)
	p, _ := a.RepoPreferences(tab.Path)
	p.CleanupOnPull = true
	if err := a.SetRepoPreferences(tab.Path, p); err != nil {
		t.Fatal(err)
	}
	// base is pushed and then deleted on the remote, but top is stacked on it.
	local.Git("checkout", "-q", "-b", "base", "main")
	local.CommitFile("b.txt", "1", "base work")
	local.Git("push", "-q", "-u", "origin", "base")
	local.Git("checkout", "-q", "-b", "top")
	local.CommitFile("t.txt", "1", "top work")
	local.Git("checkout", "-q", "main")
	remote.Git("branch", "-D", "base")
	if err := a.Pull(tab.Path, ""); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(localBranches(local), "base") {
		t.Fatal("cleanup deleted base, which top is stacked on")
	}
}
