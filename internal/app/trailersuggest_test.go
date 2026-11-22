package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// trailerSuggestRepo is five commits with three keys: a label, tickets and people.
func trailerSuggestRepo(t *testing.T) (*App, string) {
	t.Helper()
	a := newTestApp(t)
	fx := gittest.New(t)
	sam, sarah := "Sam Ortiz <sam@tidewater.example>", "Sarah Lind <sarah@tidewater.example>"
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "fix: crash on an empty feed", "Changelog: fixed", "Closes: #1", "Reviewed-by: "+sarah)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Round tide heights", "Changelog: added", "Closes: #12", "Fixes: #7", "Reviewed-by: "+sarah)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Raise an alert on high wind", "Changelog: fixed", "Reviewed-by: "+sam)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "d.txt", "Tidy up")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "e.txt", "Rename a script", "Changelog: none", "Reviewed-by: "+sam)
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 5)
	return a, path
}

// The search box understands a few things typed out: any value of a key, a leading "-" for
// the commits that don't match, and a key by any of its endings.
func TestSearchUnderstandsHasAndNot(t *testing.T) {
	t.Parallel()
	a, path := trailerSuggestRepo(t)
	for _, c := range []struct {
		query string
		want  int
		why   string
	}{
		{"has:changelog", 4, "every commit with the key"},
		{"has:Reviewed-By", 4, "whatever the case"},
		{"-has:changelog", 1, "the one commit without it"},
		{"-changelog: none", 4, "every commit but the one that says none, with or without the key"},
		{"-Reviewed-by: Sarah", 3, "the three she didn't review"},
		{"closed: #12", 1, "Closes, by another ending"},
		{"closed: #1", 1, "a number still matches whole"},
		{"fix: #7", 1, "Fixes, by another ending"},
		{"fix: crash", 1, "the subject: a spelling no commit uses is still searched for as text"},
		{"has:nothing", 0, "no such key, and no such text"},
	} {
		res, err := a.SearchGraph(path, c.query)
		if err != nil {
			t.Fatal(err)
		}
		if res.Total != c.want || len(res.Rows) != c.want {
			t.Errorf("%q found %d (%v), want %d: %s", c.query, res.Total, res.Rows, c.want, c.why)
		}
	}
	// What doesn't match is given by its rows, newest first.
	if res, _ := a.SearchGraph(path, "-has:changelog"); len(res.Rows) != 1 || res.Rows[0] != 1 {
		t.Errorf("-has:changelog = %v, want row 1, the commit with no trailers", res.Rows)
	}
}

// What the search box offers under what's typed: keys and values that contain it, a key's
// values after its colon, and counts within the filter that's on.
func TestTrailerSuggest(t *testing.T) {
	t.Parallel()
	a, path := trailerSuggestRepo(t)
	sam, sarah := "person:Sam Ortiz <sam@tidewater.example>", "person:Sarah Lind <sarah@tidewater.example>"
	offered := func(query string) TrailerSuggestions {
		t.Helper()
		got, err := a.TrailerSuggest(path, query, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got.Items == nil {
			t.Fatalf("%q: the rows are nil, and the window needs a list", query)
		}
		return got
	}
	// row finds an offered value by its ID, or a key's own row by "".
	row := func(got TrailerSuggestions, key, value string) (TrailerSuggestion, bool) {
		for _, item := range got.Items {
			if item.Key == key && item.Value == value {
				return item, true
			}
		}
		return TrailerSuggestion{}, false
	}

	// A word: the values that contain it, with whose they are.
	got := offered("sa")
	if got.Match != "sa" || len(got.Items) != 2 {
		t.Fatalf("sa = %+v, want the two reviewers", got)
	}
	if item, ok := row(got, "reviewed-by", sarah); !ok || item.KeyLabel != "Reviewed-by" || item.Label != "Sarah Lind <sarah@tidewater.example>" || item.Kind != string(trailerindex.KindPerson) || item.Commits != 2 {
		t.Fatalf("Sarah as offered = %+v, %v", item, ok)
	}

	// A word in a key's name: the key itself comes first, to choose among its values.
	got = offered("change")
	if len(got.Items) != 1 || got.Items[0].Key != "changelog" || got.Items[0].Value != "" || got.Items[0].KeyLabel != "Changelog" || got.Items[0].Commits != 4 {
		t.Fatalf("change = %+v, want the Changelog key", got.Items)
	}

	// A key and a colon: its values, narrowed by what follows.
	got = offered("Changelog: ")
	if got.Match != "" || len(got.Items) != 3 || got.Items[0].Value != "text:fixed" || got.Items[0].Commits != 2 {
		t.Fatalf("Changelog: = %+v, want its three values, fixed first", got)
	}
	got = offered("changelog:f")
	if got.Match != "f" || len(got.Items) != 1 || got.Items[0].Value != "text:fixed" {
		t.Fatalf("changelog:f = %+v, want fixed", got)
	}
	// By another ending of the key.
	if got = offered("closed: "); len(got.Items) != 2 {
		t.Fatalf("closed: = %+v, want the two tickets Closes has", got.Items)
	}
	// Nothing is offered for nothing, for a search of what doesn't match, or for no such thing.
	for _, q := range []string{"", "  ", "-changelog", "zzz"} {
		if got = offered(q); len(got.Items) != 0 {
			t.Fatalf("%q offered %+v", q, got.Items)
		}
	}
	if got = offered("sa"); !got.Status.Done {
		t.Fatalf("status = %+v, want the history read", got.Status)
	}

	// With a filter on, a value counts the commits picking it would leave.
	if _, err := a.SetTrailerFilter(path, []trailerindex.Pick{{Key: "reviewed-by", Value: sarah}}, false); err != nil {
		t.Fatal(err)
	}
	got = offered("Changelog: ")
	if len(got.Items) != 2 {
		t.Fatalf("Changelog: within Sarah's reviews = %+v, want fixed and added: none would leave nothing", got.Items)
	}
	for _, item := range got.Items {
		if item.Commits != 1 {
			t.Fatalf("%s within Sarah's reviews = %d commits, want 1", item.Label, item.Commits)
		}
	}
	// A picked value isn't offered again, and another value of its key counts as it is:
	// values of one key are either-or.
	got = offered("sa")
	if _, there := row(got, "reviewed-by", sarah); there {
		t.Fatalf("Sarah is picked and offered again: %+v", got.Items)
	}
	if item, ok := row(got, "reviewed-by", sam); !ok || item.Commits != 2 {
		t.Fatalf("Sam beside Sarah = %+v, %v, want his 2 commits", item, ok)
	}
}
