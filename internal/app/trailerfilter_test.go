package app

import (
	"context"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// graphRows waits for the repository's graph to have n rows, loaded to the end.
func graphRows(t *testing.T, a *App, path string, n int) {
	t.Helper()
	var last GraphPage
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		page, err := a.GraphRows(path, 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		last = page
		if !page.Loading && page.Total == n {
			return
		}
	}
	t.Fatalf("the graph never had %d rows: %d, loading %v", n, last.Total, last.Loading)
}

// A filter shows only the commits its picks keep, or with Highlight keeps everything and
// says which rows match. Picks of one key are either-or, and of two keys both.
func TestTrailerFilterNarrowsTheGraph(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	sam, sarah := "Sam Ortiz <sam@tidewater.example>", "Sarah Lind <sarah@tidewater.example>"
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "One", "Changelog: fixed", "Reviewed-by: "+sarah)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two", "Changelog: added", "Reviewed-by: "+sarah)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Three", "Changelog: fixed", "Reviewed-by: "+sam)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "d.txt", "Four")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 4)
	fixed := trailerindex.Pick{Key: "changelog", Value: "text:fixed"}
	added := trailerindex.Pick{Key: "Changelog", Value: "text:added"}
	bySarah := trailerindex.Pick{Key: "reviewed-by", Value: "person:" + sarah}

	// Only these: the two fixed commits are the graph.
	state, err := a.SetTrailerFilter(path, []trailerindex.Pick{fixed}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Picks) != 1 || !state.Only || state.Kept != 2 {
		t.Fatalf("state = %+v, want one pick keeping two commits", state)
	}
	if p := state.Picks[0]; p.KeyLabel != "Changelog" || p.ValueLabel != "fixed" || p.Kind != string(trailerindex.KindLabel) && p.Kind != string(trailerindex.KindText) {
		t.Fatalf("the pick as shown = %+v", p)
	}
	graphRows(t, a, path, 2)
	page, _ := a.GraphRows(path, 0, 10)
	if page.Rows[0].Subject != "Three" || page.Rows[1].Subject != "One" {
		t.Fatalf("rows = %q, %q, want the fixed commits, newest first", page.Rows[0].Subject, page.Rows[1].Subject)
	}

	// Two values of one key: either. Another key as well: both.
	if state, _ = a.SetTrailerFilter(path, []trailerindex.Pick{fixed, added}, true); state.Kept != 3 {
		t.Fatalf("fixed or added keeps %d, want 3", state.Kept)
	}
	graphRows(t, a, path, 3)
	if state, _ = a.SetTrailerFilter(path, []trailerindex.Pick{fixed, added, bySarah}, true); state.Kept != 2 {
		t.Fatalf("fixed or added, reviewed by Sarah keeps %d, want 2", state.Kept)
	}
	graphRows(t, a, path, 2)

	// Highlight: nothing leaves, and the matching rows are named.
	state, err = a.SetTrailerFilter(path, []trailerindex.Pick{fixed}, false)
	if err != nil || state.Only || state.Kept != 2 {
		t.Fatalf("highlight state = %+v, %v", state, err)
	}
	graphRows(t, a, path, 4)
	rows, err := a.TrailerFilterRows(path)
	if err != nil || rows.Total != 2 || len(rows.Rows) != 2 || rows.Rows[0] != 1 || rows.Rows[1] != 3 {
		t.Fatalf("matching rows = %+v, %v, want rows 1 and 3", rows, err)
	}
	if got, _ := a.TrailerFilter(path); len(got.Picks) != 1 || got.Total != 4 {
		t.Fatalf("the filter as it stands = %+v, want one pick of 4 commits", got)
	}

	// A new commit that matches joins the filtered graph once its trailers are read.
	if _, err := a.SetTrailerFilter(path, []trailerindex.Pick{fixed}, true); err != nil {
		t.Fatal(err)
	}
	graphRows(t, a, path, 2)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "e.txt", "Five", "Changelog: fixed")
	graphRows(t, a, path, 3)

	// A value nobody has is dropped, and no picks clears the filter.
	state, err = a.SetTrailerFilter(path, []trailerindex.Pick{{Key: "changelog", Value: "text:nothing"}}, true)
	if err != nil || len(state.Picks) != 0 {
		t.Fatalf("a pick of nothing = %+v, %v", state, err)
	}
	graphRows(t, a, path, 5)
	if rows, _ := a.TrailerFilterRows(path); rows.Total != 0 || rows.Rows == nil {
		t.Fatalf("rows with no filter = %+v", rows)
	}
}

// A repository closes while its filter is on and a read of its trailers is ending. The end
// of a read works the filter out again, and it must not wait for what closing holds.
func TestClosingWithAFilterOnDoesNotHang(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "One", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two", "Changelog: added")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Three", "Changelog: fixed")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 3)
	fixed := []trailerindex.Pick{{Key: "changelog", Value: "text:fixed"}}
	if _, err := a.SetTrailerFilter(path, fixed, true); err != nil {
		t.Fatal(err)
	}
	graphRows(t, a, path, 2)
	// Each change of mode loads the graph again, and each load has the trailers read again:
	// the app shuts down with those reads on their way, some of them just ending.
	for i := range 8 {
		if _, err := a.SetTrailerFilter(path, fixed, i%2 == 1); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(i) * 5 * time.Millisecond)
	}
	closed := make(chan struct{})
	go func() {
		a.Shutdown(context.Background())
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(30 * time.Second):
		// The test's cleanup closes the app too and would hang behind this one: stop here.
		panic("closing hung: the end of a trailer read is waiting for the lock that closing holds")
	}
}
