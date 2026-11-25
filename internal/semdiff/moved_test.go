package semdiff

import (
	"context"
	"strings"
	"testing"
	"time"
)

// tidesFile is three functions; moving the first to the end deletes and adds it.
const tidesFile = `package tides

func forecast(station string) (Tide, error) {
	readings, err := fetchReadings(station)
	if err != nil {
		return Tide{}, err
	}
	return predict(readings), nil
}

func harbour() string {
	return "Tidewater harbour master"
}

func pier() string {
	return "Tidewater pier"
}
`

func movedAndEdited() (before, after string) {
	before = tidesFile
	fn := tidesFile[strings.Index(tidesFile, "func forecast"):strings.Index(tidesFile, "func harbour")]
	rest := strings.Replace(tidesFile, fn, "", 1)
	edited := strings.Replace(fn, "fetchReadings(station)", "fetchReadings(ctx, station)", 1)
	return before, rest + "\n" + strings.TrimRight(edited, "\n") + "\n"
}

func TestAMoveThatWasEditedIsAMoveThatChanged(t *testing.T) {
	before, after := movedAndEdited()
	for _, id := range []string{"", "go"} {
		res, err := Diff(context.Background(), before, after, Options{Moves: true, Language: id, ReadTime: -1})
		if err != nil {
			t.Fatal(err)
		}
		var changed []Move
		for _, mv := range res.Moves {
			if mv.Changed {
				changed = append(changed, mv)
			}
		}
		if len(changed) != 1 {
			t.Fatalf("%q: moves %+v, want one that changed", id, res.Moves)
		}
		mv := changed[0]
		a, b := split(before), split(after)
		if !strings.HasPrefix(a.span(mv.OldStart, mv.OldEnd), "func forecast") || !strings.HasPrefix(b.span(mv.NewStart, mv.NewEnd), "func forecast") {
			t.Fatalf("%q: move covers %q and %q", id, a.span(mv.OldStart, mv.OldEnd), b.span(mv.NewStart, mv.NewEnd))
		}
		// Its own diff has the one edit, on the move's second line.
		if len(mv.Edits) != 1 || mv.Edits[0].OldStart != 1 || mv.Edits[0].NewStart != 1 || mv.Edits[0].OldEnd != 2 {
			t.Fatalf("%q: edits %+v", id, mv.Edits)
		}
		if id == "go" {
			var marked []string
			line := strings.Split(b.span(mv.NewStart, mv.NewEnd), "\n")[1]
			for _, s := range mv.Edits[0].New {
				marked = append(marked, line[s.From:s.To])
			}
			if strings.Join(marked, "|") != "ctx," {
				t.Errorf("marked %q in the move", marked)
			}
		}
	}
}

// A move's own diff reads within QuickRead, as a file can have many, unless the caller asked
// for no limit: then a slow machine can't turn the move's code edits into line edits.
func TestAMoveReadsWithNoLimitWhenAskedTo(t *testing.T) {
	for _, c := range []struct{ caller, want time.Duration }{
		{-1, -1},
		{0, QuickRead},
		{time.Microsecond, QuickRead},
		{PatientRead, QuickRead},
		{LongRead, QuickRead},
	} {
		if got := moveReadTime(c.caller); got != c.want {
			t.Errorf("caller %s: a move reads within %s, want %s", c.caller, got, c.want)
		}
	}
}

func TestAnEditInPlaceIsNotAMove(t *testing.T) {
	before := tidesFile
	after := strings.Replace(tidesFile, "fetchReadings(station)", "fetchReadings(ctx, station)", 1)
	res, _ := Diff(context.Background(), before, after, Options{Moves: true})
	if len(res.Moves) != 0 {
		t.Errorf("moves %+v, want none", res.Moves)
	}
}

func TestCodeRewrittenElsewhereIsNotAMove(t *testing.T) {
	// Deleted here and something else added there: they share only braces.
	before := tidesFile
	after := strings.Replace(tidesFile, "func harbour() string {\n\treturn \"Tidewater harbour master\"\n}\n", "", 1) +
		"\nfunc ebb(level float64) bool {\n\tif level < 0 {\n\t\treturn true\n\t}\n\treturn false\n}\n"
	res, _ := Diff(context.Background(), before, after, Options{Moves: true})
	for _, mv := range res.Moves {
		if mv.Changed {
			t.Errorf("move %+v, want none that changed", mv)
		}
	}
}
