package app

import (
	"os"
	"testing"
	"time"
)

// #1061: on a large repository, how long a path focus takes to show its first rows, and how
// long the labels of branches whose tips changed nothing there take after them. Set
// GITREVENGE_BENCH_REPO to it, and GITREVENGE_BENCH_PATH to a folder in it (mm/ by default).
func TestPathLabelsOnTheBenchRepository(t *testing.T) {
	dir := os.Getenv("GITREVENGE_BENCH_REPO")
	if dir == "" {
		t.Skip("set GITREVENGE_BENCH_REPO to a large repository to time the labels under a path focus")
	}
	folder := os.Getenv("GITREVENGE_BENCH_PATH")
	if folder == "" {
		folder = "mm/"
	}
	a := newTestApp(t)
	tab, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.openRepo(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	// The whole history's first screen, so the path's load replaces a graph that's showing.
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if g, _ := r.Graph(); g != nil && g.Len() >= 100 {
			break
		}
	}
	before, _ := r.Graph()
	start := time.Now()
	if err := a.SetGraphPath(tab.Path, folder); err != nil {
		t.Fatal(err)
	}
	var rowsAt time.Duration
	for time.Since(start) < 5*time.Minute {
		if g, _ := r.Graph(); rowsAt == 0 && g != before && g.Len() >= 100 {
			rowsAt = time.Since(start)
		}
		if labels := r.PathLabels(); labels != nil && rowsAt > 0 {
			n := 0
			for _, list := range labels {
				n += len(list)
			}
			t.Logf("%s: first 100 rows in %v; %d branches labelled where they last touched it in %v (of %d refs)", folder, rowsAt.Round(time.Millisecond), n, time.Since(start).Round(time.Millisecond), len(r.Refs().All))
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the rows or the labels never arrived")
}
