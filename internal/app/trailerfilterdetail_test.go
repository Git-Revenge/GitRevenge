package app

import (
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// What the graph's finer points ask of a filter: which of a commit's lines it picks, and
// what a day held before it took commits out.
func TestTrailerFilterLinesAndDays(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	sam, sarah := "Sam Ortiz <sam@tidewater.example>", "Sarah Lind <sarah@tidewater.example>"
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "One", "Changelog: fixed", "Reviewed-by: "+sarah)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two", "Changelog: added", "Reviewed-by: sarah lind <slind@users.noreply.example>", "Reviewed-by: "+sam)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Three", "Changelog: fixed", "Reviewed-by: "+sam)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "d.txt", "Four")
	fx.WriteFile(".mailmap", "Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>\n")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 4)
	page, err := a.GraphRows(path, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]string{}
	for _, row := range page.Rows {
		id[row.Subject] = row.ID
	}

	// No filter: nothing is picked, and no day is counted.
	if lines, err := a.TrailerFilterLines(path, id["One"]); err != nil || len(lines) != 0 {
		t.Fatalf("lines with no filter = %+v, %v", lines, err)
	}
	if days, err := a.TrailerFilterDays(path, []int64{0, 1 << 62}); err != nil || len(days) != 0 {
		t.Fatalf("days with no filter = %v, %v", days, err)
	}

	// Sarah's reviews, by the name she's shown under.
	bySarah := trailerindex.Pick{Key: "reviewed-by", Value: "person:" + sarah}
	state, err := a.SetTrailerFilter(path, []trailerindex.Pick{bySarah}, true)
	if err != nil || state.Kept != 2 {
		t.Fatalf("Sarah's reviews = %+v, %v, want 2 commits: the mailmap joins her two names", state, err)
	}
	graphRows(t, a, path, 2)

	// The line that matched, as the commit wrote it: her old name on Two, and not Sam's line.
	lines, err := a.TrailerFilterLines(path, id["Two"])
	if err != nil || len(lines) != 1 || lines[0].Key != "Reviewed-by" || lines[0].Value != "sarah lind <slind@users.noreply.example>" {
		t.Fatalf("the picked lines of Two = %+v, %v, want her line as written", lines, err)
	}
	if lines, _ = a.TrailerFilterLines(path, id["One"]); len(lines) != 1 || lines[0].Value != sarah {
		t.Fatalf("the picked lines of One = %+v", lines)
	}
	// A commit the filter doesn't keep has none, and neither has one with no trailers or no such id.
	for _, sha := range []string{id["Three"], id["Four"], "not an id"} {
		if lines, err = a.TrailerFilterLines(path, sha); err != nil || len(lines) != 0 {
			t.Fatalf("the picked lines of %q = %+v, %v", sha, lines, err)
		}
	}
	// Two picks of two keys: a line of each.
	if _, err = a.SetTrailerFilter(path, []trailerindex.Pick{bySarah, {Key: "changelog", Value: "text:added"}}, true); err != nil {
		t.Fatal(err)
	}
	if lines, _ = a.TrailerFilterLines(path, id["Two"]); len(lines) != 2 || lines[0].Key != "Changelog" {
		t.Fatalf("the picked lines of Two under two picks = %+v", lines)
	}

	// Every commit was authored some time: the span of all time holds all four, kept or not.
	var days []int
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if days, err = a.TrailerFilterDays(path, []int64{0, 1 << 62, 0, 1}); err != nil {
			t.Fatal(err)
		}
		if len(days) == 2 {
			break
		}
	}
	if len(days) != 2 || days[0] != 4 || days[1] != 0 {
		t.Fatalf("days = %v, want the 4 commits of all time and none in its first second", days)
	}

	// Highlight takes nothing out, so there's nothing to count against.
	if _, err = a.SetTrailerFilter(path, []trailerindex.Pick{bySarah}, false); err != nil {
		t.Fatal(err)
	}
	graphRows(t, a, path, 4)
	if days, _ = a.TrailerFilterDays(path, []int64{0, 1 << 62}); len(days) != 0 {
		t.Fatalf("days under Highlight = %v", days)
	}
	// Each row says whether it's one of the filter's, however many there are.
	page, err = a.GraphRows(path, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range page.Rows {
		if want := row.Subject == "One" || row.Subject == "Two"; row.Match != want {
			t.Fatalf("under Highlight, %q says match = %v", row.Subject, row.Match)
		}
	}
	// The lines are still marked.
	if lines, _ = a.TrailerFilterLines(path, id["One"]); len(lines) != 1 {
		t.Fatalf("the picked lines of One under Highlight = %+v", lines)
	}

	// With no filter, no row is marked.
	if _, err = a.SetTrailerFilter(path, nil, true); err != nil {
		t.Fatal(err)
	}
	page, _ = a.GraphRows(path, 0, 10)
	for _, row := range page.Rows {
		if row.Match {
			t.Fatalf("with no filter, %q is marked", row.Subject)
		}
	}
}
