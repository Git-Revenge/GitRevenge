package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestRestackAfterSquashMerge(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// base is pushed; top is stacked on it.
	local.Git("checkout", "-q", "-b", "base", "main")
	local.CommitFile("b.txt", "base\n", "base work")
	local.Git("push", "-q", "-u", "origin", "base")
	local.Git("checkout", "-q", "-b", "top")
	local.CommitFile("t.txt", "top\n", "top work")
	local.Git("checkout", "-q", "main")

	// On the host, base is squash-merged into main and deleted.
	host := gittest.Clone(t, remote)
	host.Git("merge", "-q", "--squash", "origin/base")
	host.Git("commit", "-qm", "base work (#1)")
	host.Git("push", "-q", "origin", "main")
	remote.Git("branch", "-D", "base")

	if err := a.Fetch(tab.Path, "origin"); err != nil {
		t.Fatal(err)
	}
	refs := refsAfterReload(t, a, tab.Path, func(rs []git.Ref) bool { return stackOf(rs, "top").StackParentMerged })
	if stackOf(refs, "top").StackParent != "base" {
		t.Fatalf("top = %+v", stackOf(refs, "top"))
	}

	if err := a.Restack(tab.Path, "top"); err != nil {
		t.Fatal(err)
	}
	own := strings.Fields(local.Git("log", "--format=%s", "origin/main..top"))
	if strings.Join(own, " ") != "top work" {
		t.Fatalf("top's own commits after restack = %q, want only top work", own)
	}
	if local.Git("rev-parse", "top~1") != local.Git("rev-parse", "origin/main") {
		t.Fatal("top should sit directly on origin/main")
	}

	// Undo puts top back where it was (two steps: the restack, then the checkout).
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(local.Git("log", "-1", "--format=%s", "top~1")) != "base work" {
		t.Fatal("undo didn't bring top back onto base")
	}
	if err := a.Restack(tab.Path, "main"); err == nil {
		t.Fatal("restacking a branch that isn't stacked was accepted")
	}
}
