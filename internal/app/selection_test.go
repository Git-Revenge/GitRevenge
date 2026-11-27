package app

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

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

func TestSelectionIsALineWhenEachIsTheNextsParent(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	base := fx.CommitFile("a.txt", "1\n", "base")
	fx.CommitFile("a.txt", "2\n", "two")
	fx.CommitFile("a.txt", "3\n", "three")
	fx.CommitFile("a.txt", "4\n", "four")
	path := openForMap(t, a, fx)
	graphRows(t, a, path, 4)

	line, err := a.SelectedCommits(path, []SelectionPick{{Lo: 0, Hi: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !line.Line || line.Base != base || line.BaseEdge {
		t.Fatalf("rows 0-2 should be a line from base: %+v", line)
	}
	gap, err := a.SelectedCommits(path, []SelectionPick{{Lo: 0, Hi: 0}, {Lo: 2, Hi: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if gap.Line {
		t.Fatalf("rows 0 and 2 skip one: not a line: %+v", gap)
	}
	// The whole history starts at a first commit: its base is nothing.
	all, err := a.SelectedCommits(path, []SelectionPick{{Lo: 0, Hi: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if !all.Line || all.Base != "" {
		t.Fatalf("the whole history is a line from nothing: %+v", all)
	}
}

func TestCompareCommits(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	base := fx.CommitFile("a.txt", "one\n", "base")
	fx.Git("checkout", "-q", "-b", "side")
	side := fx.CommitFile("s.txt", "s\n", "side")
	fx.Git("checkout", "-q", "-")
	fx.CommitFile("a.txt", "one\ntwo\n", "main one")
	main := fx.CommitFile("m.txt", "m\n", "main two")
	path := openForMap(t, a, fx)

	got, err := a.CompareCommits(path, side, main)
	if err != nil {
		t.Fatal(err)
	}
	if got.OnlyFrom != 1 || got.OnlyTo != 2 {
		t.Fatalf("side has 1 the other hasn't, main 2: %+v", got)
	}
	paths := map[string]string{}
	for _, f := range got.Files {
		paths[f.Path] = f.Status
	}
	if paths["s.txt"] != "D" || paths["m.txt"] != "A" || paths["a.txt"] != "M" {
		t.Fatalf("files = %v", paths)
	}

	// A file's two versions, in the comparison's mode.
	c, err := a.FileContents(path, DiffCompare, side+".."+main, "a.txt", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Old.Text != "one\n" || c.New.Text != "one\ntwo\n" {
		t.Fatalf("a.txt from side to main = %q → %q", c.Old.Text, c.New.Text)
	}
	// From nothing, the file is all new.
	fresh, err := a.FileContents(path, DiffCompare, ".."+base, "a.txt", "", false)
	if err != nil || fresh.Old.Exists || fresh.New.Text != "one\n" {
		t.Fatalf("from nothing = %+v, %v", fresh, err)
	}
	if _, err := a.FileContents(path, DiffCompare, main, "a.txt", "", false); err == nil {
		t.Fatal("a comparison with one commit should be refused")
	}
	nothing, err := a.CompareCommits(path, "", base)
	if err != nil || len(nothing.Files) != 1 || nothing.OnlyFrom != 0 || nothing.OnlyTo != 0 {
		t.Fatalf("from nothing = %+v, %v", nothing, err)
	}
}

func TestCompareModeRenamesDeletesAndBinaries(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	base := fx.CommitFile("a.txt", "one\ntwo\n", "base")
	fx.CommitFile("gone.txt", "bye\n", "add gone")
	fx.WriteFile("pic.bin", "\x00\x01\x02binary")
	fx.Git("add", "pic.bin")
	fx.Git("commit", "-q", "-m", "add a binary")
	from := fx.Head()
	fx.Git("mv", "a.txt", "c.txt")
	fx.Git("rm", "-q", "gone.txt")
	fx.WriteFile("pic.bin", "\x00\x03\x04binary")
	fx.Git("add", "-A")
	fx.Git("commit", "-q", "-m", "rename, delete, change the binary")
	to := fx.Head()
	path := openForMap(t, a, fx)
	commits := from + ".." + to

	renamed, err := a.FileContents(path, DiffCompare, commits, "c.txt", "a.txt", false)
	if err != nil || renamed.Old.Text != "one\ntwo\n" || renamed.New.Text != "one\ntwo\n" {
		t.Fatalf("renamed = %+v, %v", renamed, err)
	}
	gone, err := a.FileContents(path, DiffCompare, commits, "gone.txt", "", false)
	if err != nil || !gone.Old.Exists || gone.New.Exists {
		t.Fatalf("deleted = %+v, %v", gone, err)
	}
	bin, err := a.FileContents(path, DiffCompare, commits, "pic.bin", "", false)
	if err != nil || bin.Kind != "binary" {
		t.Fatalf("binary = kind %q, %v", bin.Kind, err)
	}
	// The version a preview shows is the second commit's.
	if at, ok := versionRef(DiffCompare, base+".."+to, "c.txt"); !ok || at.Rev != to {
		t.Fatalf("versionRef = %+v, %v", at, ok)
	}
}

func TestCompareInAPartialCloneSaysWhatsMissing(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	path, head := partialTab(t, a)
	page, err := a.GraphRows(path, 0, 2)
	for deadline := time.Now().Add(10 * time.Second); (err != nil || len(page.Rows) < 2) && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		page, err = a.GraphRows(path, 0, 2)
	}
	if len(page.Rows) < 2 {
		t.Fatalf("rows = %+v, %v", page.Rows, err)
	}
	first := page.Rows[1].ID

	cmp, err := a.CompareCommits(path, first, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmp.Files) != 1 || cmp.Files[0].Path != "notes.txt" || len(cmp.CountsMissing) == 0 {
		t.Fatalf("comparison = %+v; want notes.txt named, its counts missing", cmp)
	}
	fc, err := a.FileContents(path, DiffCompare, first+".."+head, "notes.txt", "", false)
	if err != nil || fc.Kind != "missing" || len(fc.Missing) != 1 {
		t.Fatalf("file = kind %q, missing %v, %v; want the first version missing", fc.Kind, fc.Missing, err)
	}
}

func TestSquashOffer(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	local, _ := gittest.WithRemote(t, 2)
	pushed := local.Head()
	one := local.CommitFile("a.txt", "1\n", "one")
	two := local.CommitFile("a.txt", "2\n", "two")
	path := openForMap(t, a, local)
	offer := func(oldest, newest string) SquashOffer {
		t.Helper()
		o, err := a.SquashOffer(path, oldest, newest)
		if err != nil {
			t.Fatal(err)
		}
		return o
	}

	if o := offer(one, two); !o.OK {
		t.Fatalf("two new commits on main should squash: %+v", o)
	}
	if o := offer(pushed, one); o.OK || o.Reason != shortRef(pushed)+" is on origin/main too." {
		t.Fatalf("a pushed commit = %+v", o)
	}
	local.Git("branch", "keep", one)
	if o := offer(one, two); o.OK || o.Reason != shortRef(one)+" is on keep too." {
		t.Fatalf("a commit another branch holds = %+v", o)
	}
	local.Git("checkout", "-q", "-b", "side", pushed)
	if o := offer(one, two); o.OK || o.Reason != "Only for one line on side." {
		t.Fatalf("commits that aren't on the checked-out branch = %+v", o)
	}
	local.Git("checkout", "-q", "--detach", two)
	if o := offer(one, two); o.OK || !strings.Contains(o.Reason, "detached") {
		t.Fatalf("a detached HEAD = %+v", o)
	}
}
