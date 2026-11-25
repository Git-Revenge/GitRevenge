package app

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// unitsTab is an open repository where km/h comes and goes: three commits bring it in, one
// takes it out, and two never mention it. Two have a Changelog trailer of fixed.
func unitsTab(t *testing.T, a *App) (*gittest.Repo, string) {
	t.Helper()
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "feed.js", "Wind in km/h", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "map.js", "Pins say km/h", "Changelog: added")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "readme.md", "A readme", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "feed.js", "Wind in knots")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "notes.md", "Notes in km/h")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "other.md", "Nothing here")
	return fx, openForMap(t, a, fx)
}

// historyDone waits for the repository's history search to stop running, and returns its
// filter as it then stands.
func historyDone(t *testing.T, a *App, path string) TrailerFilterState {
	t.Helper()
	var st TrailerFilterState
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var err error
		if st, err = a.TrailerFilter(path); err != nil {
			t.Fatal(err)
		}
		if st.History != nil && !st.History.Running {
			return st
		}
	}
	t.Fatalf("the history search never ended: %+v", st.History)
	return st
}

func TestHistorySearchFiltersTheGraphToWhatItFinds(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	var mu sync.Mutex
	progress := 0
	a.emitHook = func(name string, data any) {
		if p, ok := data.(HistoryProgress); ok && name == "history:progress" && p.Filter.History != nil {
			mu.Lock()
			progress++
			mu.Unlock()
		}
	}
	_, path := unitsTab(t, a)
	st, err := a.SetHistorySearch(path, git.HistoryOptions{Text: "km/h"})
	if err != nil || st.History == nil || !st.Only {
		t.Fatalf("started = %+v, %v", st, err)
	}
	st = historyDone(t, a, path)
	h := st.History
	// Three brought km/h in and one took it out; the other two never had it.
	if h.Found != 4 || h.Searched != 6 || h.Total != 6 || h.Stopped || h.Problem != "" || h.BackTo == 0 || st.Kept != 4 {
		t.Fatalf("done = %+v, kept %d", h, st.Kept)
	}
	graphRows(t, a, path, 4)
	mu.Lock()
	defer mu.Unlock()
	if progress == 0 {
		t.Fatal("the window wasn't told")
	}
}

func TestHistorySearchAndATrailerMustBothHold(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	_, path := unitsTab(t, a)
	trailersRead(t, a, path)
	fixed := []trailerindex.Pick{{Key: "changelog", Value: "text:fixed"}}
	if _, err := a.SetTrailerFilter(path, fixed, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetHistorySearch(path, git.HistoryOptions{Text: "km/h"}); err != nil {
		t.Fatal(err)
	}
	st := historyDone(t, a, path)
	// Fixed and km/h: only the feed's first commit. The readme is fixed without km/h.
	if st.Kept != 1 || len(st.Picks) != 1 || st.History.Found != 4 {
		t.Fatalf("both = kept %d, picks %v, found %d", st.Kept, st.Picks, st.History.Found)
	}
	graphRows(t, a, path, 1)
	// Taking the pick out leaves the search; Highlight keeps everything.
	if st, _ = a.SetTrailerFilter(path, nil, false); st.History == nil || st.Kept != 4 || st.Only {
		t.Fatalf("without the pick = %+v", st)
	}
	graphRows(t, a, path, 6)
	// Clearing the search, with no picks, clears the filter.
	if st, _ = a.SetHistorySearch(path, git.HistoryOptions{}); st.History != nil || len(st.Picks) != 0 {
		t.Fatalf("cleared = %+v", st)
	}
}

func TestHistorySearchStopsAndCarriesOn(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	// One commit a git: long enough to stop before it's done.
	a.historyChunkSize = 1
	_, path := unitsTab(t, a)
	if _, err := a.SetHistorySearch(path, git.HistoryOptions{Text: "km/h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StopHistorySearch(path); err != nil {
		t.Fatal(err)
	}
	st := historyDone(t, a, path)
	if !st.History.Stopped || st.History.Searched >= 6 {
		t.Fatalf("stopped = %+v", st.History)
	}
	if _, err := a.CarryOnHistorySearch(path); err != nil {
		t.Fatal(err)
	}
	st = historyDone(t, a, path)
	if st.History.Stopped || st.History.Searched != 6 || st.History.Found != 4 {
		t.Fatalf("carried on = %+v", st.History)
	}
}

func TestChunksStartSmallAndDouble(t *testing.T) {
	t.Parallel()
	var got []int
	for n := range 7 {
		got = append(got, chunkSize(n, 250, 2000))
	}
	if want := []int{250, 500, 1000, 2000, 2000, 2000, 2000}; !slices.Equal(got, want) {
		t.Fatalf("sizes = %v", got)
	}
	if chunkSize(70, 250, 2000) != 2000 || chunkSize(3, 1, 1) != 1 {
		t.Fatal("past the shift, or with one commit a chunk")
	}
}

func TestHistoryChangesSaysWhatEachCommitDid(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path := unitsTab(t, a)
	if _, err := a.SetHistorySearch(path, git.HistoryOptions{Text: "km/h"}); err != nil {
		t.Fatal(err)
	}
	historyDone(t, a, path)
	all := strings.Fields(fx.Git("rev-list", "HEAD"))
	got, err := a.HistoryChanges(path, all)
	if err != nil {
		t.Fatal(err)
	}
	// Newest first: other.md, notes.md, knots, readme, map, feed.
	want := map[string][]git.HistoryChange{
		all[1]: {{File: "notes.md", Added: 1}},
		all[2]: {{File: "feed.js", Removed: 1}},
		all[4]: {{File: "map.js", Added: 1}},
		all[5]: {{File: "feed.js", Added: 1}},
	}
	if len(got) != len(want) {
		t.Fatalf("changes = %v", got)
	}
	for id, w := range want {
		if c := got[id]; len(c) != 1 || c[0] != w[0] {
			t.Errorf("%s = %v, want %v", id[:7], c, w)
		}
	}
	// Asked again, it answers from what it worked out.
	if again, _ := a.HistoryChanges(path, all[:2]); len(again) != 1 {
		t.Fatalf("again = %v", again)
	}
}

// The graph asks about its rows and the commit panel about its commit at the same moment:
// neither cuts the other short, which would read as commits that changed nothing (#982).
func TestHistoryChangesAnswersEveryCaller(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path := unitsTab(t, a)
	if _, err := a.SetHistorySearch(path, git.HistoryOptions{Text: "km/h"}); err != nil {
		t.Fatal(err)
	}
	historyDone(t, a, path)
	all := strings.Fields(fx.Git("rev-list", "HEAD"))
	asks := [][]string{{all[1], all[2]}, {all[4], all[5]}}
	got := make([]map[string][]git.HistoryChange, len(asks))
	var wg sync.WaitGroup
	for i, ids := range asks {
		wg.Go(func() {
			var err error
			if got[i], err = a.HistoryChanges(path, ids); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	for i := range asks {
		if len(got[i]) != 2 {
			t.Errorf("ask %d = %v", i, got[i])
		}
	}
}

func TestHistorySearchSaysWhyItEndedEarly(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	_, path := unitsTab(t, a)
	if _, err := a.SetHistorySearch(path, git.HistoryOptions{Text: "km(", Regex: true}); err != nil {
		t.Fatal(err)
	}
	if st := historyDone(t, a, path); st.History.Problem != "Not a regular expression: its ( and ) don't pair up." {
		t.Fatalf("problem = %q", st.History.Problem)
	}
	if _, err := a.SetHistorySearch(path, git.HistoryOptions{Text: "two\nlines"}); err == nil {
		t.Fatal("searched for two lines")
	}

	// A partial clone: the search stops where files were never downloaded, and says so.
	src := gittest.New(t)
	src.CommitFile("old.txt", "km/h once\n", "one")
	src.CommitFile("old.txt", "knots now\n", "two")
	src.Git("config", "uploadpack.allowFilter", "true")
	dest := filepath.Join(t.TempDir(), "partial")
	src.Git("clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(src.Dir), dest)
	tab, err := a.OpenRepo(dest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetHistorySearch(tab.Path, git.HistoryOptions{Text: "km/h"}); err != nil {
		t.Fatal(err)
	}
	if st := historyDone(t, a, tab.Path); !strings.HasPrefix(st.History.Problem, "This is a partial clone") {
		t.Fatalf("partial clone = %+v", st.History)
	}
}
