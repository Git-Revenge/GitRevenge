// Package perf measures Git Revenge against a large real repository. It only runs when
// GITREVENGE_BENCH_REPO points at one, e.g.:
//
//	GITREVENGE_BENCH_REPO=/src/linux go test ./internal/perf -v -count=1 -timeout 30m
//
// The targets come from docs/PLAN.md. Run before tagging each milestone.
package perf

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

const (
	firstPageRows    = 100
	targetFirstPage  = time.Second
	targetHeapBytes  = 500 << 20
	windowReads      = 1000
	targetWindowRead = 16 * time.Millisecond // one frame at 60fps
	targetStatus     = 500 * time.Millisecond
)

func benchRepo(t *testing.T) *git.Repo {
	t.Helper()
	dir := os.Getenv("GITREVENGE_BENCH_REPO")
	if dir == "" {
		t.Skip("set GITREVENGE_BENCH_REPO to a large repository to run performance checks")
	}
	bin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	r := git.OpenRepo(git.NewRunner(bin), dir)
	gitDir, err := r.ReadString(context.Background(), "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatalf("%s is not a git repository: %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "objects", "info", "commit-graph")); err != nil {
		if _, err := os.Stat(filepath.Join(gitDir, "objects", "info", "commit-graphs")); err != nil {
			t.Logf("WARNING: no commit-graph file; git must walk all history before the first commit. " +
				"Run `git commit-graph write --reachable` for representative numbers.")
		}
	}
	return r
}

var errFirstPage = errors.New("first page reached")

// newGraph returns an empty graph with the repository's Trunk lanes pinned under the
// default branch kinds, as the app lays it out (ADR 0009).
func newGraph(ctx context.Context, r *git.Repo) *graph.Graph {
	g := graph.New()
	var remotes []string
	if rs, err := r.Remotes(ctx); err == nil {
		for _, rm := range rs {
			remotes = append(remotes, rm.Name)
		}
	}
	g.Pin(session.TrunkTips(ctx, r, branchkind.New(nil, nil), remotes, false))
	return g
}

func TestTargets(t *testing.T) {
	r := benchRepo(t)
	ctx := context.Background()

	// Time to first screen: from starting git log until enough rows are laid out to draw.
	start := time.Now()
	g := newGraph(ctx, r)
	err := r.Log(ctx, git.LogOptions{}, func(c git.LogCommit) error {
		if err := g.Add(c); err != nil {
			return err
		}
		if g.Len() == firstPageRows {
			return errFirstPage
		}
		return nil
	})
	if err != nil && !errors.Is(err, errFirstPage) {
		t.Fatal(err)
	}
	firstPage := time.Since(start)

	// Full load.
	start = time.Now()
	g = newGraph(ctx, r)
	if err := r.Log(ctx, git.LogOptions{}, g.Add); err != nil {
		t.Fatal(err)
	}
	fullLoad := time.Since(start)

	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// Random window reads, as when scrolling or jumping around.
	n := g.Len()
	rnd := rand.New(rand.NewPCG(1, 1))
	start = time.Now()
	var worst time.Duration
	for range windowReads {
		s := time.Now()
		g.Rows(rnd.IntN(max(1, n)), firstPageRows)
		worst = max(worst, time.Since(s))
	}
	avgWindow := time.Since(start) / windowReads

	start = time.Now()
	res := g.Search("fix", 10000)
	search := time.Since(start)

	start = time.Now()
	refs, err := r.Refs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	refsTime := time.Since(start)

	// Status, cold then warm: the second run has the index and file system caches.
	start = time.Now()
	if _, err := r.Status(ctx); err != nil {
		t.Fatal(err)
	}
	statusCold := time.Since(start)
	start = time.Now()
	st, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	statusWarm := time.Since(start)

	t.Logf("repository:           %s", os.Getenv("GITREVENGE_BENCH_REPO"))
	t.Logf("commits:              %d", n)
	t.Logf("refs:                 %d (read in %v)", len(refs), refsTime.Round(time.Millisecond))
	t.Logf("first %d rows:       %v (target < %v)", firstPageRows, firstPage.Round(time.Millisecond), targetFirstPage)
	t.Logf("full load + layout:   %v", fullLoad.Round(time.Millisecond))
	t.Logf("heap in use:          %d MB (target < %d MB)", ms.HeapInuse>>20, targetHeapBytes>>20)
	t.Logf("100-row window:       avg %v, worst %v (target < %v)", avgWindow.Round(time.Microsecond), worst.Round(time.Microsecond), targetWindowRead)
	t.Logf("search \"fix\":         %v (%d matches)", search.Round(time.Millisecond), res.Total)
	t.Logf("status:               cold %v, warm %v (target < %v), %d changed files", statusCold.Round(time.Millisecond), statusWarm.Round(time.Millisecond), targetStatus, len(st.Files))

	if firstPage > targetFirstPage {
		t.Errorf("first page took %v, target %v", firstPage, targetFirstPage)
	}
	if ms.HeapInuse > targetHeapBytes {
		t.Errorf("heap in use %d MB, target %d MB", ms.HeapInuse>>20, targetHeapBytes>>20)
	}
	if worst > targetWindowRead {
		t.Errorf("worst window read %v, target %v", worst, targetWindowRead)
	}
	if statusWarm > targetStatus {
		t.Errorf("warm status took %v, target %v", statusWarm, targetStatus)
	}
}

// TestReadAhead measures the app's own loader, which pauses DefaultReadAhead rows ahead of
// the viewport (#163): time and heap at the first pause, the scrollbar estimate, and how
// long jumping to the oldest commit takes when it isn't loaded yet.
func TestReadAhead(t *testing.T) {
	r := benchRepo(t)
	dir := os.Getenv("GITREVENGE_BENCH_REPO")
	ctx := context.Background()
	bin, _ := exec.LookPath("git")
	store, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := session.NewManager(git.NewRunner(bin), store)
	defer m.CloseAll()

	type event struct {
		p  session.GraphProgress
		at time.Duration
	}
	events := make(chan event, 1024)
	start := time.Now()
	m.OnGraph(func(_ string, p session.GraphProgress) {
		select {
		case events <- event{p, time.Since(start)}:
		default:
		}
	})
	repo, err := m.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	var firstPause, estimateAt time.Duration
	estimate := 0
	deadline := time.After(5 * time.Minute)
	for firstPause == 0 || estimate == 0 {
		select {
		case e := <-events:
			if e.p.Estimate > 0 && estimate == 0 {
				estimate, estimateAt = e.p.Estimate, e.at
			}
			if e.p.Paused && firstPause == 0 {
				firstPause = e.at
			}
			if e.p.Done {
				t.Skipf("history has only %d commits, under the read-ahead of %d", e.p.Rows, session.DefaultReadAhead)
			}
		case <-deadline:
			t.Fatal("the loader never paused")
		}
	}
	g, _ := repo.Graph()
	loaded := g.Len()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	roots, err := r.ReadString(ctx, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	// The last root in rev-list's order is the oldest one.
	fields := strings.Fields(roots)
	oldest := fields[len(fields)-1]
	start = time.Now()
	row, ok, err := repo.FindRow(ctx, oldest)
	if err != nil || !ok {
		t.Fatalf("FindRow(oldest root) = %d, %v, %v", row, ok, err)
	}
	find := time.Since(start)

	t.Logf("read-ahead:           %d rows", session.DefaultReadAhead)
	t.Logf("first pause:          %v, %d rows loaded", firstPause.Round(time.Millisecond), loaded)
	t.Logf("heap at first pause:  %d MB", ms.HeapInuse>>20)
	t.Logf("estimate:             %d commits (known after %v)", estimate, estimateAt.Round(time.Millisecond))
	t.Logf("jump to oldest root:  %v (row %d)", find.Round(time.Millisecond), row)

	if ms.HeapInuse > targetHeapBytes {
		t.Errorf("heap at first pause %d MB, target %d MB", ms.HeapInuse>>20, targetHeapBytes>>20)
	}
}

func BenchmarkFullLoad(b *testing.B) {
	r := benchRepo(&testing.T{})
	for b.Loop() {
		g := newGraph(context.Background(), r)
		if err := r.Log(context.Background(), git.LogOptions{}, g.Add); err != nil {
			b.Fatal(err)
		}
	}
}
