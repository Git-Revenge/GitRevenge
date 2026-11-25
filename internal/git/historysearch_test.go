package git

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// unitsHistory is the demo's story in small, oldest first: km/h comes into the feed and
// the map, a readme says nothing of it, knots take the feed's out, a table with km/h in it
// is written and then only realigned, and an SQL comment with km/h in it comes and goes.
type unitsHistory struct {
	fx                                                      *gittest.Repo
	repo                                                    *Repo
	feed, pin, readme, knots, explain, realign, query, drop string
}

func newUnitsHistory(t *testing.T) unitsHistory {
	t.Helper()
	fx := gittest.New(t)
	h := unitsHistory{fx: fx}
	h.feed = fx.CommitFile("src/feed.js", "// Wind in km/h.\nexport const x = 1\n", "Move the feed client")
	h.pin = fx.CommitFile("public/map.js", "pin.title = `${s.wind} km/h`\n", "Plot each station as a pin")
	h.readme = fx.CommitFile("README.md", "Tidewater\n", "Add a readme")
	h.knots = fx.CommitFile("src/feed.js", "// Wind in knots.\nexport const x = 1\n", "Show wind in knots")
	h.explain = fx.CommitFile("docs/checks.md", "| Wind (km/h) |\n", "Explain the checks")
	h.realign = fx.CommitFile("docs/checks.md", "| Wind  (km/h) |\n", "Realign the table")
	h.query = fx.CommitFile("db/notes.sql", "-- km/h note\nselect 1;\n", "Add a query")
	h.drop = fx.CommitFile("db/notes.sql", "select 1;\n", "Drop the note")
	h.repo = OpenRepo(newTestRunner(t), fx.Dir)
	return h
}

// all lists every commit, newest first, as the app feeds them.
func (h unitsHistory) all(t *testing.T, since int64) []HistoryCommit {
	t.Helper()
	var got []HistoryCommit
	if err := h.repo.HistoryCommits(context.Background(), LogOptions{}, since, func(c HistoryCommit) error {
		got = append(got, c)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func ids(cs []HistoryCommit) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

func TestHistoryCommitsListsWhatTheGraphShowsNewestFirst(t *testing.T) {
	t.Parallel()
	h := newUnitsHistory(t)
	all := h.all(t, 0)
	if len(all) != 8 || all[0].ID != h.drop || all[len(all)-1].ID != h.feed {
		t.Fatalf("listed %d commits, from %s to %s", len(all), all[0].ID, all[len(all)-1].ID)
	}
	for i := 1; i < len(all); i++ {
		if all[i].At > all[i-1].At {
			t.Fatalf("not newest first at %d", i)
		}
	}
	n, err := h.repo.HistoryCount(context.Background(), LogOptions{}, 0)
	if err != nil || n != 8 {
		t.Fatalf("count = %d, %v", n, err)
	}
	// Since leaves out the older ones.
	recent := h.all(t, all[2].At)
	if len(recent) == 0 || len(recent) >= len(all) {
		t.Fatalf("since %d: %d commits", all[2].At, len(recent))
	}
	if n, _ := h.repo.HistoryCount(context.Background(), LogOptions{}, all[2].At); n != len(recent) {
		t.Fatalf("count since = %d, listed %d", n, len(recent))
	}
}

func TestHistoryChunkFindsWhereTheTextCameAndWent(t *testing.T) {
	t.Parallel()
	h := newUnitsHistory(t)
	every := ids(h.all(t, 0))
	ctx := context.Background()
	find := func(o HistoryOptions) []string {
		t.Helper()
		got, err := h.repo.HistoryChunk(ctx, o, every)
		if err != nil {
			t.Fatalf("%+v: %v", o, err)
		}
		return got
	}
	// -S: where the number of times km/h is there changed. Realigning the row didn't.
	cameAndWent := []string{h.drop, h.query, h.explain, h.knots, h.pin, h.feed}
	if got := find(HistoryOptions{Text: "km/h"}); !reflect.DeepEqual(got, cameAndWent) {
		t.Errorf("-S = %v", got)
	}
	// -G: any added or removed line with it, the realigned row too.
	if got := find(HistoryOptions{Text: "km/h", Lines: true}); !reflect.DeepEqual(got, []string{h.drop, h.query, h.realign, h.explain, h.knots, h.pin, h.feed}) {
		t.Errorf("-G = %v", got)
	}
	if got := find(HistoryOptions{Text: "KM/H", MatchCase: true}); len(got) != 0 {
		t.Errorf("matching case = %v", got)
	}
	if got := find(HistoryOptions{Text: "KM/H"}); !reflect.DeepEqual(got, cameAndWent) {
		t.Errorf("ignoring case = %v", got)
	}
	// knots for km/h leaves the regex's count as it was: one before, one after.
	if got := find(HistoryOptions{Text: "knots|km/h", Regex: true}); !reflect.DeepEqual(got, []string{h.drop, h.query, h.explain, h.pin, h.feed}) {
		t.Errorf("regex = %v", got)
	}
	// Plain text with characters a regular expression reads otherwise, under -G.
	if got := find(HistoryOptions{Text: "(km/h)", Lines: true}); !reflect.DeepEqual(got, []string{h.realign, h.explain}) {
		t.Errorf("-G as written = %v", got)
	}
	if got := find(HistoryOptions{Text: "km/h", Paths: []string{"src/"}}); !reflect.DeepEqual(got, []string{h.knots, h.feed}) {
		t.Errorf("only in src/ = %v", got)
	}
	// A chunk is searched in the order given, and only what's given.
	if got, _ := h.repo.HistoryChunk(ctx, HistoryOptions{Text: "km/h"}, []string{h.feed, h.readme, h.pin}); !reflect.DeepEqual(got, []string{h.feed, h.pin}) {
		t.Errorf("a chunk = %v", got)
	}
}

func TestHistoryChangesCountsEachFile(t *testing.T) {
	t.Parallel()
	h := newUnitsHistory(t)
	every := ids(h.all(t, 0))
	got, err := h.repo.HistoryChanges(context.Background(), HistoryOptions{Text: "km/h"}, every)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]HistoryChange{
		h.feed:    {{File: "src/feed.js", Added: 1}},
		h.pin:     {{File: "public/map.js", Added: 1}},
		h.knots:   {{File: "src/feed.js", Removed: 1}},
		h.explain: {{File: "docs/checks.md", Added: 1}},
		h.query:   {{File: "db/notes.sql", Added: 1}},
		// A removed "-- km/h note" reads "--- km/h note" in the patch: a line, not a header.
		h.drop: {{File: "db/notes.sql", Removed: 1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes =\n%v\nwant\n%v", got, want)
	}
	lines, err := h.repo.HistoryChanges(context.Background(), HistoryOptions{Text: "km/h", Lines: true}, []string{h.realign})
	if err != nil || !reflect.DeepEqual(lines[h.realign], []HistoryChange{{File: "docs/checks.md", Added: 1, Removed: 1}}) {
		t.Fatalf("-G changes = %v, %v", lines, err)
	}
}

func TestHistorySaysWhatsWrong(t *testing.T) {
	t.Parallel()
	h := newUnitsHistory(t)
	ctx := context.Background()
	for _, o := range []HistoryOptions{{Text: "kmalloc_array(", Lines: true, Regex: true}, {Text: "kmalloc_array(", Regex: true}} {
		_, err := h.repo.HistoryChunk(ctx, o, []string{h.feed})
		var pe *PatternError
		if !errors.As(err, &pe) || pe.Reason != "its ( and ) don't pair up" {
			t.Errorf("%+v: err = %v", o, err)
		}
	}
	for _, text := range []string{"", "two\nlines"} {
		if _, err := h.repo.HistoryChunk(ctx, HistoryOptions{Text: text}, []string{h.feed}); !errors.Is(err, ErrFindText) {
			t.Errorf("text %q: err = %v", text, err)
		}
	}
}

func TestHistoryInAPartialCloneDownloadsNothing(t *testing.T) {
	t.Parallel()
	dest := partialClone(t)
	runner := newTestRunner(t)
	if runner.version().Less(noLazyFetch) {
		t.Skip("git older than 2.44: TestHistoryWithAnOlderGitRefusesToDownload covers it")
	}
	repo := OpenRepo(runner, dest)
	var every []string
	if err := repo.HistoryCommits(context.Background(), LogOptions{}, 0, func(c HistoryCommit) error {
		every = append(every, c.ID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.HistoryChunk(context.Background(), HistoryOptions{Text: "km/h"}, every); !errors.Is(err, ErrNotDownloaded) {
		t.Fatalf("err = %v", err)
	}
	if !stillMissing(t, repo) {
		t.Fatal("the search downloaded old.txt")
	}
}

func TestHistoryWithAnOlderGitRefusesToDownload(t *testing.T) {
	t.Parallel()
	dest := partialClone(t)
	runner := newTestRunner(t)
	runner.verOnce.Do(func() { runner.ver = Version{2, 43, 0} })
	repo := OpenRepo(runner, dest)
	head, err := repo.ReadString(context.Background(), "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.HistoryChunk(context.Background(), HistoryOptions{Text: "km/h"}, []string{head}); !errors.Is(err, ErrLazyFetch) {
		t.Fatalf("err = %v", err)
	}
	if !stillMissing(t, repo) {
		t.Fatal("the search downloaded old.txt")
	}

}
