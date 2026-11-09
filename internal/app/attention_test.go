package app

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestSignalFlags(t *testing.T) {
	t.Parallel()
	asOf := 100 * day
	b := git.OpenBranch{Name: "x", LastCommit: asOf - 9*day - 1, FirstCommit: asOf - 20*day, Behind: 60, Added: 400, Removed: 101}
	s := signal(b, asOf, Thresholds{}.withDefaults())
	if s.StaleDays != 9 || s.AgeDays != 20 {
		t.Errorf("days = %d, %d", s.StaleDays, s.AgeDays)
	}
	if want := []string{flagStale, flagLongLived, flagDrifting, flagBigBatch}; !slices.Equal(s.Flags, want) {
		t.Errorf("flags = %v, want %v", s.Flags, want)
	}
	// Thresholds are exclusive, and a tip after the as-of point isn't negative days.
	s = signal(git.OpenBranch{LastCommit: asOf + day, FirstCommit: asOf - 14*day, Behind: 50, Added: 500}, asOf, Thresholds{}.withDefaults())
	if s.StaleDays != 0 || len(s.Flags) != 0 {
		t.Errorf("at the thresholds: %+v", s)
	}
	s = signal(b, asOf, Thresholds{Stale: 30, LongLived: 30, Behind: 100, Size: 1000})
	if len(s.Flags) != 0 {
		t.Errorf("custom thresholds: %v", s.Flags)
	}
}

func TestInsightsBranchesAndConflicts(t *testing.T) {
	t.Parallel()
	origin := gittest.New(t)
	origin.CommitFile("a.txt", "base\n", "Start")
	origin.Branch("conflicting")
	origin.Branch("clean")
	origin.Checkout("conflicting")
	origin.CommitFile("a.txt", "theirs\n", "Theirs")
	origin.Checkout("clean")
	origin.CommitFile("b.txt", "b\n", "Add b")
	origin.Checkout("main")
	origin.CommitFile("a.txt", "ours\n", "Ours")
	fx := gittest.Clone(t, origin)

	a := newTestApp(t)
	var mu sync.Mutex
	got := map[string]ConflictResult{}
	a.emitHook = func(name string, data any) {
		if c, ok := data.(ConflictResult); ok && name == "insights:conflict" {
			mu.Lock()
			got[c.Branch] = c
			mu.Unlock()
		}
	}
	res, err := a.InsightsBranches(fx.Dir, InsightsInputs{Window: "14d"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Window.Target != "origin/main" || len(res.Branches) != 2 || res.Thresholds.Stale != 7 {
		t.Fatalf("result = %+v", res)
	}

	deadline := time.Now().Add(20 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	c := got["origin/conflicting"]
	if !c.Conflicts || !slices.Equal(c.Files, []string{"a.txt"}) || c.TargetSHA != res.Window.TargetSHA {
		t.Errorf("conflicting = %+v", c)
	}
	if cl, ok := got["origin/clean"]; !ok || cl.Conflicts {
		t.Errorf("clean = %+v (received %v)", cl, ok)
	}
}

func TestInsightsHotspots(t *testing.T) {
	t.Parallel()
	origin := gittest.New(t)
	origin.CommitFile("a.txt", "1\n", "Start")
	origin.Branch("touches-a")
	origin.Checkout("main")
	origin.CommitFile("a.txt", "1\n2\n", "Grow a")
	origin.CommitFile("b.txt", "b\n", "Add b")
	origin.Commit("Revert \"Something\"")
	origin.Checkout("touches-a")
	origin.CommitFile("a.txt", "x\n", "Change a on a branch")
	origin.Checkout("main")
	fx := gittest.Clone(t, origin)

	res, err := newTestApp(t).InsightsHotspots(fx.Dir, InsightsInputs{Window: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range res.Files {
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"a.txt", "b.txt"}) || res.Files[0].OpenBranches != 1 || res.Files[1].OpenBranches != 0 {
		t.Errorf("files = %+v", res.Files)
	}
	if len(res.Rework) != 1 || !res.Rework[0].Revert {
		t.Errorf("rework = %+v", res.Rework)
	}
}

func TestInsightsThresholdsSavedPerScope(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	if got, err := a.InsightsThresholds("repo:x"); err != nil || got != (Thresholds{Stale: 7, LongLived: 14, Behind: 50, Size: 500}) {
		t.Fatalf("defaults = %+v, %v", got, err)
	}
	want := Thresholds{Stale: 3, LongLived: 10, Behind: 20, Size: 200}
	if err := a.SetInsightsThresholds("group:Harbour", want); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.InsightsThresholds("group:Harbour"); got != want {
		t.Errorf("saved = %+v, want %+v", got, want)
	}
	if got, _ := a.InsightsThresholds("repo:x"); got.Stale != 7 {
		t.Errorf("another scope changed: %+v", got)
	}
	if err := a.SetInsightsThresholds("repo:x", Thresholds{Stale: 0, LongLived: 1, Behind: 1, Size: 1}); err == nil {
		t.Error("a zero threshold was accepted")
	}
}
