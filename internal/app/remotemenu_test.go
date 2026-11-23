package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// hiddenRefs waits until the hidden refs are exactly want, by short name, as the graph
// reloads, and returns the last ones it saw. Waiting on a count alone would pass on the
// graph before a rename reloads it.
func hiddenRefs(t *testing.T, a *App, path string, want ...string) []string {
	t.Helper()
	var hidden []string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		loadedPage(t, a, path)
		refs, err := a.Refs(path)
		if err != nil {
			t.Fatal(err)
		}
		hidden = nil
		for _, r := range refs {
			if r.Hidden {
				hidden = append(hidden, r.Short)
			}
		}
		slices.Sort(hidden)
		if slices.Equal(hidden, want) {
			break
		}
	}
	return hidden
}

func TestHideRemote(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Commit("one")
	fx.Git("remote", "add", "upstream", "https://github.com/o/r.git")
	fx.Git("remote", "add", "origin", "https://github.com/me/r.git")
	for _, ref := range []string{"refs/remotes/upstream/main", "refs/remotes/upstream/fix", "refs/remotes/origin/main"} {
		fx.Git("update-ref", ref, "HEAD")
	}
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.HideRemote(tab.Path, "upstream", true); err != nil {
		t.Fatal(err)
	}
	// Only upstream's branches: not origin/main, and not the local main of the same name.
	if got := hiddenRefs(t, a, tab.Path, "upstream/fix", "upstream/main"); !slices.Equal(got, []string{"upstream/fix", "upstream/main"}) {
		t.Fatalf("hidden = %v", got)
	}
	remoteHidden := func() map[string]bool {
		rs, err := a.Remotes(tab.Path)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, r := range rs {
			out[r.Name] = r.Hidden
		}
		return out
	}
	if h := remoteHidden(); !h["upstream"] || h["origin"] {
		t.Fatalf("remotes hidden = %v", h)
	}

	// The setting follows a rename, and goes with a removal.
	if err := a.RemoteAction(tab.Path, "rename", []string{"upstream", "theirs"}); err != nil {
		t.Fatal(err)
	}
	if h := remoteHidden(); !h["theirs"] {
		t.Fatalf("after rename: %v", h)
	}
	if got := hiddenRefs(t, a, tab.Path, "theirs/fix", "theirs/main"); !slices.Equal(got, []string{"theirs/fix", "theirs/main"}) {
		t.Fatalf("hidden after rename = %v", got)
	}
	if err := a.HideRemote(tab.Path, "theirs", false); err != nil {
		t.Fatal(err)
	}
	if got := hiddenRefs(t, a, tab.Path); len(got) != 0 {
		t.Fatalf("hidden after showing = %v", got)
	}
	if err := a.HideRemote(tab.Path, "theirs", true); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoteAction(tab.Path, "remove", []string{"theirs"}); err != nil {
		t.Fatal(err)
	}
	if hidden := a.hiddenRemotes(tab.Path); len(hidden) != 0 {
		t.Fatalf("a removed remote stays hidden: %v", hidden)
	}
}

func TestSetPushDefault(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Commit("one")
	fx.Git("remote", "add", "origin", "https://github.com/me/r.git")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetPushDefault(tab.Path, "origin"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(fx.Git("config", "--local", "remote.pushDefault")); got != "origin" {
		t.Fatalf("pushDefault = %q", got)
	}
	if s, _ := a.RemoteSettings(tab.Path); s.PushDefault != "origin" {
		t.Fatalf("settings = %+v", s)
	}
	if err := a.SetPushDefault(tab.Path, "nowhere"); err == nil {
		t.Fatal("set to a remote that isn't there")
	}
	if err := a.SetPushDefault(tab.Path, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.SetPushDefault(tab.Path, ""); err != nil {
		t.Fatalf("unsetting twice: %v", err)
	}
	if s, _ := a.RemoteSettings(tab.Path); s.PushDefault != "" {
		t.Fatalf("still set: %+v", s)
	}
}

func TestRemoteFolder(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	bare := filepath.Join(t.TempDir(), "bare.git")
	if err := os.Mkdir(bare, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, "sibling"), 0o700); err != nil {
		t.Fatal(err)
	}
	fileURL := "file://" + filepath.ToSlash(bare)
	if !strings.HasPrefix(filepath.ToSlash(bare), "/") {
		fileURL = "file:///" + filepath.ToSlash(bare)
	}
	cases := map[string]string{
		bare:                                   bare,
		fileURL:                                bare,
		"sibling":                              filepath.Join(repo, "sibling"),
		filepath.Join(t.TempDir(), "gone.git"): "",
		"git@github.com:o/r.git":               "",
		"https://github.com/o/r.git":           "",
		"":                                     "",
	}
	for in, want := range cases {
		if got := remoteFolder(repo, in); got != want {
			t.Errorf("remoteFolder(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetchBranchAndStatus(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 2)
	// Two branches appear on the remote after the clone: only the one asked for is fetched.
	other := gittest.Clone(t, remote)
	other.Git("switch", "-q", "-c", "feature")
	other.Commit("feature one")
	other.Commit("feature two")
	other.Git("push", "-q", "origin", "feature")
	other.Git("switch", "-q", "-c", "other")
	other.Git("push", "-q", "origin", "other")
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.FetchBranch(tab.Path, "origin", "feature"); err != nil {
		t.Fatal(err)
	}
	if out := local.Git("for-each-ref", "--format=%(refname:short)", "refs/remotes/origin/"); !strings.Contains(out, "origin/feature") || strings.Contains(out, "origin/other") {
		t.Fatalf("remote branches after fetching one:\n%s", out)
	}
	st, err := a.RemoteBranchStatus(tab.Path, "origin", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if st != (RemoteBranchStatus{Base: "main", Ahead: 2, Behind: 0}) {
		t.Fatalf("status = %+v", st)
	}
	if st, _ := a.RemoteBranchStatus(tab.Path, "origin", "main"); st != (RemoteBranchStatus{Base: "main"}) {
		t.Fatalf("main against itself = %+v", st)
	}
	if err := a.FetchBranch(tab.Path, "origin", "-x"); err == nil {
		t.Fatal("fetched a branch named like an option")
	}
}
