package git

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
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
	t.Parallel()
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
	t.Parallel()
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

// The upstream fields come from a second query, for only the branches that have one (#346).
func TestRefsUpstreamsOnlyWhereConfigured(t *testing.T) {
	t.Parallel()
	local, _ := gittest.WithRemote(t, 2)
	local.Git("branch", "plain")
	local.Git("branch", "dir/tracking", "--track", "origin/main")
	local.Git("branch", "dir/plain")
	refs := refsByName(t, local)

	for _, name := range []string{"main", "dir/tracking"} {
		if got := refs["refs/heads/"+name].Upstream; got != "origin/main" {
			t.Errorf("%s upstream = %q, want origin/main", name, got)
		}
	}
	for _, name := range []string{"plain", "dir/plain"} {
		r := refs["refs/heads/"+name]
		if r.Upstream != "" || r.Gone || r.Ahead != 0 || r.Behind != 0 {
			t.Errorf("%s should have no upstream: %+v", name, r)
		}
	}
	if got := refs["refs/remotes/origin/main"].Upstream; got != "" {
		t.Errorf("a remote branch has no upstream, got %q", got)
	}
}

func TestRefsNoUpstreamsAtAll(t *testing.T) {
	t.Parallel()
	fx := gittest.WithMergeHistory(t)
	refs := refsByName(t, fx)
	if len(refs) == 0 {
		t.Fatal("expected refs")
	}
	for name, r := range refs {
		if r.Upstream != "" {
			t.Errorf("%s has upstream %q", name, r.Upstream)
		}
	}
}

func TestRefsConfigForDeletedBranch(t *testing.T) {
	t.Parallel()
	local, _ := gittest.WithRemote(t, 1)
	local.Git("config", "branch.deleted.remote", "origin")
	local.Git("config", "branch.deleted.merge", "refs/heads/deleted")
	refs := refsByName(t, local)
	if _, ok := refs["refs/heads/deleted"]; ok {
		t.Fatal("config alone shouldn't make a branch appear")
	}
	if got := refs["refs/heads/main"].Upstream; got != "origin/main" {
		t.Fatalf("main upstream = %q", got)
	}
}

func TestRefsOnlyDeletedBranchesConfigured(t *testing.T) {
	t.Parallel()
	fx := gittest.WithMergeHistory(t)
	fx.Git("config", "branch.deleted.remote", "origin")
	fx.Git("config", "branch.deleted.merge", "refs/heads/deleted")
	if _, ok := refsByName(t, fx)["refs/heads/main"]; !ok {
		t.Fatal("main should be listed")
	}
}

func TestRefsTooManyUpstreamsForACommandLine(t *testing.T) {
	old := maxUpstreamPatternBytes
	maxUpstreamPatternBytes = 10
	t.Cleanup(func() { maxUpstreamPatternBytes = old })

	local, _ := gittest.WithRemote(t, 1)
	local.Git("branch", "plain")
	refs := refsByName(t, local)
	if got := refs["refs/heads/main"].Upstream; got != "origin/main" {
		t.Fatalf("main upstream = %q", got)
	}
	if got := refs["refs/heads/plain"].Upstream; got != "" {
		t.Fatalf("plain upstream = %q", got)
	}
}

func BenchmarkRefsManyBranches(b *testing.B) {
	fx := gittest.New(b)
	var in strings.Builder
	for i := range 5000 {
		fmt.Fprintf(&in, "commit refs/heads/b/%d/x%d\ncommitter A <a@b> %d +0000\ndata 2\nc\n\n", i%100, i, 1700000000+i)
	}
	cmd := exec.Command("git", "fast-import", "--quiet")
	cmd.Dir = fx.Dir
	cmd.Stdin = strings.NewReader(in.String())
	if err := cmd.Run(); err != nil {
		b.Fatal(err)
	}
	fx.Git("pack-refs", "--all")
	repo := OpenRepo(NewRunner("git"), fx.Dir)
	for b.Loop() {
		if _, err := repo.Refs(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRefsRemoteTracking(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	refs, err := OpenRepo(newTestRunner(t), gittest.New(t).Dir).Refs(context.Background())
	if err != nil || len(refs) != 0 {
		t.Fatalf("refs = %v, err = %v", refs, err)
	}
}
