package app

import (
	"context"
	"strconv"
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

// #1062: under a path focus a filter narrows the path's history further, as under a branch
// focus, and counts what it keeps there: it never says more than the history's total.
func TestTrailerFilterUnderAPathFocusCountsWhatItKeepsThere(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "src/a.txt", "One", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "docs/b.txt", "Two", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "docs/c.txt", "Three", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "src/d.txt", "Four")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 4)
	if err := a.SetGraphPath(path, "src/"); err != nil {
		t.Fatal(err)
	}
	graphRows(t, a, path, 2)
	fixed := []trailerindex.Pick{{Key: "changelog", Value: "text:fixed"}}

	// Highlight: both of src/'s commits stay, and one is fixed.
	state, err := a.SetTrailerFilter(path, fixed, false)
	if err != nil {
		t.Fatal(err)
	}
	if state.Kept != 1 || state.Total != 2 {
		t.Errorf("highlighting: %d of %d, want 1 of 2", state.Kept, state.Total)
	}
	// Only these: that one is the graph.
	if _, err := a.SetTrailerFilter(path, fixed, true); err != nil {
		t.Fatal(err)
	}
	graphRows(t, a, path, 1)
	if state, _ = a.TrailerFilter(path); state.Kept != 1 || state.Total != 2 {
		t.Errorf("only these: %d of %d, want 1 of 2", state.Kept, state.Total)
	}
	page, _ := a.GraphRows(path, 0, 10)
	if len(page.Rows) != 1 || page.Rows[0].Subject != "One" {
		t.Errorf("rows = %+v, want One", page.Rows)
	}
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

// A key is held back when nearly every commit has it. Under Only these the graph holds only
// what the filter keeps, and the history is as long as it was: the key a filter is on, on
// every commit the graph now shows, is still a key on some of the history (#880).
func TestAFilterDoesNotMakeItsKeyLookLikeItIsOnEveryCommit(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(15)
	for i := range 55 {
		fx.Git("commit", "-q", "--allow-empty", "-m", "Fix "+strconv.Itoa(i), "-m", "Changelog: fixed")
	}
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 70)
	offered := func() bool {
		t.Helper()
		panel, err := a.TrailerPanel(path, "", 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range panel.Keys {
			if k.Key.ID == "changelog" {
				return true
			}
		}
		return false
	}
	if !offered() {
		t.Fatal("with no filter, the panel doesn't offer Changelog, which is on 55 of 70 commits")
	}

	fixed := []trailerindex.Pick{{Key: "changelog", Value: "text:fixed"}}
	if state, err := a.SetTrailerFilter(path, fixed, true); err != nil || state.Kept != 55 {
		t.Fatalf("the filter = %+v, %v, want 55 kept", state, err)
	}
	graphRows(t, a, path, 55)
	// A reading is looked at again once it's a moment old.
	time.Sleep(rulesGoodFor + 300*time.Millisecond)
	if !offered() {
		t.Fatal("under Only these, the panel holds Changelog back: its 55 commits were measured against the 55 the graph shows, not the 70 the history has")
	}
	if state, err := a.TrailerFilter(path); err != nil || state.Kept != 55 || state.Total != 70 {
		t.Fatalf("the filter afterwards = %+v, %v, want 55 of 70", state, err)
	}
}

// Where a commit is in the graph, and why it isn't there when it isn't: kept out by a filter
// that narrows the graph, or not in the history at all. The graph's view keeps a selection
// the filter hid, and gives up one that's gone (#881).
func TestCommitRowSaysWhenAFilterKeepsTheCommitOut(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "One", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two", "Changelog: added")
	added := fx.Head()
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Three", "Changelog: fixed")
	fixed := fx.Head()
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 3)
	nowhere := "0123456789abcdef0123456789abcdef01234567"
	row := func(id string) int {
		t.Helper()
		got, err := a.CommitRow(path, id)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	if got := row(added); got != 1 {
		t.Fatalf("with no filter, the commit is at row %d, want 1", got)
	}
	if got := row(nowhere); got != rowAbsent {
		t.Fatalf("with no filter, a commit that isn't in the history = %d, want %d", got, rowAbsent)
	}

	pick := []trailerindex.Pick{{Key: "changelog", Value: "text:fixed"}}
	if _, err := a.SetTrailerFilter(path, pick, true); err != nil {
		t.Fatal(err)
	}
	graphRows(t, a, path, 2)
	if got := row(fixed); got != 0 {
		t.Fatalf("under Only these, a commit the filter keeps is at row %d, want 0", got)
	}
	if got := row(added); got != rowFilteredOut {
		t.Fatalf("under Only these, a commit the filter keeps out = %d, want %d", got, rowFilteredOut)
	}

	// Highlight takes nothing out: every commit has its row again.
	if _, err := a.SetTrailerFilter(path, pick, false); err != nil {
		t.Fatal(err)
	}
	graphRows(t, a, path, 3)
	if got := row(added); got != 1 {
		t.Fatalf("under Highlight, the commit is at row %d, want 1", got)
	}
	if got := row(nowhere); got != rowAbsent {
		t.Fatalf("under Highlight, a commit that isn't in the history = %d, want %d", got, rowAbsent)
	}
}

// The strip's buttons step between the commits a highlighting filter keeps (#866): every one,
// in order, going round at the ends as a search does.
func TestTrailerFilterStepGoesThroughEveryMatch(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "One", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two", "Changelog: added")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Three", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "d.txt", "Four")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 4)
	step := func(from, dir int) int {
		t.Helper()
		row, err := a.TrailerFilterStep(path, from, dir)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	if got := step(-1, 1); got != -1 {
		t.Fatalf("with no filter, the step is to row %d, want -1", got)
	}
	// Newest first: Four is row 0, Three row 1, Two row 2, One row 3. The fixes are 1 and 3.
	fixed := []trailerindex.Pick{{Key: "changelog", Value: "text:fixed"}}
	if _, err := a.SetTrailerFilter(path, fixed, false); err != nil {
		t.Fatal(err)
	}
	graphRows(t, a, path, 4)
	for _, c := range []struct{ from, dir, want int }{
		{-1, 1, 1}, {0, 1, 1}, {1, 1, 3}, {2, 1, 3}, {3, 1, 1},
		{3, -1, 1}, {2, -1, 1}, {1, -1, 3}, {0, -1, 3},
	} {
		if got := step(c.from, c.dir); got != c.want {
			t.Errorf("from row %d, dir %d: row %d, want %d", c.from, c.dir, got, c.want)
		}
	}
}

// Stepping works the filter's rows out once for a reading and a graph, and again when either
// is another one: a new commit the filter keeps is stepped to (#899).
func TestTrailerFilterStepKeepsItsRows(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "One", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two", "Changelog: added")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 2)
	fixed := []trailerindex.Pick{{Key: "changelog", Value: "text:fixed"}}
	if _, err := a.SetTrailerFilter(path, fixed, false); err != nil {
		t.Fatal(err)
	}
	kept := func() []int {
		f := a.trailerFilters.get(path)
		a.trailerFilters.mu.Lock()
		defer a.trailerFilters.mu.Unlock()
		return f.rowsKept
	}
	if row, err := a.TrailerFilterStep(path, -1, 1); err != nil || row != 1 {
		t.Fatalf("first step: row %d, %v; want 1", row, err)
	}
	first := kept()
	if row, _ := a.TrailerFilterStep(path, 1, 1); row != 1 {
		t.Fatalf("second step: row %d, want 1 again", row)
	}
	if again := kept(); len(first) != 1 || len(again) != 1 || &again[0] != &first[0] {
		t.Fatalf("the rows were worked out again with nothing changed: %v then %v", first, again)
	}
	if res, err := a.TrailerFilterRows(path); err != nil || res.Total != 1 || len(res.Rows) != 1 || res.Rows[0] != 1 {
		t.Fatalf("ruler rows = %+v, %v", res, err)
	}

	// A new commit the filter keeps: once it's read, the next step goes to it.
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Three", "Changelog: fixed")
	deadline := time.Now().Add(20 * time.Second)
	for {
		row, err := a.TrailerFilterStep(path, -1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if row == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the new commit was never stepped to: row %d", row)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if row, _ := a.TrailerFilterStep(path, 0, 1); row != 2 {
		t.Fatalf("after the new commit, the next match is row %d, want 2", row)
	}
}

func TestStepRow(t *testing.T) {
	t.Parallel()
	if got := stepRow(nil, 3, 1); got != -1 {
		t.Errorf("no rows: %d", got)
	}
	rows := []int{2, 5, 9}
	for _, c := range []struct{ from, dir, want int }{
		{-1, 1, 2}, {2, 1, 5}, {3, 1, 5}, {9, 1, 2}, {10, 1, 2},
		{9, -1, 5}, {6, -1, 5}, {2, -1, 9}, {-1, -1, 9}, {0, -1, 9},
	} {
		if got := stepRow(rows, c.from, c.dir); got != c.want {
			t.Errorf("stepRow(%v, %d, %d) = %d, want %d", rows, c.from, c.dir, got, c.want)
		}
	}
}
