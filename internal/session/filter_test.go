package session

import (
	"context"
	"testing"

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
