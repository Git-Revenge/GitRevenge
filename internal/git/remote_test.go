package git

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestRemotesManage(t *testing.T) {
	local, remote := gittest.WithRemote(t, 1)
	repo := OpenRepo(newTestRunner(t), local.Dir)
	ctx := context.Background()

	rs, err := repo.Remotes(ctx)
	if err != nil || len(rs) != 1 || rs[0].Name != "origin" || rs[0].FetchURL == "" || rs[0].PushURL == "" {
		t.Fatalf("remotes = %+v, %v", rs, err)
	}
	if err := repo.AddRemote(ctx, "upstream", remote.Dir); err != nil {
		t.Fatal(err)
	}
	if err := repo.RenameRemote(ctx, "upstream", "mirror"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetRemoteURL(ctx, "mirror", "https://example.com/x.git"); err != nil {
		t.Fatal(err)
	}
	rs, _ = repo.Remotes(ctx)
	byName := map[string]Remote{}
	for _, r := range rs {
		byName[r.Name] = r
	}
	if len(rs) != 2 || byName["mirror"].FetchURL != "https://example.com/x.git" {
		t.Fatalf("after edits: %+v", rs)
	}
	if err := repo.RemoveRemote(ctx, "mirror"); err != nil {
		t.Fatal(err)
	}
	if rs, _ := repo.Remotes(ctx); len(rs) != 1 {
		t.Fatalf("after remove: %+v", rs)
	}
	// A name starting with "-" must not be read as an option.
	if err := repo.AddRemote(ctx, "-bad", "x"); err == nil {
		t.Fatal("dash-prefixed remote name accepted")
	}
}

func TestParseProgress(t *testing.T) {
	p := ParseProgress("Receiving objects:  45% (450/1000), 1.2 MiB | 2 MiB/s")
	if p.Phase != "Receiving objects" || p.Percent != 45 {
		t.Fatalf("%+v", p)
	}
	if p := ParseProgress("remote: Counting objects: 100% (5/5), done."); p.Phase != "Counting objects" || p.Percent != 100 {
		t.Fatalf("%+v", p)
	}
	if p := ParseProgress("From /tmp/x"); p.Percent != -1 || p.Phase != "" {
		t.Fatalf("%+v", p)
	}
}

func TestFetchPullPush(t *testing.T) {
	local, remote := gittest.WithRemote(t, 1)
	other := gittest.Clone(t, remote)
	other.CommitFile("theirs.txt", "t", "their work")
	other.Git("push", "-q", "origin", "main")
	repo := OpenRepo(newTestRunner(t), local.Dir)
	ctx := context.Background()

	var mu sync.Mutex
	var lines []string
	o := NetOptions{OnProgress: func(p Progress) {
		mu.Lock()
		lines = append(lines, p.Line)
		mu.Unlock()
	}}
	if err := repo.Fetch(ctx, "origin", o); err != nil {
		t.Fatal(err)
	}
	if local.Git("rev-parse", "origin/main") != other.Head() {
		t.Fatal("fetch did not update origin/main")
	}
	mu.Lock()
	gotProgress := len(lines) > 0
	mu.Unlock()
	if !gotProgress {
		t.Fatal("expected progress lines from fetch")
	}

	if err := repo.Pull(ctx, PullFFOnly, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if local.Head() != other.Head() {
		t.Fatal("pull did not fast-forward")
	}

	local.CommitFile("mine.txt", "m", "my work")
	if err := repo.Push(ctx, PushOptions{Remote: "origin", Refspec: "main"}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if remote.Git("rev-parse", "main") != local.Head() {
		t.Fatal("push did not update the remote")
	}
}

func TestPushNewBranchSetsUpstreamAndTags(t *testing.T) {
	local, remote := gittest.WithRemote(t, 1)
	repo := OpenRepo(newTestRunner(t), local.Dir)
	ctx := context.Background()
	local.Branch("feature")
	local.CommitFile("f.txt", "f", "feature")

	if err := repo.Push(ctx, PushOptions{Remote: "origin", Refspec: "feature", SetUpstream: true}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := local.Git("rev-parse", "--abbrev-ref", "feature@{upstream}"); got != "origin/feature" {
		t.Fatalf("upstream = %q", got)
	}
	local.Git("tag", "v1")
	if err := repo.Push(ctx, PushOptions{Remote: "origin", Refspec: "refs/tags/v1"}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if remote.Git("rev-parse", "v1") != local.Head() {
		t.Fatal("tag not pushed")
	}
	// Deleting a remote branch is a push of ":branch".
	if err := repo.Push(ctx, PushOptions{Remote: "origin", Refspec: ":feature"}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.TryGit("rev-parse", "--verify", "feature"); err == nil {
		t.Fatal("remote branch should be deleted")
	}
}

func TestPushRejectedAndForceWithLease(t *testing.T) {
	local, remote := gittest.WithRemote(t, 1)
	other := gittest.Clone(t, remote)
	other.CommitFile("o.txt", "o", "other pushes first")
	other.Git("push", "-q", "origin", "main")
	repo := OpenRepo(newTestRunner(t), local.Dir)
	ctx := context.Background()
	local.CommitFile("l.txt", "l", "diverged")

	err := repo.Push(ctx, PushOptions{Remote: "origin", Refspec: "main"}, NetOptions{})
	var gerr *Error
	if !errors.As(err, &gerr) || !strings.Contains(gerr.Stderr+gerr.Error(), "rejected") && !strings.Contains(gerr.Stderr, "fetch first") {
		t.Fatalf("expected a rejected push: %v", err)
	}
	// We never fetched the other commit, so the lease is stale: must still be refused.
	if err := repo.Push(ctx, PushOptions{Remote: "origin", Refspec: "main", ForceWithLease: true}, NetOptions{}); err == nil {
		t.Fatal("force-with-lease must refuse when the remote moved since we last fetched")
	}
	local.Git("fetch", "-q")
	if err := repo.Push(ctx, PushOptions{Remote: "origin", Refspec: "main", ForceWithLease: true}, NetOptions{}); err != nil {
		t.Fatalf("after fetching, force-with-lease should overwrite: %v", err)
	}
	if remote.Git("rev-parse", "main") != local.Head() {
		t.Fatal("remote should now have our commit")
	}
}

func TestIssueURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/Git-Revenge/GitRevenge.git":  "https://github.com/Git-Revenge/GitRevenge/issues/{n}",
		"git@github.com:Git-Revenge/GitRevenge.git":      "https://github.com/Git-Revenge/GitRevenge/issues/{n}",
		"ssh://git@github.com/Git-Revenge/GitRevenge":    "https://github.com/Git-Revenge/GitRevenge/issues/{n}",
		"https://user:token@github.com/o/r.git":          "https://github.com/o/r/issues/{n}",
		"https://gitlab.com/group/sub/proj.git":          "https://gitlab.com/group/sub/proj/-/issues/{n}",
		"ssh://git@gitlab.example.com:2222/team/app.git": "https://gitlab.example.com/team/app/-/issues/{n}",
		"https://bitbucket.org/o/r.git":                  "",
		"/srv/git/local.git":                             "",
		"":                                               "",
	}
	for in, want := range cases {
		if got := IssueURL(in); got != want {
			t.Errorf("IssueURL(%q) = %q, want %q", in, got, want)
		}
	}
}
