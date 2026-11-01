package git

import (
	"context"
	"strconv"
	"testing"

	"github.com/risadams/gitrevenge/internal/gittest"
)

func refsByName(t *testing.T, fx *gittest.Repo) map[string]Ref {
	t.Helper()
	refs, err := OpenRepo(newTestRunner(t), fx.Dir).Refs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]Ref{}
	for _, r := range refs {
		m[r.Name] = r
	}
	return m
}

func TestRefsKindsAndHead(t *testing.T) {
	fx := gittest.WithMergeHistory(t)
	fx.Git("tag", "light")
	refs := refsByName(t, fx)

	main := refs["refs/heads/main"]
	if main.Kind != LocalBranch || !main.Head || main.Short != "main" || main.Commit != fx.Head() {
		t.Fatalf("main = %+v", main)
	}
	if refs["refs/heads/topic"].Head {
		t.Fatal("only the checked-out branch is HEAD")
	}
	// Annotated tags are peeled to the commit.
	tag := refs["refs/tags/v0.1.0"]
	if tag.Kind != Tag || tag.Commit != fx.Git("rev-parse", "v0.1.0^{commit}") {
		t.Fatalf("annotated tag = %+v", tag)
	}
	if refs["refs/tags/light"].Commit != fx.Head() {
		t.Fatal("lightweight tag commit wrong")
	}
}

func TestRefsUpdated(t *testing.T) {
	fx := gittest.WithMergeHistory(t)
	fx.Git("tag", "light")
	refs := refsByName(t, fx)

	committed, err := strconv.ParseInt(fx.Git("log", "-1", "--format=%ct", "main"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if got := refs["refs/heads/main"].Updated; got != committed {
		t.Fatalf("branch updated = %d, want its commit date %d", got, committed)
	}
	if got := refs["refs/tags/light"].Updated; got != committed {
		t.Fatalf("lightweight tag updated = %d, want its commit date %d", got, committed)
	}
	tagged, err := strconv.ParseInt(fx.Git("for-each-ref", "--format=%(taggerdate:unix)", "refs/tags/v0.1.0"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if got := refs["refs/tags/v0.1.0"].Updated; got != tagged {
		t.Fatalf("annotated tag updated = %d, want its tag date %d", got, tagged)
	}
}

func TestRefsRemoteTracking(t *testing.T) {
	local, remote := gittest.WithRemote(t, 2)
	local.CommitFile("ahead.txt", "x", "local work")
	seed := gittest.Clone(t, remote)
	seed.CommitFile("theirs.txt", "y", "remote work")
	seed.Git("push", "-q", "origin", "main")
	local.Git("fetch", "-q")

	refs := refsByName(t, local)
	main := refs["refs/heads/main"]
	if main.Upstream != "origin/main" || main.Ahead != 1 || main.Behind != 1 {
		t.Fatalf("main tracking = %+v", main)
	}
	if r, ok := refs["refs/remotes/origin/main"]; !ok || r.Kind != RemoteBranch || r.Short != "origin/main" {
		t.Fatalf("remote branch = %+v", r)
	}
	if _, ok := refs["refs/remotes/origin/HEAD"]; ok {
		t.Fatal("symbolic origin/HEAD should be skipped")
	}
}

func TestRefsUpstreamGone(t *testing.T) {
	local, remote := gittest.WithRemote(t, 1)
	local.Branch("feature")
	local.Git("push", "-q", "-u", "origin", "feature")
	remote.Git("branch", "-D", "feature")
	local.Git("fetch", "-q", "--prune")
	if r := refsByName(t, local)["refs/heads/feature"]; !r.Gone {
		t.Fatalf("feature should report a gone upstream: %+v", r)
	}
}

func TestRefsStashes(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	fx.WriteFile("a.txt", "1")
	fx.Stash("first stash")
	fx.WriteFile("b.txt", "2")
	fx.Stash("second stash")

	refs := refsByName(t, fx)
	s0, s1 := refs["stash@{0}"], refs["stash@{1}"]
	if s0.Kind != Stash || s1.Kind != Stash {
		t.Fatalf("stashes = %+v / %+v", s0, s1)
	}
	if s0.Subject != "On main: second stash" || s1.Subject != "On main: first stash" {
		t.Fatalf("stash subjects = %q / %q", s0.Subject, s1.Subject)
	}
}

func TestRefsEmptyRepo(t *testing.T) {
	refs, err := OpenRepo(newTestRunner(t), gittest.New(t).Dir).Refs(context.Background())
	if err != nil || len(refs) != 0 {
		t.Fatalf("refs = %v, err = %v", refs, err)
	}
}
