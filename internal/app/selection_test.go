package app

import (
	"os"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestSelectedCommitsOldestFirst(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Ana Cruz", "ana@tidewater.example", "a.txt", "One")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two")
	commitAs(fx, "Ana Cruz", "ana@tidewater.example", "c.txt", "Three")
	commitAs(fx, "Ana Cruz", "ana@tidewater.example", "d.txt", "Four")
	path := openForMap(t, a, fx)
	graphRows(t, a, path, 4)
	page, err := a.GraphRows(path, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	id := func(row int) string { return page.Rows[row].ID }

	// Rows 0 and 1 (Four, Three) and row 3 (One), picked in any order.
	sel, err := a.SelectedCommits(path, []SelectionPick{{Lo: 0, Hi: 1, LoID: id(0), HiID: id(1)}, {Lo: 3, Hi: 3, LoID: id(3)}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{id(3), id(1), id(0)}; !slices.Equal(sel.IDs, want) {
		t.Fatalf("ids = %v, want oldest first %v", sel.IDs, want)
	}
	if len(sel.Commits) != 3 || sel.Commits[0].Subject != "One" || sel.Commits[2].Subject != "Four" {
		t.Fatalf("commits = %+v", sel.Commits)
	}
	if sel.Authors != 1 || sel.Author != "Ana Cruz" || !sel.Head || sel.Merges != 0 || sel.Stale {
		t.Fatalf("selection = %+v", sel)
	}
	if sel.First > sel.Last {
		t.Fatalf("first %d after last %d", sel.First, sel.Last)
	}

	two, err := a.SelectedCommits(path, []SelectionPick{{Lo: 1, Hi: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if two.Authors != 2 || two.Author != "" || two.Head {
		t.Fatalf("two authors, not HEAD: %+v", two)
	}

	// Rows that aren't the commits picked: the graph changed under the selection.
	stale, err := a.SelectedCommits(path, []SelectionPick{{Lo: 0, Hi: 1, LoID: id(1)}})
	if err != nil || !stale.Stale || len(stale.IDs) != 0 {
		t.Fatalf("a pick whose end moved should be stale: %+v, %v", stale, err)
	}
	if past, err := a.SelectedCommits(path, []SelectionPick{{Lo: 2, Hi: 9}}); err != nil || !past.Stale {
		t.Fatalf("rows past the end should be stale: %+v, %v", past, err)
	}
}

func TestSelectionFilesMostChangedFirst(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	// A root commit changes nothing here: it has no parent to differ from.
	fx.CommitFile("base.txt", "0\n", "base")
	one := fx.CommitFile("b.txt", "1\n", "one")
	two := fx.CommitFile("a.txt", "1\n", "two")
	three := fx.CommitFile("b.txt", "2\n", "three")
	path := openForMap(t, a, fx)

	got, err := a.SelectionFiles(path, []string{one, two, three})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || got.Read != 3 || len(got.Files) != 2 {
		t.Fatalf("files = %+v", got)
	}
	if f := got.Files[0]; f.Path != "b.txt" || !slices.Equal(f.Commits, []string{one, three}) {
		t.Fatalf("most changed first, its commits oldest first: %+v", f)
	}
	if f := got.Files[1]; f.Path != "a.txt" || !slices.Equal(f.Commits, []string{two}) {
		t.Fatalf("then the rest: %+v", f)
	}
	none, err := a.SelectionFiles(path, nil)
	if err != nil || none.Files == nil || none.Total != 0 {
		t.Fatalf("no commits = %+v, %v", none, err)
	}
}

func TestCommitRowsFollowsCommits(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "one")
	two := fx.CommitFile("a.txt", "2\n", "two")
	path := openForMap(t, a, fx)
	graphRows(t, a, path, 2)
	rows, err := a.CommitRows(path, []string{two, "0123456789abcdef0123456789abcdef01234567"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rows, []int{0, rowAbsent}) {
		t.Fatalf("rows = %v", rows)
	}
}

func TestIntegrateRevertsSeveral(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "base")
	first := fx.CommitFile("a.txt", "two\n", "two")
	second := fx.CommitFile("a.txt", "three\n", "three")
	path := openForMap(t, a, fx)

	if err := a.Integrate(path, "revert-all", []string{second, first}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(fx.Path("a.txt")); string(got) != "one\n" {
		t.Fatalf("a.txt = %q", got)
	}
	if err := a.Integrate(path, "revert-all", nil); err == nil {
		t.Fatal("reverting nothing should say so")
	}
	if got := commitsLabel([]string{first, second}); got != "2 commits" {
		t.Fatalf("label = %q", got)
	}
}
