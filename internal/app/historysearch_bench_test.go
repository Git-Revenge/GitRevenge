package app

import (
	"os"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// TestHistorySearchOnTheBenchRepository times the history search (#981) on a large
// repository, through the app as the filter strip runs it: the first commit found, and the
// whole search. It runs only when GITREVENGE_BENCH_REPO names one, e.g. the Linux kernel:
//
//	GITREVENGE_BENCH_REPO=/a/linux go test ./internal/app -run TestHistorySearchOnTheBench -v -timeout 40m
func TestHistorySearchOnTheBenchRepository(t *testing.T) {
	dir := os.Getenv("GITREVENGE_BENCH_REPO")
	if dir == "" {
		t.Skip("set GITREVENGE_BENCH_REPO to a large repository to time the history search")
	}
	a := newTestApp(t)
	tab, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	since2025 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	for _, c := range []struct {
		name string
		o    git.HistoryOptions
	}{
		{"since 2025", git.HistoryOptions{Text: "kmalloc_array(", Since: since2025}},
		{"in mm/", git.HistoryOptions{Text: "kmalloc_array(", Paths: []string{"mm/"}}},
		{"everything", git.HistoryOptions{Text: "kmalloc_array("}},
	} {
		start := time.Now()
		if _, err := a.SetHistorySearch(tab.Path, c.o); err != nil {
			t.Fatal(err)
		}
		var first time.Duration
		var st TrailerFilterState
		for {
			if st, err = a.TrailerFilter(tab.Path); err != nil {
				t.Fatal(err)
			}
			h := st.History
			if first == 0 && h.Found > 0 {
				first = time.Since(start)
			}
			if !h.Running {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		h := st.History
		t.Logf("%-11s first %6s  all %8s  found %4d in %7d commits, %d workers, estimate at the end %s, problem %q", c.name,
			first.Round(time.Millisecond), time.Since(start).Round(time.Second), h.Found, h.Searched, historyWorkers,
			(time.Duration(h.EstimateMs) * time.Millisecond).Round(time.Second), h.Problem)
	}
}

// TestHistorySearchGraphFollowsOnTheBenchRepository (#995) samples, every ten seconds for
// ninety, how many commits a whole-history search has found and how many the graph shows
// under Only these, then stops it. Before #995 a reload walked the whole history and the graph
// lagged the search by a whole reload; now it walks only as far as the search has got.
//
//	GITREVENGE_BENCH_REPO=/a/linux go test ./internal/app -run TestHistorySearchGraphFollows -v -timeout 10m
func TestHistorySearchGraphFollowsOnTheBenchRepository(t *testing.T) {
	dir := os.Getenv("GITREVENGE_BENCH_REPO")
	if dir == "" {
		t.Skip("set GITREVENGE_BENCH_REPO to a large repository to time the history search")
	}
	a := newTestApp(t)
	tab, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := a.SetHistorySearch(tab.Path, git.HistoryOptions{Text: "kmalloc_array("}); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = a.StopHistorySearch(tab.Path) }()
	for s := 10; s <= 90; s += 10 {
		time.Sleep(time.Until(start.Add(time.Duration(s) * time.Second)))
		st, err := a.TrailerFilter(tab.Path)
		if err != nil {
			t.Fatal(err)
		}
		page, _ := a.GraphRows(tab.Path, 0, 1)
		t.Logf("%3d s  found %5d  graph %5d  searched %8d  loading %v", s, st.History.Found, page.Total, st.History.Searched, page.Loading)
		if !st.History.Running {
			break
		}
	}
}
