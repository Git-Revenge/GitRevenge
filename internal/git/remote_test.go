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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestFetchFailuresReadsBothOfGitsWordings(t *testing.T) {
	t.Parallel()
	const gone = "ERROR: Repository not found.\nfatal: Could not read from remote repository.\n\n" +
		"Please make sure you have the correct access rights\nand the repository exists.\n"
	const offline = "ssh: Could not resolve hostname example.org: Name or service not known\nfatal: Could not read from remote repository.\n"
	for name, stderr := range map[string]string{
		"together": gone + "could not fetch 'ghsa-m5wc' (exit code: 128)\n" + offline + "could not fetch 'it's' (exit code: 128)\n",
		"in turn":  "From github.com:a/b\n   1111111..2222222  main       -> origin/main\n" + gone + "error: could not fetch ghsa-m5wc\n" + offline + "error: could not fetch it's\n",
		"old git":  gone + "error: Could not fetch ghsa-m5wc\r\n" + offline + "error: Could not fetch it's\r\n",
	} {
		got := FetchFailures(&Error{Stderr: stderr})
		if len(got) != 2 || got[0].Remote != "ghsa-m5wc" || got[1].Remote != "it's" {
			t.Errorf("%s: failures = %+v", name, got)
			continue
		}
		if !strings.Contains(got[0].Output, "Repository not found") || strings.Contains(got[0].Output, "resolve hostname") {
			t.Errorf("%s: first remote's output = %q", name, got[0].Output)
		}
		if !strings.HasPrefix(got[1].Output, "ssh: Could not resolve hostname") || strings.Contains(got[1].Output, "Repository not found") {
			t.Errorf("%s: second remote's output = %q", name, got[1].Output)
		}
	}
	// One remote fails without git naming it, and other errors aren't fetches at all.
	for _, err := range []error{&Error{Stderr: gone}, errors.New("could not fetch 'x' (exit code: 1)"), nil} {
		if got := FetchFailures(err); len(got) != 0 {
			t.Errorf("FetchFailures(%v) = %+v, want none", err, got)
		}
	}
}

// Git's own output for a fetch of every remote, taken in turn and together.
func TestFetchFailuresFromGit(t *testing.T) {
	t.Parallel()
	for _, parallel := range []string{"1", "0"} {
		local, _ := gittest.WithRemote(t, 1)
		local.Git("remote", "add", "gone", local.Dir+"-nowhere.git")
		local.Git("remote", "add", "parked", local.Dir+"-parked.git")
		local.Git("config", "remote.parked.skipFetchAll", "true")
		local.Git("config", "fetch.parallel", parallel)
		repo := OpenRepo(newTestRunner(t), local.Dir)
		ctx := context.Background()

		err := repo.Fetch(ctx, "", NetOptions{})
		got := FetchFailures(err)
		if len(got) != 1 || got[0].Remote != "gone" || !strings.Contains(got[0].Output, "does not appear to be a git repository") {
			t.Errorf("fetch.parallel=%s: failures = %+v (err %v)", parallel, got, err)
		}
		asked, err := repo.FetchAllRemotes(ctx)
		if err != nil || strings.Join(asked, ",") != "gone,origin" {
			t.Errorf("FetchAllRemotes = %v, %v, want gone and origin", asked, err)
		}
	}
}

func TestPushNewBranchSetsUpstreamAndTags(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestRemoteHost(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"https://GitHub.com/Git-Revenge/GitRevenge.git":  "github.com",
		"git@github.com:Git-Revenge/GitRevenge.git":      "github.com",
		"https://user:token@github.com/o/r.git":          "github.com",
		"ssh://git@gitlab.example.com:2222/team/app.git": "gitlab.example.com",
		"/srv/git/local.git":                             "",
		"":                                               "",
	}
	for in, want := range cases {
		if got := RemoteHost(in); got != want {
			t.Errorf("RemoteHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// #923: fetches prune unless config says not to, and never touch local branches.
func TestFetchPrunesByDefault(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*gittest.Repo, *Repo) {
		local, remote := gittest.WithRemote(t, 1)
		other := gittest.Clone(t, remote)
		other.Git("push", "-q", "origin", "main:gone-soon")
		local.Git("fetch", "-q", "origin")
		// A local branch tracking it, which must survive.
		local.Git("branch", "-q", "--track", "gone-soon", "origin/gone-soon")
		other.Git("push", "-q", "origin", "--delete", "gone-soon")
		return local, OpenRepo(newTestRunner(t), local.Dir)
	}
	has := func(fx *gittest.Repo, ref string) bool {
		return strings.TrimSpace(fx.Git("for-each-ref", "--format=%(refname)", ref)) != ""
	}
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		config [][2]string
		pruned bool
	}{
		{"no config prunes", nil, true},
		{"remote prune false keeps", [][2]string{{"remote.origin.prune", "false"}}, false},
		{"fetch prune false keeps", [][2]string{{"fetch.prune", "false"}}, false},
		{"remote prune false beats fetch prune true", [][2]string{{"fetch.prune", "true"}, {"remote.origin.prune", "false"}}, false},
		{"fetch prune true prunes", [][2]string{{"fetch.prune", "true"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, fetchOrPull := range []string{"fetch", "pull"} {
				fx, repo := setup(t)
				for _, kv := range tc.config {
					fx.Git("config", kv[0], kv[1])
				}
				var err error
				if fetchOrPull == "fetch" {
					err = repo.Fetch(ctx, "origin", NetOptions{})
				} else {
					err = repo.Pull(ctx, PullFFOnly, NetOptions{})
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := !has(fx, "refs/remotes/origin/gone-soon"); got != tc.pruned {
					t.Errorf("%s: pruned = %v, want %v", fetchOrPull, got, tc.pruned)
				}
				if !has(fx, "refs/heads/gone-soon") {
					t.Errorf("%s: the local branch went too", fetchOrPull)
				}
			}
		})
	}
	// Every remote at once follows the same rule.
	fx, repo := setup(t)
	if err := repo.Fetch(ctx, "", NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if has(fx, "refs/remotes/origin/gone-soon") {
		t.Error("fetching every remote didn't prune")
	}
}
