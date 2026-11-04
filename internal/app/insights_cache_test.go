package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestCachedInsightFollowsRefs(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(2)
	a := newTestApp(t)
	ctx := context.Background()
	g, root, err := a.repoGit(ctx, fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	get := func(in InsightsInputs) int {
		t.Helper()
		v, err := cachedInsight(ctx, a, g, root, "test", in, func() (int, error) {
			calls++
			return calls, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	in := InsightsInputs{Window: "7d"}
	first, second := get(in), get(in)
	if first != 1 || second != 1 {
		t.Fatalf("same inputs and refs should reuse the result; calls = %d", calls)
	}
	if get(InsightsInputs{Window: "14d"}) != 2 {
		t.Errorf("other inputs should compute; calls = %d", calls)
	}
	fx.CommitFile("x.txt", "x", "Another")
	if get(in) != 3 {
		t.Errorf("a new commit should compute again; calls = %d", calls)
	}
	fx.Git("branch", "side")
	if get(in) != 4 {
		t.Errorf("a new branch should compute again; calls = %d", calls)
	}
}

func TestInsightsReuseConflictChecks(t *testing.T) {
	origin := gittest.New(t)
	origin.CommitFile("a.txt", "base\n", "Start")
	origin.Branch("side")
	origin.CommitFile("a.txt", "theirs\n", "Theirs")
	origin.Checkout("main")
	origin.CommitFile("a.txt", "ours\n", "Ours")
	fx := gittest.Clone(t, origin)

	a := newTestApp(t)
	var mu sync.Mutex
	var events []ConflictResult
	a.emitHook = func(_ string, data any) {
		if c, ok := data.(ConflictResult); ok {
			mu.Lock()
			events = append(events, c)
			mu.Unlock()
		}
	}
	wait := func(n int) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			got := len(events)
			mu.Unlock()
			if got >= n {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("waited for %d conflict events", n)
	}

	first, err := a.InsightsBranches(fx.Dir, InsightsInputs{})
	if err != nil {
		t.Fatal(err)
	}
	wait(1)
	if _, ok := a.insights.conflict(first.Window.TargetSHA, first.Branches[0].SHA); !ok {
		t.Fatal("the check wasn't cached")
	}
	// Asking again reports the cached result without another merge-tree run.
	a.insights.putConflict(first.Window.TargetSHA, first.Branches[0].SHA, git.Conflict{Conflicts: true, Files: []string{"from-cache"}})
	if _, err := a.InsightsBranches(fx.Dir, InsightsInputs{}); err != nil {
		t.Fatal(err)
	}
	wait(2)
	mu.Lock()
	defer mu.Unlock()
	if len(events[1].Files) != 1 || events[1].Files[0] != "from-cache" {
		t.Errorf("second report = %+v, want the cached result", events[1])
	}
}

func TestInsightsFetchedAt(t *testing.T) {
	origin := gittest.New(t)
	origin.Linear(1)
	fx := gittest.Clone(t, origin)
	a := newTestApp(t)
	// A fresh clone has fetched but may not have written FETCH_HEAD yet.
	fx.Git("fetch", "-q", "origin")
	at, err := a.InsightsFetchedAt(fx.Dir)
	if err != nil || at == 0 {
		t.Errorf("after a fetch: %d, %v", at, err)
	}
	fresh := gittest.New(t)
	fresh.Linear(1)
	if at, err := a.InsightsFetchedAt(fresh.Dir); err != nil || at != 0 {
		t.Errorf("never fetched: %d, %v", at, err)
	}
}
