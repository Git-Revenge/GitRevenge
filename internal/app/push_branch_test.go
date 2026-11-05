package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// pushBranchApp opens a clone on main with a local feat that tracks nothing, recording notices.
func pushBranchApp(t *testing.T) (a *App, path string, local, remote *gittest.Repo, notices func() []string) {
	t.Helper()
	isolateGit(t)
	var mu sync.Mutex
	var said []string
	a = newTestApp(t)
	a.emitHook = func(name string, data any) {
		if name != "notice" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		said = append(said, data.(Notice).Message)
	}
	local, remote = gittest.WithRemote(t, 1)
	local.Git("checkout", "-q", "-b", "feat")
	local.CommitFile("feat.txt", "1", "feat 1")
	local.Git("checkout", "-q", "main")
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return a, tab.Path, local, remote, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), said...)
	}
}

func lastOf(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

func TestPushBranchPublishesANewBranchAndTracksIt(t *testing.T) {
	a, path, local, remote, notices := pushBranchApp(t)
	if err := a.PushBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}
	if got, want := remote.Git("rev-parse", "refs/heads/feat"), local.Git("rev-parse", "feat"); got != want {
		t.Errorf("remote feat = %s, want %s", got, want)
	}
	if got := local.Git("rev-parse", "--abbrev-ref", "feat@{upstream}"); got != "origin/feat" {
		t.Errorf("upstream = %q", got)
	}
	if got := local.Git("rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Errorf("HEAD moved to %q", got)
	}
	if got := lastOf(notices()); got != "Pushed feat to origin." {
		t.Errorf("notice = %q", got)
	}
}

func TestPushBranchFollowsTheUpstreamName(t *testing.T) {
	a, path, local, remote, notices := pushBranchApp(t)
	local.Git("push", "-q", "origin", "feat:other")
	local.Git("branch", "-q", "--set-upstream-to=origin/other", "feat")
	local.Git("checkout", "-q", "feat")
	local.CommitFile("more.txt", "2", "feat 2")
	local.Git("checkout", "-q", "main")

	if err := a.PushBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}
	if got, want := remote.Git("rev-parse", "refs/heads/other"), local.Git("rev-parse", "feat"); got != want {
		t.Errorf("remote other = %s, want %s", got, want)
	}
	if got := remote.Git("for-each-ref", "refs/heads/feat"); got != "" {
		t.Errorf("made a stray remote branch: %s", got)
	}
	if got := lastOf(notices()); got != "Pushed feat to origin/other." {
		t.Errorf("notice = %q", got)
	}
}

func TestPushBranchUsesTheOnlyRemote(t *testing.T) {
	a, path, local, remote, _ := pushBranchApp(t)
	local.Git("remote", "rename", "origin", "fork")
	if err := a.PushBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}
	if got, want := remote.Git("rev-parse", "refs/heads/feat"), local.Git("rev-parse", "feat"); got != want {
		t.Errorf("remote feat = %s, want %s", got, want)
	}
	if got := local.Git("rev-parse", "--abbrev-ref", "feat@{upstream}"); got != "fork/feat" {
		t.Errorf("upstream = %q", got)
	}
}

func TestPushBranchTreatsALocalUpstreamAsNone(t *testing.T) {
	a, path, local, remote, _ := pushBranchApp(t)
	local.Git("branch", "-q", "--set-upstream-to=main", "feat")
	if err := a.PushBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}
	if got, want := remote.Git("rev-parse", "refs/heads/feat"), local.Git("rev-parse", "feat"); got != want {
		t.Errorf("remote feat = %s, want %s", got, want)
	}
}

func TestPushBranchWithNothingToSendSaysSo(t *testing.T) {
	a, path, local, _, notices := pushBranchApp(t)
	local.Git("push", "-q", "-u", "origin", "feat")
	if err := a.PushBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}
	if got := lastOf(notices()); got != "feat is already up to date on origin." {
		t.Errorf("notice = %q", got)
	}
}

func TestPushBranchWorksOnTheCheckedOutBranch(t *testing.T) {
	a, path, local, remote, notices := pushBranchApp(t)
	local.CommitFile("m.txt", "m", "on main")
	if err := a.PushBranch(path, "main"); err != nil {
		t.Fatal(err)
	}
	if got, want := remote.Git("rev-parse", "refs/heads/main"), local.Git("rev-parse", "main"); got != want {
		t.Errorf("remote main = %s, want %s", got, want)
	}
	if got := lastOf(notices()); got != "Pushed main to origin." {
		t.Errorf("notice = %q", got)
	}
}

func TestPushBranchExplainsARefusal(t *testing.T) {
	a, path, local, remote, notices := pushBranchApp(t)
	local.Git("push", "-q", "-u", "origin", "feat")
	other := gittest.Clone(t, remote)
	other.Git("checkout", "-q", "feat")
	other.CommitFile("o.txt", "o", "theirs")
	other.Git("push", "-q", "origin", "feat")
	local.Git("checkout", "-q", "feat")
	local.CommitFile("l.txt", "l", "mine")
	local.Git("checkout", "-q", "main")

	err := a.PushBranch(path, "feat")
	if err == nil || !strings.Contains(err.Error(), "Pull first") {
		t.Fatalf("err = %v", err)
	}
	for _, n := range notices() {
		if strings.HasPrefix(n, "Pushed") {
			t.Errorf("claimed a push that was refused: %q", n)
		}
	}
}

func TestPushBranchWithNoRemotes(t *testing.T) {
	a, path, local, _, _ := pushBranchApp(t)
	local.Git("remote", "remove", "origin")
	if err := a.PushBranch(path, "feat"); err == nil || !strings.Contains(err.Error(), "no remotes") {
		t.Fatalf("err = %v", err)
	}
}
