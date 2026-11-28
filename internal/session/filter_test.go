package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// A filter leaves only the commits it keeps, newest first, strung on one lane: they aren't
// neighbours any more, and the graph doesn't draw them as if they were.
func TestFilterKeepsOnlyItsCommitsOnOneLane(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	ids := fx.Linear(5)
	fx.Branch("topic")
	fx.Checkout("topic")
	side := fx.CommitFile("side.txt", "s", "On a branch")
	fx.Checkout("main")
	top := fx.CommitFile("top.txt", "t", "On main")
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 7)
	if on, _, _ := r.Filtered(); on {
		t.Fatal("a graph nobody filtered says it's filtered")
	}
	if _, ok := r.Authored([][2]int64{{0, 1 << 62}}); ok {
		t.Fatal("a graph nobody filtered counts what a filter passed over")
	}

	keep := map[string]struct{}{ids[1]: {}, ids[3]: {}, side: {}}
	r.SetFilter(&Filter{Keep: keep, Sig: "one"})
	waitGraph(t, r, 3)
	g, _ := r.Graph()
	rows := g.Rows(0, 3)
	// Newest first: the branch's commit was made after the five, then ids[3], then ids[1].
	for i, want := range []string{side, ids[3], ids[1]} {
		if rows[i].ID != want {
			t.Fatalf("row %d is %s, want %s", i, rows[i].ID[:7], want[:7])
		}
		if rows[i].Col != 0 {
			t.Errorf("row %d is in lane %d, want the one lane", i, rows[i].Col)
		}
	}
	// Each is shown as the child of the next, and the last has nothing after it.
	if rows[0].Parents != 1 || rows[1].Parents != 1 || rows[2].Parents != 0 {
		t.Fatalf("parents = %d, %d, %d, want 1, 1, 0", rows[0].Parents, rows[1].Parents, rows[2].Parents)
	}
	if _, there := g.RowOf(top); there {
		t.Fatal("a commit the filter doesn't keep is in the graph")
	}
	if on, looked, total := r.Filtered(); !on || looked != 7 || total != 7 {
		t.Fatalf("filtered = %v, having looked through %d of %d commits, want all 7", on, looked, total)
	}

	// What a span of time held before the filter: every commit looked through, kept or not.
	rows = g.Rows(0, 3)
	when := rows[1].AuthorTime
	counts, ok := r.Authored([][2]int64{{0, 1 << 62}, {0, 1}, {when, when + 1}})
	if !ok || counts[0] != 7 || counts[1] != 0 || counts[2] < 1 {
		t.Fatalf("authored = %v, %v, want all 7, none before time began, and at least the commit itself", counts, ok)
	}

	// Another filter replaces it, and one that keeps nothing shows nothing.
	r.SetFilter(&Filter{Keep: map[string]struct{}{top: {}}, Sig: "two"})
	waitGraph(t, r, 1)
	r.SetFilter(&Filter{Keep: map[string]struct{}{}, Sig: "none"})
	waitGraph(t, r, 0)

	// Cleared, everything is back as it was.
	r.SetFilter(nil)
	waitGraph(t, r, 7)
	g, _ = r.Graph()
	if row, ok := g.RowOf(top); !ok || row != 0 {
		t.Fatalf("the newest commit is at row %d (found %v) once the filter is cleared", row, ok)
	}
}

// A filter and a focus both narrow the graph: what shows passes both.
func TestFilterStacksWithFocus(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	ids := fx.Linear(3)
	fx.Branch("topic")
	fx.Checkout("topic")
	side := fx.CommitFile("side.txt", "s", "On a branch")
	fx.Checkout("main")
	top := fx.CommitFile("top.txt", "t", "On main")
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 5)

	r.SetFilter(&Filter{Keep: map[string]struct{}{ids[0]: {}, side: {}, top: {}}, Sig: "one"})
	waitGraph(t, r, 3)
	// Focused on the branch, the commit only main has is out of reach, kept or not.
	r.SetFocus(Focus{Branch: "topic"})
	waitGraph(t, r, 2)
	g, _ := r.Graph()
	if _, there := g.RowOf(top); there {
		t.Fatal("a commit outside the focus shows under the filter")
	}
	for _, id := range []string{side, ids[0]} {
		if _, ok := g.RowOf(id); !ok {
			t.Fatalf("%s is kept and in focus, and isn't in the graph", id[:7])
		}
	}
}

// #995: a filter's Until ends a load's walk there, for a history search that has looked at
// nothing older: what it shows is whole for what it reached, and it doesn't count the rest.
func TestFilterUntilEndsTheWalk(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	ids := fx.Linear(8) // ids[7] is the newest
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 8)
	keep := map[string]struct{}{ids[6]: {}, ids[4]: {}, ids[1]: {}}
	rowsOf := func() []string {
		g, _ := r.Graph()
		var out []string
		for _, row := range g.Rows(0, g.Len()) {
			out = append(out, row.ID)
		}
		return out
	}

	// Ending at a commit the filter passes over: the kept ones newer than it.
	r.SetFilter(&Filter{Keep: keep, Sig: "to 3", Until: ids[3]})
	waitGraph(t, r, 2)
	if got := rowsOf(); got[0] != ids[6] || got[1] != ids[4] {
		t.Fatalf("rows = %v", got)
	}
	g, _ := r.Graph()
	if rows := g.Rows(0, 2); rows[1].Parents != 0 {
		t.Errorf("the last row shown has %d parents, want none", rows[1].Parents)
	}
	// Not the whole history: it isn't counted as looked through, nor are its days.
	if on, looked, _ := r.Filtered(); !on || looked != 0 {
		t.Errorf("filtered = %v, looked %d, want nothing counted", on, looked)
	}
	if _, ok := r.Authored([][2]int64{{0, 1 << 62}}); ok {
		t.Error("a walk that ended early counts its days")
	}

	// Ending at a kept commit: it's shown, the last.
	r.SetFilter(&Filter{Keep: keep, Sig: "to 4", Until: ids[4]})
	waitGraph(t, r, 2)
	if got := rowsOf(); got[1] != ids[4] {
		t.Fatalf("rows = %v", got)
	}

	// A commit the walk never meets: the whole history, as without it.
	r.SetFilter(&Filter{Keep: keep, Sig: "nowhere", Until: "0000000000000000000000000000000000000000"})
	waitGraph(t, r, 3)
	if on, looked, total := r.Filtered(); !on || looked != 8 || total != 8 {
		t.Errorf("filtered = %v, looked %d of %d, want all 8", on, looked, total)
	}

	// Without it, the whole history.
	r.SetFilter(&Filter{Keep: keep, Sig: "all"})
	waitGraph(t, r, 3)
	if got := rowsOf(); got[2] != ids[1] {
		t.Fatalf("rows = %v", got)
	}
}

// #1059: a path focus narrows the graph to the commits that changed something under the path,
// with git's own simplification, and stacks with a branch focus.
func TestFocusOnAPath(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	c1 := fx.CommitFile("a/x.txt", "x\n", "c1: a")
	fx.CommitFile("b/y.txt", "y\n", "c2: b")
	fx.Branch("side")
	s1 := fx.CommitFile("a/x2.txt", "x2\n", "s1: a, on side")
	fx.Checkout("main")
	m1 := fx.CommitFile("a/x.txt", "x, on main\n", "m1: a, on main")
	fx.CommitFile("b/w.txt", "w\n", "c3: b")
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 5)

	r.SetFocus(Focus{Path: "a/"})
	waitGraph(t, r, 3)
	g, _ := r.Graph()
	for _, id := range []string{c1, s1, m1} {
		if _, ok := g.RowOf(id); !ok {
			t.Errorf("%s touched a/ and isn't shown", id[:7])
		}
	}
	// Both branches' work joins c1 again: their lanes hold together.
	row, _ := g.RowOf(c1)
	if rows := g.Rows(row, 1); rows[0].Parents != 0 {
		t.Errorf("c1 has %d parents, want none", rows[0].Parents)
	}
	// The count runs beside the load, so it can land a moment after the rows.
	var on bool
	var total int
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if on, _, total = r.Filtered(); total == 3 {
			break
		}
	}
	if on || total != 3 {
		t.Errorf("filtered = %v, total %d; a path is a focus, not a filter, and counts 3", on, total)
	}

	// With a branch focus: that branch's history, under the path.
	r.SetFocus(Focus{Branch: "side", Path: "a/"})
	waitGraph(t, r, 2)
	g, _ = r.Graph()
	if _, ok := g.RowOf(m1); ok {
		t.Error("main's commit shows under a focus on side")
	}
}

// #1061: under a path focus, a branch whose tip touched the path is labelled as usual; one
// whose tip didn't is labelled where its history last did, with how far its tip is past it;
// one that never touched it has no label.
func TestPathFocusLabelsBranchesWhereTheyLastTouchedIt(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	c1 := fx.CommitFile("a/x.txt", "x\n", "c1: a")
	fx.CommitFile("b/y.txt", "y\n", "c2: b")
	fx.Branch("touch")
	t1 := fx.CommitFile("a/t.txt", "t\n", "t1: a, on touch")
	fx.Checkout("main")
	fx.Branch("away")
	fx.CommitFile("b/q.txt", "q\n", "q1: b, on away")
	fx.Git("checkout", "-q", "--orphan", "never")
	fx.Git("rm", "-r", "-q", "-f", ".")
	fx.CommitFile("c/n.txt", "n\n", "n1: c, on never")
	fx.Checkout("main")
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 5)

	r.SetFocus(Focus{Path: "a/"})
	waitGraph(t, r, 2)
	var labels map[string][]git.Ref
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && labels == nil; time.Sleep(20 * time.Millisecond) {
		labels = r.PathLabels()
	}
	at := map[string]int{}
	for id, list := range labels {
		for _, ref := range list {
			if ref.Kind == git.LocalBranch {
				at[ref.Short] = ref.Later
				if id != c1 {
					t.Errorf("%s is labelled at %s, want c1", ref.Short, id[:7])
				}
			}
		}
	}
	// main is 1 past c1 (c2); away is 2 past it (c2, q1).
	if len(at) != 2 || at["main"] != 1 || at["away"] != 2 {
		t.Errorf("labels = %v, want main 1 and away 2 at c1", at)
	}
	// touch's own tip is in the history, labelled as any ref is.
	if refs := r.Refs().ByCommit[t1]; len(refs) != 1 || refs[0].Short != "touch" || refs[0].Later != 0 {
		t.Errorf("touch's tip: %v", refs)
	}

	// Without the path focus, no labels.
	r.SetFocus(Focus{})
	waitGraph(t, r, 5)
	if got := r.PathLabels(); got != nil {
		t.Errorf("labels without a path: %v", got)
	}
}

// #1062: focused on a file whose oldest commit here renamed it, the loader says what it was
// called; on a folder, or a file added under its own name, it says nothing.
func TestPathFocusFindsARenameAtTheFilesOldestCommit(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	body := strings.Repeat("send an alert when the gust is over the limit\n", 8)
	fx.CommitFile("lib/sms.js", body, "add sms")
	fx.Git("mv", "lib/sms.js", "lib/texts.js")
	fx.Commit("call it texts")
	fx.CommitFile("lib/texts.js", body+"and log it\n", "log it")
	fx.CommitFile("lib/gusts.js", "gusts\n", "add gusts")
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 4)
	renamed := func() string {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if old := r.GraphRenamedFrom(); old != "" {
				return old
			}
		}
		return ""
	}

	r.SetFocus(Focus{Path: "lib/texts.js"})
	waitGraph(t, r, 2)
	if got := renamed(); got != "lib/sms.js" {
		t.Errorf("focused on the renamed file: %q, want lib/sms.js", got)
	}
	for path, rows := range map[string]int{"lib/gusts.js": 1, "lib/": 4} {
		r.SetFocus(Focus{Path: path})
		waitGraph(t, r, rows)
		// The whole history's count arrives beside the load as the rename would: once it has,
		// so would a rename.
		for deadline := time.Now().Add(10 * time.Second); r.GraphOutOf() == 0; time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("focused on %s, the whole history was never counted", path)
			}
		}
		time.Sleep(200 * time.Millisecond)
		if got := r.GraphRenamedFrom(); got != "" {
			t.Errorf("focused on %s: %q", path, got)
		}
	}
}

// #1061: a path focus's labels wait for its rows, and outlive its load. When the next focus
// replaced a load that had finished, they waited for the old graph's rows for ever, and
// closing the repository waited for them (a CI run timed out at ten minutes).
func TestPathFocusLabelsEndWithTheirLoad(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	fx.CommitFile("a/x.txt", "x\n", "c1: a")
	fx.Branch("side")
	fx.CommitFile("b/y.txt", "y\n", "s1: b")
	fx.Checkout("main")
	fx.CommitFile("b/z.txt", "z\n", "c2: b")
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitGraph(t, r, 3)
	for i := range 10 {
		if i%2 == 0 {
			r.SetFocus(Focus{Path: "a/"})
		} else {
			r.SetFocus(Focus{Branch: "side", Path: "a/"})
		}
		waitGraph(t, r, 1)
	}
	closed := make(chan struct{})
	go func() {
		m.CloseAll()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(20 * time.Second):
		t.Fatal("closing the repository waited on a path focus's labels")
	}
}
