package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// #947: a commit that moves a function and edits it, and wraps a statement in an if, diffed
// through the app as the views ask for it.
func TestFileDiffFindsMovesThatChangedAndMovesIntoStructure(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	// forecast is the smaller function, so the line diff keeps run where it is and moves it.
	before := "package tides\n\nfunc forecast(station string) (int, error) {\n\treadings, err := fetchReadings(station)\n\treturn predict(readings), err\n}\n\nfunc run() {\n\tsetup()\n\tload()\n\tcheck()\n\treport()\n\tcleanup()\n\tfinish()\n}\n"
	after := "package tides\n\nfunc run() {\n\tsetup()\n\tif ready() {\n\t\tload()\n\t}\n\tcheck()\n\treport()\n\tcleanup()\n\tfinish()\n}\n\nfunc forecast(station string) (int, error) {\n\treadings, err := fetchReadings(ctx, station)\n\treturn predict(readings), err\n}\n"
	fx.CommitFile("tides.go", before, "first")
	fx.CommitFile("tides.go", after, "move forecast and wrap load")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fd, err := a.FileDiff(tab.Path, DiffCommit, fx.Head(), "tides.go", "", false, ReadPatient)
	if err != nil {
		t.Fatal(err)
	}
	if fd.Diff.Engine != "code" {
		t.Fatalf("not read as code: %+v", fd.Diff)
	}
	var changed, into int
	for _, m := range fd.Diff.Moves {
		if m.Changed && len(m.Edits) == 1 {
			changed++
		}
		if m.Into == "if statement" {
			into++
		}
	}
	if changed != 1 || into != 1 {
		t.Fatalf("moves %+v: want one that changed with its edit, and one into an if statement", fd.Diff.Moves)
	}
}
