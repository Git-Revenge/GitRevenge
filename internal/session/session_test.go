package session

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/fsutil"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/settings"
	"github.com/git-revenge/gitrevenge/internal/watch"
)

func newManager(t *testing.T) (*Manager, *settings.Store) {
	t.Helper()
	bin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	store, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(git.NewRunner(bin), store)
	t.Cleanup(m.CloseAll)
	return m, store
}

// realPath resolves symlinks (macOS temp dirs live under a /var -> /private/var symlink,
// and git reports the resolved path).
func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestOpenFromSubdirectoryResolvesRoot(t *testing.T) {
	m, store := newManager(t)
	fx := gittest.New(t)
	fx.CommitFile("src/deep/file.txt", "x", "add")

	r, err := m.Open(context.Background(), fx.Path("src/deep"))
	if err != nil {
		t.Fatal(err)
	}
	if !fsutil.SamePath(r.Path, realPath(t, fx.Dir)) {
		t.Fatalf("Path = %s, want %s", r.Path, fx.Dir)
	}
	s := store.Get()
	if len(s.OpenTabs) != 1 || s.ActiveTab != r.Path || len(s.Recent) != 1 {
		t.Fatalf("settings not updated: %+v", s)
	}
}

func TestOpenTwiceReturnsSameRepo(t *testing.T) {
	m, store := newManager(t)
	fx := gittest.New(t)
	a, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || len(m.List()) != 1 || len(store.Get().OpenTabs) != 1 {
		t.Fatal("opening the same repo twice must reuse the tab")
	}
}

func TestOpenRejectsNonRepoAndBare(t *testing.T) {
	m, _ := newManager(t)
	plain := t.TempDir()
	if _, err := m.Open(context.Background(), plain); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("plain dir: err = %v, want ErrNotRepo", err)
	}
	bare := gittest.NewBare(t)
	if _, err := m.Open(context.Background(), bare.Dir); !errors.Is(err, ErrBare) {
		t.Fatalf("bare repo: err = %v, want ErrBare", err)
	}
	if _, err := m.Open(context.Background(), filepath.Join(plain, "missing")); err == nil {
		t.Fatal("missing folder should fail")
	}
}

func TestCloseMovesActiveToNeighbour(t *testing.T) {
	m, store := newManager(t)
	var paths []string
	for range 3 {
		r, err := m.Open(context.Background(), gittest.New(t).Dir)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, r.Path)
	}
	if err := m.SetActive(paths[1]); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(paths[1]); err != nil {
		t.Fatal(err)
	}
	s := store.Get()
	if len(s.OpenTabs) != 2 || s.ActiveTab != paths[2] {
		t.Fatalf("after close: tabs=%v active=%s, want active %s", s.OpenTabs, s.ActiveTab, paths[2])
	}
	if len(s.Recent) != 3 {
		t.Fatal("closing a tab must not remove it from recents")
	}
}

func TestRestoreDropsMissingRepos(t *testing.T) {
	m, store := newManager(t)
	keep := gittest.New(t)
	gone := gittest.New(t)
	k, _ := m.Open(context.Background(), keep.Dir)
	g, _ := m.Open(context.Background(), gone.Dir)
	m.CloseAll()
	if err := os.RemoveAll(gone.Dir); err != nil {
		t.Fatal(err)
	}

	active, failed := m.Restore(context.Background())
	if len(failed) != 1 || failed[g.Path] == nil {
		t.Fatalf("failed = %v, want only %s", failed, g.Path)
	}
	if active != k.Path {
		t.Fatalf("active = %s, want %s (saved active tab was removed)", active, k.Path)
	}
	if s := store.Get(); len(s.OpenTabs) != 1 || s.OpenTabs[0] != k.Path {
		t.Fatalf("tabs after restore = %v", s.OpenTabs)
	}
}

func TestObjectsReaderIsWarm(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	sha := fx.Linear(1)[0]
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	info, err := r.Objects.Info("HEAD")
	if err != nil || info.ID != sha {
		t.Fatalf("Objects.Info(HEAD) = %+v, %v", info, err)
	}
}

func TestRestoreKeepsStartPage(t *testing.T) {
	m, store := newManager(t)
	if _, err := m.Open(context.Background(), gittest.New(t).Dir); err != nil {
		t.Fatal(err)
	}
	if err := m.SetActive(""); err != nil {
		t.Fatal(err)
	}
	m.CloseAll()

	active, failed := m.Restore(context.Background())
	if active != "" || len(failed) != 0 || store.Get().ActiveTab != "" {
		t.Fatalf("active = %q, failed = %v; the start page should stay selected", active, failed)
	}
}

func TestRestoreKeepsConfigTab(t *testing.T) {
	m, store := newManager(t)
	r, err := m.Open(context.Background(), gittest.New(t).Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(s *settings.Settings) { s.ConfigTab = "/home/me/.gitconfig" }); err != nil {
		t.Fatal(err)
	}
	if err := m.SetActive(settings.ConfigTabID); err != nil {
		t.Fatal(err)
	}
	m.CloseAll()
	if active, _ := m.Restore(context.Background()); active != settings.ConfigTabID {
		t.Fatalf("active = %q, want the Git Config tab", active)
	}

	// With the tab closed, its id falls back like any missing tab.
	if err := store.Update(func(s *settings.Settings) { s.ConfigTab = ""; s.ActiveTab = settings.ConfigTabID }); err != nil {
		t.Fatal(err)
	}
	m.CloseAll()
	if active, _ := m.Restore(context.Background()); active != r.Path {
		t.Fatalf("active = %q, want %s", active, r.Path)
	}
}

func TestRestoreKeepsInsightsTab(t *testing.T) {
	m, store := newManager(t)
	if _, err := m.Open(context.Background(), gittest.New(t).Dir); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(s *settings.Settings) { s.InsightsTab = "group:Harbour" }); err != nil {
		t.Fatal(err)
	}
	if err := m.SetActive(settings.InsightsTabID); err != nil {
		t.Fatal(err)
	}
	m.CloseAll()
	if active, _ := m.Restore(context.Background()); active != settings.InsightsTabID {
		t.Fatalf("active = %q, want the Insights tab", active)
	}
}

func TestOnChangeReportsRepoChanges(t *testing.T) {
	m, _ := newManager(t)
	changes := make(chan watch.Kind, 16)
	m.OnChange(func(_ string, k watch.Kind) { changes <- k })
	fx := gittest.New(t)
	fx.Linear(1)
	if _, err := m.Open(context.Background(), fx.Dir); err != nil {
		t.Fatal(err)
	}

	fx.Branch("feature")
	timeout := time.After(5 * time.Second)
	for {
		select {
		case k := <-changes:
			if k.Has(watch.Head) {
				return
			}
		case <-timeout:
			t.Fatal("no head change reported after checkout")
		}
	}
}

// waitGraph waits until the repo's graph has finished loading with want rows.
func waitGraph(t *testing.T, r *Repo, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		g, loading := r.Graph()
		if g != nil && !loading && g.Len() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	g, loading := r.Graph()
	t.Fatalf("graph never reached %d rows (have %d, loading %v)", want, g.Len(), loading)
}

func TestGraphLoadsAndReloadsOnCommit(t *testing.T) {
	m, _ := newManager(t)
	done := make(chan GraphProgress, 64)
	m.OnGraph(func(_ string, p GraphProgress) {
		if p.Done {
			done <- p
		}
	})
	fx := gittest.New(t)
	fx.Linear(3)
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 3)
	select {
	case p := <-done:
		if p.Err != nil || p.Rows != 3 {
			t.Fatalf("progress = %+v", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no done progress event")
	}

	fx.CommitFile("more.txt", "x", "fourth")
	waitGraph(t, r, 4)
}

func TestGraphShowsStashesOnTheirBase(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	base := fx.Linear(2)[1]
	fx.WriteFile("file.txt", "wip\n")
	fx.WriteFile("untracked.txt", "u\n")
	fx.Git("stash", "push", "-u", "-m", "my stash")
	stash := fx.Git("rev-parse", "refs/stash")

	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// 2 commits + 1 stash row; the stash's index and untracked commits stay hidden.
	waitGraph(t, r, 3)
	g, _ := r.Graph()
	rows := g.Rows(0, 3)
	if rows[0].ID != stash || rows[0].Parents != 1 {
		t.Fatalf("first row should be the stash with one parent: %+v", rows[0])
	}
	if row, ok := g.RowOf(base); !ok || row != 1 {
		t.Fatalf("base commit row = %d, %v", row, ok)
	}
}

// progressLog records graph progress events for a test.
type progressLog struct {
	mu     sync.Mutex
	events []GraphProgress
}

func (p *progressLog) add(_ string, e GraphProgress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, e)
}

// waitFor waits until an event matching ok arrives and returns it.
func (p *progressLog) waitFor(t *testing.T, what string, ok func(GraphProgress) bool) GraphProgress {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		for _, e := range p.events {
			if ok(e) {
				p.mu.Unlock()
				return e
			}
		}
		p.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no progress event: %s", what)
	return GraphProgress{}
}

func TestGraphPausesAheadOfDemand(t *testing.T) {
	m, _ := newManager(t)
	m.readAhead = 5
	var log progressLog
	m.OnGraph(log.add)
	fx := gittest.New(t)
	shas := fx.Linear(30)
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}

	log.waitFor(t, "paused at the read-ahead limit", func(p GraphProgress) bool { return p.Paused && p.Rows == 5 })
	if g, loading := r.Graph(); g.Len() != 5 || !loading {
		t.Fatalf("graph has %d rows, loading %v; want 5 and still loading", g.Len(), loading)
	}
	log.waitFor(t, "an estimate of the whole history", func(p GraphProgress) bool { return p.Estimate == 30 })

	// Scrolling further reads on to the new limit and pauses again.
	r.DemandRows(10)
	log.waitFor(t, "paused at the new limit", func(p GraphProgress) bool { return p.Paused && p.Rows == 15 })

	// Finding a commit that isn't loaded yet reads on until it appears.
	row, ok, err := r.FindRow(context.Background(), shas[0])
	if err != nil || !ok || row != 29 {
		t.Fatalf("FindRow(oldest) = %d, %v, %v; want 29", row, ok, err)
	}
	if _, ok, _ := r.FindRow(context.Background(), strings.Repeat("0", 40)); ok {
		t.Fatal("found a commit that isn't in the history")
	}
	log.waitFor(t, "done", func(p GraphProgress) bool { return p.Done && p.Rows == 30 && p.Estimate == 30 })
}

func TestGraphReloadWhilePausedSwapsAtTheLimit(t *testing.T) {
	m, _ := newManager(t)
	m.readAhead = 5
	var log progressLog
	m.OnGraph(log.add)
	fx := gittest.New(t)
	fx.Linear(20)
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	log.waitFor(t, "first pause", func(p GraphProgress) bool { return p.Paused })

	head := fx.CommitFile("new.txt", "x", "newest")
	log.waitFor(t, "reloaded graph swapped in", func(p GraphProgress) bool { return p.Reset && p.Paused })
	g, _ := r.Graph()
	if row, ok := g.RowOf(head); !ok || row != 0 {
		t.Fatalf("new commit at row %d (found %v), want 0", row, ok)
	}

	g, err = r.FullGraph(context.Background())
	if err != nil || g.Len() != 21 {
		t.Fatalf("full graph = %v rows, %v; want 21", g.Len(), err)
	}
}

func TestOpenSaysWhenItCantWatch(t *testing.T) {
	m, _ := newManager(t)
	failure := errors.New("no watching today")
	m.startWatch = func(watch.Options) (*watch.Watcher, error) { return nil, failure }
	var gotPath string
	var gotErr error
	m.OnWatchFailed(func(path string, err error) { gotPath, gotErr = path, err })

	fx := gittest.New(t)
	fx.Linear(1)
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != r.Path || !errors.Is(gotErr, failure) {
		t.Fatalf("OnWatchFailed got %q, %v", gotPath, gotErr)
	}
}
