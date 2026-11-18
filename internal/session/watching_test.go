package session

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/settings"
	"github.com/git-revenge/gitrevenge/internal/watch"
)

// watched lists the repositories being watched, in order.
func watched(m *Manager) []string {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	var paths []string
	for _, r := range m.watching {
		paths = append(paths, r.Path)
	}
	return paths
}

func expectWatched(t *testing.T, m *Manager, what string, want ...string) {
	t.Helper()
	if got := watched(m); !slices.Equal(got, want) {
		t.Fatalf("%s: watching %v, want %v", what, got, want)
	}
}

// changeLog collects what OnChange reports, by repository.
type changeLog struct {
	mu   sync.Mutex
	seen map[string]watch.Kind
}

func (c *changeLog) add(path string, k watch.Kind) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen == nil {
		c.seen = map[string]watch.Kind{}
	}
	c.seen[path] |= k
}

func (c *changeLog) has(path string, k watch.Kind) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seen[path].Has(k)
}

func (c *changeLog) wait(t *testing.T, path string, k watch.Kind) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if c.has(path, k) {
			return
		}
	}
	t.Fatalf("no change reported in %s", path)
}

func TestOnlyTheRepositoryOnScreenIsWatched(t *testing.T) {
	m, _ := newManager(t)
	var changes changeLog
	m.OnChange(changes.add)
	back, front := gittest.New(t), gittest.New(t)
	back.Linear(2)
	front.Linear(2)
	b, err := m.Open(context.Background(), back.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, b, 2)
	f, err := m.Open(context.Background(), front.Dir)
	if err != nil {
		t.Fatal(err)
	}
	expectWatched(t, m, "after opening a second tab", f.Path)

	// A commit behind the tab on screen is neither reported nor loaded: one in front is
	// reported, and by then the first would have been too.
	back.CommitFile("behind.txt", "x", "made behind")
	front.CommitFile("front.txt", "x", "made in front")
	changes.wait(t, f.Path, watch.Refs)
	if changes.has(b.Path, watch.Refs) {
		t.Fatal("a repository that isn't on screen reported a change")
	}
	if g, _ := b.Graph(); g.Len() != 2 {
		t.Fatalf("a repository that isn't on screen reloaded its graph: %d rows", g.Len())
	}

	// Showing it brings it up to date, and watches it from then on.
	if err := m.SetActive(b.Path); err != nil {
		t.Fatal(err)
	}
	expectWatched(t, m, "after switching tabs", b.Path)
	waitGraph(t, b, 3)
	back.CommitFile("later.txt", "x", "made later")
	changes.wait(t, b.Path, watch.Refs)
	waitGraph(t, b, 4)
}

func TestShowingARepositoryThatHasntChangedKeepsItsGraph(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	fx.Linear(3)
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 3)
	before, _ := r.Graph()

	r.graph.refresh(context.Background())
	if after, loading := r.Graph(); after != before || loading {
		t.Fatalf("an unchanged repository was read again (loading %v)", loading)
	}

	// Two branches at one commit: the refs are where they were, but HEAD is on the other.
	fx.Git("branch", "twin")
	r.graph.refresh(context.Background())
	waitGraph(t, r, 3)
	withTwin, _ := r.Graph()
	if withTwin == before {
		t.Fatal("a new branch didn't reload the graph")
	}
	fx.Git("checkout", "-q", "twin")
	r.graph.refresh(context.Background())
	waitGraph(t, r, 3)
	if onTwin, _ := r.Graph(); onTwin == withTwin {
		t.Fatal("switching to a branch at the same commit didn't reload the graph")
	}
}

func TestTabsThatShowRepositoriesWatchThem(t *testing.T) {
	m, store := newManager(t)
	one, two := gittest.New(t), gittest.New(t)
	one.Linear(1)
	two.Linear(1)
	a, err := m.Open(context.Background(), one.Dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Open(context.Background(), two.Dir)
	if err != nil {
		t.Fatal(err)
	}
	type shown struct {
		path string
		late bool
	}
	var mu sync.Mutex
	var log []shown
	m.OnShown(func(path string, late bool) {
		mu.Lock()
		defer mu.Unlock()
		log = append(log, shown{path, late})
	})
	took := func() []shown {
		mu.Lock()
		defer mu.Unlock()
		out := log
		log = nil
		return out
	}
	set := func(change func(s *settings.Settings), active string) {
		t.Helper()
		if err := store.Update(change); err != nil {
			t.Fatal(err)
		}
		if err := m.SetActive(active); err != nil {
			t.Fatal(err)
		}
	}

	set(func(s *settings.Settings) { s.IgnoreTab = a.Path }, settings.IgnoreTabID)
	expectWatched(t, m, "the Ignore rules tab", a.Path)
	if got := took(); !slices.Equal(got, []shown{{a.Path, false}}) {
		t.Fatalf("shown = %v", got)
	}

	set(func(s *settings.Settings) { s.InsightsTab = "repo:" + b.Path }, settings.InsightsTabID)
	expectWatched(t, m, "Insights for one repository", b.Path)
	took()

	// A group's repositories that are open. The one already watched has its watcher
	// restarted, so it may have missed something.
	set(func(s *settings.Settings) {
		s.Groups = []settings.Group{{Name: "both", Paths: []string{b.Path, a.Path, t.TempDir()}}}
		s.InsightsTab = "group:both"
	}, settings.InsightsTabID)
	expectWatched(t, m, "Insights for a group", b.Path, a.Path)
	if got := took(); !slices.Equal(got, []shown{{b.Path, true}, {a.Path, false}}) {
		t.Fatalf("shown = %v", got)
	}

	set(func(*settings.Settings) {}, settings.ConfigTabID)
	expectWatched(t, m, "the Git Config tab")
	set(func(*settings.Settings) {}, a.Path)
	expectWatched(t, m, "a repository's tab", a.Path)
	set(func(*settings.Settings) {}, "")
	expectWatched(t, m, "the start page")
}

func TestClosingTheTabOnScreenWatchesTheNextOne(t *testing.T) {
	m, _ := newManager(t)
	one, two := gittest.New(t), gittest.New(t)
	one.Linear(1)
	two.Linear(1)
	a, _ := m.Open(context.Background(), one.Dir)
	b, _ := m.Open(context.Background(), two.Dir)
	if err := m.Close(b.Path); err != nil {
		t.Fatal(err)
	}
	expectWatched(t, m, "after closing the tab on screen", a.Path)
	if err := m.Close(a.Path); err != nil {
		t.Fatal(err)
	}
	expectWatched(t, m, "after closing the last tab")
}

func TestRestoreLoadsTheTabOnScreenFirst(t *testing.T) {
	m, _ := newManager(t)
	var paths []string
	for range 3 {
		fx := gittest.New(t)
		fx.Linear(2)
		r, err := m.Open(context.Background(), fx.Dir)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, r.Path)
	}
	if err := m.SetActive(paths[1]); err != nil {
		t.Fatal(err)
	}
	m.CloseAll()

	var mu sync.Mutex
	var loaded []string
	m.OnGraph(func(path string, p GraphProgress) {
		if p.Done {
			mu.Lock()
			loaded = append(loaded, path)
			mu.Unlock()
		}
	})
	active, failed := m.Restore(context.Background())
	if active != paths[1] || len(failed) != 0 {
		t.Fatalf("restore: active %s, failed %v", active, failed)
	}
	expectWatched(t, m, "after restoring", paths[1])
	want := []string{paths[1], paths[0], paths[2]}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		mu.Lock()
		n := len(loaded)
		mu.Unlock()
		if n >= len(want) {
			break
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(loaded, want) {
		t.Fatalf("graphs loaded in the order %v, want %v", loaded, want)
	}
}

func TestATabBehindLoadsWhenAskedBeforeItsTurn(t *testing.T) {
	m, _ := newManager(t)
	one, two := gittest.New(t), gittest.New(t)
	one.Linear(2)
	two.Linear(2)
	a, _ := m.Open(context.Background(), one.Dir)
	if _, err := m.Open(context.Background(), two.Dir); err != nil {
		t.Fatal(err)
	}
	m.CloseAll()
	// Restore without the warm-up, as if it hadn't reached this tab yet.
	r := m.add(a.Path, a.GitDir, a.CommonDir)
	if g, _ := r.graph.current, false; g != nil {
		t.Fatal("a tab that hasn't been asked for has loaded")
	}
	waitGraph(t, r, 2)
}

// A watcher can take a while to start: one watch for every folder of a big tree on Linux, or
// a busy machine. The switch doesn't wait for it past a moment, and whoever is showing the
// repository is told to read it again once it's watched.
func TestAWatcherSlowToStartIsReportedLate(t *testing.T) {
	m, _ := newManager(t)
	one, two := gittest.New(t), gittest.New(t)
	one.Linear(1)
	two.Linear(1)
	a, err := m.Open(context.Background(), one.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Open(context.Background(), two.Dir); err != nil {
		t.Fatal(err)
	}

	type shown struct {
		path string
		late bool
	}
	reports := make(chan shown, 4)
	m.OnShown(func(path string, late bool) { reports <- shown{path, late} })
	started := make(chan struct{})
	m.wait = 10 * time.Millisecond
	m.startWatch = func(o watch.Options) (*watch.Watcher, error) {
		<-started
		return watch.New(o)
	}

	if err := m.SetActive(a.Path); err != nil {
		t.Fatal(err)
	}
	// Back already, with the watcher still on its way: the tab behind is no longer watched,
	// and the one coming on screen isn't yet.
	select {
	case r := <-reports:
		t.Fatalf("reported %+v before the watcher had started", r)
	default:
	}
	close(started)
	select {
	case r := <-reports:
		if r != (shown{a.Path, true}) {
			t.Fatalf("reported %+v, want %s shown late", r, a.Path)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("nothing said the repository was watched in the end")
	}
	expectWatched(t, m, "once the watcher started", a.Path)
}
