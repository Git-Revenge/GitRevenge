package graph

import (
	"fmt"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
)

func TestMonthMarks(t *testing.T) {
	at := func(y int, m time.Month, d int) int64 { return time.Date(y, m, d, 12, 0, 0, 0, time.Local).Unix() }
	// Newest first, as the graph lists them: three in November, two in October, one in
	// October a year before, and one dated out of order, back in November.
	times := []int64{
		at(2026, time.November, 10), at(2026, time.November, 9), at(2026, time.November, 1),
		at(2026, time.October, 31), at(2026, time.October, 2),
		at(2025, time.October, 15),
		at(2026, time.November, 3),
	}
	g := New()
	for i, when := range times {
		c := git.LogCommit{ID: fmt.Sprintf("%040d", i+1), AuthorName: "Ada", Subject: "c", AuthorTime: when}
		if i < len(times)-1 {
			c.Parents = []string{fmt.Sprintf("%040d", i+2)}
		}
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	marks := g.MonthMarks()
	var rows []int
	for _, m := range marks {
		rows = append(rows, m.Row)
		if m.Time != times[m.Row] {
			t.Fatalf("mark at row %d has time %d, want %d", m.Row, m.Time, times[m.Row])
		}
	}
	want := []int{0, 3, 5, 6}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("month marks at rows %v, want %v", rows, want)
	}
	if got := New().MonthMarks(); got == nil || len(got) != 0 {
		t.Fatalf("an empty graph has marks %v, want none as an empty list", got)
	}
}
