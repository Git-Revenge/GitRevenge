package app

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	a := New()
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	t.Cleanup(func() { a.Shutdown(t.Context()) })
	return a
}

func TestOpenRepoTabAndRecents(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	fx.Branch("feature")

	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Head != "feature" || tab.Name != filepath.Base(tab.Path) {
		t.Fatalf("tab = %+v", tab)
	}
	recent, err := a.RecentRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 || !recent[0].Exists {
		t.Fatalf("recent = %+v", recent)
	}
	if err := a.RemoveRecentRepo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if recent, _ := a.RecentRepos(); len(recent) != 0 {
		t.Fatal("recent entry not removed")
	}
}

func TestOpenRepoHeadStates(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)

	empty := gittest.New(t)
	tab, err := a.OpenRepo(empty.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Head != "main" {
		t.Fatalf("unborn branch head = %q, want main", tab.Head)
	}

	detached := gittest.New(t)
	sha := detached.Linear(2)[0]
	detached.Checkout(sha)
	tab, err = a.OpenRepo(detached.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tab.Head, "detached at ") {
		t.Fatalf("detached head = %q", tab.Head)
	}
}

func TestOpenRepoFriendlyError(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	_, err := a.OpenRepo(t.TempDir())
	if err == nil || err.Error() != "That folder isn't a git repository." {
		t.Fatalf("err = %v", err)
	}
}

func TestRestoreSession(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	path := a.settingsPath
	a.Shutdown(t.Context())

	b := New()
	b.settingsPath = path
	t.Cleanup(func() { b.Shutdown(t.Context()) })
	st, err := b.RestoreSession()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Tabs) != 1 || st.Tabs[0].Path != tab.Path || st.Active != tab.Path || len(st.Failed) != 0 {
		t.Fatalf("restored = %+v", st)
	}
}
