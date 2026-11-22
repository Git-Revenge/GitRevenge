package app

import (
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// Shipped reads a commit's trailers through the reader the Smart filters panel uses, so a
// ticket counted in one is the same ticket in the other, and a repository's rules for its
// trailer keys reach both.
func TestShippedReadsTrailersAsSmartFiltersDo(t *testing.T) {
	isolateGlobal(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "Start")
	fx.CommitFile("a.txt", "2\n", "Round tides (#1)\n\nTicket: tide 12\nBug: #7\nCloses: #3\nFixes: 00d60fd3b932 (\"KEYS: keyctls [ver #2]\")\nReviewed-by: Sam Ortiz <sam@tidewater.example>")
	a := newTestApp(t)
	tab, _ := a.OpenRepo(fx.Dir)
	refs := func() []string {
		t.Helper()
		res, err := a.InsightsShipped(tab.Path, InsightsInputs{Window: "7d"})
		if err != nil {
			t.Fatal(err)
		}
		return res.Items[0].Refs
	}

	// The subject's reference, then the trailers that name tickets, whichever key they're
	// under. A quoted subject's "#2" under Fixes isn't one, and neither is "tide 12" yet.
	if got := refs(); !slices.Equal(got, []string{"#1", "#7", "#3"}) {
		t.Fatalf("references = %v", got)
	}

	// The panel reads the same commit the same way: the same tickets, under their keys.
	trailersRead(t, a, tab.Path)
	for key, want := range map[string]string{"bug": "ticket:#7", "closes": "ticket:#3"} {
		values, err := a.TrailerValues(tab.Path, key, "", 0, 0)
		if err != nil || len(values.Values) != 1 || values.Values[0].ID != want || values.Values[0].Kind != trailerindex.KindTicket {
			t.Fatalf("the panel's values of %s = %+v, %v, want the ticket Shipped found", key, values.Values, err)
		}
	}

	// The repository says Ticket holds tickets: Shipped follows at once, with nothing else changed.
	if _, err := a.SetTrailerKind(tab.Path, "ticket", "tickets"); err != nil {
		t.Fatal(err)
	}
	if got := refs(); !slices.Equal(got, []string{"#1", "tide 12", "#7", "#3"}) {
		t.Fatalf("with Ticket said to hold tickets = %v", got)
	}
	// A key it holds back names none.
	if _, err := a.SetTrailerPlace(tab.Path, "bug", "held"); err != nil {
		t.Fatal(err)
	}
	if got := refs(); !slices.Equal(got, []string{"#1", "tide 12", "#3"}) {
		t.Fatalf("with Bug held back = %v", got)
	}
	// And left to the guess again, it's back.
	if _, err := a.SetTrailerPlace(tab.Path, "bug", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetTrailerKind(tab.Path, "ticket", ""); err != nil {
		t.Fatal(err)
	}
	if got := refs(); !slices.Equal(got, []string{"#1", "#7", "#3"}) {
		t.Fatalf("with the rules taken back = %v", got)
	}
}

// Shipped reads its range for itself: it gives the same answer before a repository's
// trailers have been read for the panel as after.
func TestShippedDoesNotWaitForTheTrailerIndex(t *testing.T) {
	isolateGlobal(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "Start")
	fx.CommitFile("a.txt", "2\n", "Round tides\n\nCloses: #3, #4")
	a := newTestApp(t)
	// Asked by path, with no tab open and so no trailer index at all.
	res, err := a.InsightsShipped(fx.Dir, InsightsInputs{Window: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Items[0].Refs; !slices.Equal(got, []string{"#3", "#4"}) {
		t.Fatalf("with no index, references = %v", got)
	}
	tab, _ := a.OpenRepo(fx.Dir)
	trailersRead(t, a, tab.Path)
	res, err = a.InsightsShipped(tab.Path, InsightsInputs{Window: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Items[0].Refs; !slices.Equal(got, []string{"#3", "#4"}) {
		t.Fatalf("once the index is read, references = %v", got)
	}
}
