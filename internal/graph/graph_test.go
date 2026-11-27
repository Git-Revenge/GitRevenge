package graph

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// fakeID makes a deterministic 40-hex-digit ID from n.
func fakeID(n int) string { return fmt.Sprintf("%040x", n+1) }

// build lays out commits given as child→parents in topological order.
func build(t *testing.T, commits [][]int) *Graph {
	t.Helper()
	g := New()
	for i, ps := range commits {
		c := git.LogCommit{ID: fakeID(i), Subject: fmt.Sprint(i), AuthorName: "a"}
		for _, p := range ps {
			c.Parents = append(c.Parents, fakeID(p))
		}
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func cols(rows []Row) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = r.Col
	}
	return out
}

func TestLinearStaysInOneLane(t *testing.T) {
	g := build(t, [][]int{{1}, {2}, {3}, {}})
	rows := g.Rows(0, 10)
	if !slices.Equal(cols(rows), []int{0, 0, 0, 0}) {
		t.Fatalf("cols = %v", cols(rows))
	}
	for _, r := range rows {
		if r.Color != rows[0].Color || r.Width != 1 {
			t.Fatalf("row %d: color %d width %d", r.Index, r.Color, r.Width)
		}
	}
	if len(rows[3].Down) != 0 {
		t.Fatal("root commit should have no lines below it")
	}
}

func TestBranchAndMerge(t *testing.T) {
	// 0: merge of 1 and 2
	// 1: main work      -> 3
	// 2: feature work   -> 3
	// 3: base
	g := build(t, [][]int{{1, 2}, {3}, {3}, {}})
	rows := g.Rows(0, 4)
	if !slices.Equal(cols(rows), []int{0, 0, 1, 0}) {
		t.Fatalf("cols = %v", cols(rows))
	}
	if !slices.Equal(rows[0].Down, []Seg{{From: 0, To: 0, Color: rows[0].Color}, {From: 0, To: 1, Color: rows[2].Color}}) {
		t.Fatalf("merge row down = %+v", rows[0].Down)
	}
	if rows[2].Color == rows[0].Color {
		t.Fatal("feature lane should get its own color")
	}
	// The feature commit's parent is already awaited by lane 0, so its line joins that lane
	// right away instead of running a parallel lane down to the base.
	if !slices.Equal(rows[2].Down, []Seg{{From: 1, To: 0, Color: rows[2].Color}, {From: 0, To: 0, Color: rows[0].Color}}) {
		t.Fatalf("feature row down = %+v", rows[2].Down)
	}
	if len(rows[3].Up) != 1 || rows[3].Width != 1 {
		t.Fatalf("base row should have a single lane: %+v", rows[3])
	}
}

// #1006: a shallow clone's edge commits are listed with no parents. Given the ones they have
// on the server, their lanes run on to the end, as EdgeKind, instead of ending as if the
// history began there.
func TestShallowEdgeLanesRunOn(t *testing.T) {
	// 0 -> 1, 1 at the edge (its parent 100 not fetched); 2 a tip -> 3, 3 a merge at the
	// edge (parents 101 and 102 not fetched). git lists 1 and 3 with no parents.
	commits := [][]int{{1}, {}, {3}, {}}
	g := New()
	g.SetTipKinds(map[string]string{fakeID(2): "docs"})
	g.SetEdge(map[string][]string{fakeID(1): {fakeID(100)}, fakeID(3): {fakeID(101), fakeID(102)}})
	for i, ps := range commits {
		c := git.LogCommit{ID: fakeID(i), Subject: fmt.Sprint(i), AuthorName: "a"}
		for _, p := range ps {
			c.Parents = append(c.Parents, fakeID(p))
		}
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	// An edge commit says so, and names the parent its object does.
	if cs := g.Commits(1, 1); len(cs) != 1 || !cs[0].Edge || cs[0].First != fakeID(100) {
		t.Fatalf("edge commit = %+v, want Edge and its parent 100", cs)
	}
	if cs := g.Commits(0, 1); cs[0].Edge {
		t.Fatalf("commit 0 isn't at the edge: %+v", cs[0])
	}
	rows := g.Rows(0, 4)
	kinds := func(segs []Seg) (out []string) {
		for _, s := range segs {
			out = append(out, s.Kind)
		}
		return out
	}
	if !rows[1].Edge || rows[1].Parents != 1 || !slices.Equal(kinds(rows[1].Down), []string{EdgeKind}) {
		t.Fatalf("edge row 1: edge %v, parents %d, down %+v", rows[1].Edge, rows[1].Parents, rows[1].Down)
	}
	if rows[0].Edge || rows[2].Edge || rows[2].Kind != "docs" {
		t.Fatalf("rows 0 and 2: %+v %+v", rows[0], rows[2])
	}
	if !rows[3].Edge || rows[3].Parents != 2 {
		t.Fatalf("edge merge row 3: %+v", rows[3])
	}
	// At the end, all three lanes run on: 1's parent and both of 3's.
	if got := kinds(rows[3].Down); len(got) != 3 || slices.ContainsFunc(got, func(k string) bool { return k != EdgeKind }) {
		t.Fatalf("last row down = %+v; want three lanes running on", rows[3].Down)
	}
	// Paged rows agree.
	if page := g.Page(3, 1); len(page[0].Down) != 2 || page[0].PassKind[0] != EdgeKind {
		t.Fatalf("paged last row: %+v", page[0])
	}

	// Without the edge the same commits end where git lists them.
	plain := build(t, commits)
	if r := plain.Rows(0, 4); len(r[1].Down) != 0 || r[1].Edge {
		t.Fatalf("no edge: row 1 = %+v", r[1])
	}
}

func TestTwoTipsOpenTwoLanes(t *testing.T) {
	// 0: tip A -> 2, 1: tip B -> 2, 2: root
	g := build(t, [][]int{{2}, {2}, {}})
	rows := g.Rows(0, 3)
	if !slices.Equal(cols(rows), []int{0, 1, 0}) {
		t.Fatalf("cols = %v", cols(rows))
	}
	if len(rows[1].Up) != 1 || rows[1].Up[0].From != 0 {
		t.Fatalf("tip B should have lane 0 passing beside it: %+v", rows[1].Up)
	}
}

func TestFreedColumnIsReused(t *testing.T) {
	// 0 -> 1,2 (merge); 2 -> root 4; 1 -> 3; 3 -> 4; 4 root. Then a new tip later reuses col 1.
	g := build(t, [][]int{{1, 2}, {3}, {4}, {4}, {}})
	if w := g.Rows(4, 1)[0].Width; w > 2 {
		t.Fatalf("width at base = %d", w)
	}
}

// randomDAG produces n commits in a valid topological order with merges and many tips.
func randomDAG(n int, seed uint64) [][]int {
	r := rand.New(rand.NewPCG(seed, seed))
	commits := make([][]int, n)
	for i := range n - 1 {
		// Parents come later in the list.
		p1 := i + 1 + r.IntN(min(8, n-1-i))
		ps := []int{p1}
		if r.IntN(5) == 0 && p1+1 < n {
			p2 := p1 + 1 + r.IntN(min(20, n-1-p1))
			if p2 < n && p2 != p1 {
				ps = append(ps, p2)
			}
		}
		commits[i] = ps
	}
	return commits
}

// Lines must join up across rows: every line entering the top of a row continues an
// identical line (column and color) leaving the row above, and every line leaving a row
// lands on a column that continues in the next row. (A line joining an existing lane keeps
// its own color until it meets that lane, so only columns are compared in that direction.)
func TestRowsConnect(t *testing.T) {
	for seed := uint64(1); seed <= 5; seed++ {
		g := build(t, randomDAG(1000, seed))
		rows := g.Rows(0, g.Len())
		for i := 0; i+1 < len(rows); i++ {
			down := map[[2]int]bool{}
			downCols := map[int]bool{}
			for _, s := range rows[i].Down {
				down[[2]int{s.To, s.Color}] = true
				downCols[s.To] = true
			}
			upCols := map[int]bool{}
			for _, s := range rows[i+1].Up {
				upCols[s.From] = true
				if !down[[2]int{s.From, s.Color}] {
					t.Fatalf("seed %d row %d: line from col %d color %d has no source in row %d", seed, i+1, s.From, s.Color, i)
				}
			}
			for c := range downCols {
				if !upCols[c] {
					t.Fatalf("seed %d row %d: line to col %d does not continue into row %d", seed, i, c, i+1)
				}
			}
		}
		// The last row is a root: nothing leaves it.
		if last := rows[len(rows)-1]; len(last.Down) != 0 {
			t.Fatalf("seed %d: lines below the final root: %+v", seed, last.Down)
		}
	}
}

// A window replayed from a checkpoint must match the same rows from a full pass.
func TestWindowsMatchFullLayout(t *testing.T) {
	g := build(t, randomDAG(1000, 2))
	full := g.Rows(0, g.Len())
	for _, start := range []int{0, 1, 127, 128, 129, 500, 990} {
		win := g.Rows(start, 20)
		for i, r := range win {
			want := full[start+i]
			if r.Col != want.Col || r.Color != want.Color || !slices.Equal(r.Up, want.Up) || !slices.Equal(r.Down, want.Down) {
				t.Fatalf("row %d differs when read from window at %d", start+i, start)
			}
		}
	}
	if got := g.Rows(995, 100); len(got) != 5 {
		t.Fatalf("window past the end returned %d rows", len(got))
	}
}

// A graph wider than wideLanes snapshots less often; windows read anywhere in it still match
// the full layout.
func TestWideGraphSpacesCheckpoints(t *testing.T) {
	const tips = 2*wideLanes + 100 // every tip's lane stays open until its parent arrives
	commits := make([][]int, 0, 2*tips+200)
	for i := range tips {
		commits = append(commits, []int{tips + i})
	}
	for range tips {
		commits = append(commits, []int{2 * tips}) // parents all lead to one root line
	}
	for i := range 199 {
		commits = append(commits, []int{2*tips + i + 1})
	}
	commits = append(commits, []int{})
	g := build(t, commits)

	if width := g.Rows(tips, 1)[0].Width; width <= 2*wideLanes {
		t.Fatalf("width %d, want the fixture wider than %d", width, 2*wideLanes)
	}
	if n, dense := len(g.checkpoints), (g.Len()+checkpointEvery-1)/checkpointEvery; n >= dense {
		t.Fatalf("%d checkpoints, want fewer than the %d a narrow graph takes", n, dense)
	}

	full := g.Rows(0, g.Len())
	for _, start := range []int{0, 127, 128, 400, tips - 1, tips, tips + 300, 2*tips - 1, g.Len() - 5} {
		for i, r := range g.Rows(start, 20) {
			want := full[start+i]
			if r.Col != want.Col || r.Color != want.Color || !slices.Equal(r.Up, want.Up) || !slices.Equal(r.Down, want.Down) {
				t.Fatalf("row %d differs when read from window at %d", start+i, start)
			}
		}
	}
}

func TestCommits(t *testing.T) {
	// A merge (row 0) of two lines that meet at a root.
	g := build(t, [][]int{{1, 2}, {3}, {3}, {}})
	got := g.Commits(-1, 3)
	want := []Commit{
		{Index: 0, ID: fakeID(0), Parents: 2, Author: "a", Subject: "0", First: fakeID(1)},
		{Index: 1, ID: fakeID(1), Parents: 1, Author: "a", Subject: "1", First: fakeID(3)},
		{Index: 2, ID: fakeID(2), Parents: 1, Author: "a", Subject: "2", First: fakeID(3)},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Commits(-1, 3) = %+v, want %+v", got, want)
	}
	if last := g.Commits(3, 10); len(last) != 1 || last[0].Parents != 0 {
		t.Fatalf("Commits(3, 10) = %+v, want the root alone", last)
	}
	if past := g.Commits(9, 2); past == nil || len(past) != 0 {
		t.Fatalf("Commits past the end = %#v, want an empty list", past)
	}
}

func TestRowOf(t *testing.T) {
	g := build(t, [][]int{{1}, {}})
	if r, ok := g.RowOf(fakeID(1)); !ok || r != 1 {
		t.Fatalf("RowOf = %d, %v", r, ok)
	}
	if _, ok := g.RowOf(fakeID(7)); ok {
		t.Fatal("unknown commit found")
	}
}

func TestFromRealRepo(t *testing.T) {
	bin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	fx := gittest.WithMergeHistory(t)
	repo := git.OpenRepo(git.NewRunner(bin), fx.Dir)
	g := New()
	if err := repo.Log(context.Background(), git.LogOptions{}, g.Add); err != nil {
		t.Fatal(err)
	}
	rows := g.Rows(0, 100)
	if len(rows) != 6 || rows[0].ID != fx.Git("rev-parse", rows[0].ID) {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Author != "Test User" || rows[0].AuthorTime == 0 {
		t.Fatalf("row 0 = %+v", rows[0])
	}
}

func BenchmarkAdd(b *testing.B) {
	commits := randomDAG(100_000, 3)
	logs := make([]git.LogCommit, len(commits))
	for i, ps := range commits {
		logs[i] = git.LogCommit{ID: fakeID(i), AuthorName: "a", Subject: "s"}
		for _, p := range ps {
			logs[i].Parents = append(logs[i].Parents, fakeID(p))
		}
	}
	b.ResetTimer()
	for b.Loop() {
		g := New()
		for _, c := range logs {
			_ = g.Add(c)
		}
	}
}

func BenchmarkRowsWindow(b *testing.B) {
	g := New()
	for i, ps := range randomDAG(100_000, 4) {
		c := git.LogCommit{ID: fakeID(i), AuthorName: "a", Subject: "s"}
		for _, p := range ps {
			c.Parents = append(c.Parents, fakeID(p))
		}
		_ = g.Add(c)
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		g.Rows((i*7919)%99_000, 100)
	}
}

func TestRowsJSONHasNoNullSegments(t *testing.T) {
	g := build(t, [][]int{{1}, {}})
	b, err := json.Marshal(g.Rows(0, 2))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("rows JSON contains null: %s", b)
	}
}

// buildPinned lays out commits like build, with the given commit indexes pinned.
func buildPinned(t *testing.T, commits [][]int, pins ...int) *Graph {
	t.Helper()
	g := New()
	var ids []string
	for _, p := range pins {
		ids = append(ids, fakeID(p))
	}
	g.Pin(ids)
	for i, ps := range commits {
		c := git.LogCommit{ID: fakeID(i), Subject: fmt.Sprint(i), AuthorName: "a"}
		for _, p := range ps {
			c.Parents = append(c.Parents, fakeID(p))
		}
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func TestPinnedTrunkHoldsColumnZero(t *testing.T) {
	// 0: feature tip (newest) → 2; 1: main tip → 2; 2 → 3; 3: root.
	// Unpinned, the feature lane would take column 0 and main would sit to its right.
	commits := [][]int{{2}, {2}, {3}, {}}
	plain := build(t, commits)
	if c := cols(plain.Rows(0, 4)); c[1] == 0 {
		t.Fatalf("fixture should put main off column 0 when unpinned: %v", c)
	}
	g := buildPinned(t, commits, 1)
	rows := g.Rows(0, 4)
	got := cols(rows)
	if got[0] == 0 || got[1] != 0 || got[2] != 0 || got[3] != 0 {
		t.Fatalf("cols = %v, want main (rows 1-3) in column 0 and the feature to its right", got)
	}
	if rows[1].Color != 0 || rows[2].Color != 0 {
		t.Fatalf("main's colour = %d, %d, want 0", rows[1].Color, rows[2].Color)
	}
	if rows[0].Color == 0 {
		t.Fatal("the feature lane shouldn't share the Trunk colour")
	}
}

func TestPinnedTipReachedFromAbove(t *testing.T) {
	// 0: a branch whose parent is main's tip (1); main continues 1 → 2 → 3.
	g := buildPinned(t, [][]int{{1}, {2}, {3}, {}}, 1)
	got := cols(g.Rows(0, 4))
	if got[1] != 0 || got[2] != 0 || got[3] != 0 || got[0] == 0 {
		t.Fatalf("cols = %v, want main in column 0 and the branch above it to the right", got)
	}
}

func TestTwoPinsAndMerges(t *testing.T) {
	// develop (0) merges feature (2) and continues to main's tip (1); main 1 → 3 → 4.
	// 0: develop [1, 2]; 1: main [3]; 2: feature [3]; 3 [4]; 4 root.
	g := buildPinned(t, [][]int{{1, 2}, {3}, {3}, {4}, {}}, 1, 0)
	got := cols(g.Rows(0, 5))
	if got[1] != 0 || got[3] != 0 || got[4] != 0 {
		t.Fatalf("cols = %v, want main in column 0", got)
	}
	if got[0] != 1 {
		t.Fatalf("cols = %v, want develop in column 1", got)
	}
	if got[2] < 2 {
		t.Fatalf("cols = %v, want the feature right of both Trunk columns", got)
	}
}

func TestPinnedWindowsMatchFullLayout(t *testing.T) {
	commits := randomDAG(1500, 3)
	g := buildPinned(t, commits, 5, 40)
	full := g.Rows(0, g.Len())
	for _, start := range []int{0, 1, 127, 128, 129, 700, 1490} {
		for i, r := range g.Rows(start, 20) {
			want := full[start+i]
			if r.Col != want.Col || r.Color != want.Color || !slices.Equal(r.Up, want.Up) || !slices.Equal(r.Down, want.Down) {
				t.Fatalf("row %d differs when read from window at %d", start+i, start)
			}
		}
	}
	// Every row's segments connect: what leaves a row downward enters the next from above.
	for i := 0; i+1 < len(full); i++ {
		down := map[int]bool{}
		for _, s := range full[i].Down {
			down[s.To] = true
		}
		for _, s := range full[i+1].Up {
			if !down[s.From] {
				t.Fatalf("row %d: line from column %d has nothing above it", i+1, s.From)
			}
		}
	}
}

func TestTipKindsFollowLanes(t *testing.T) {
	// 0: docs tip (orphan line 0 → 2 → 4); 1: main tip → 3 → 5 (root); 2, 4 docs history.
	// 1 also merges a side line 6 (no kind) whose parent is 3.
	commits := [][]int{{2}, {3, 6}, {4}, {5}, {}, {}, {3}}
	g := New()
	g.SetTipKinds(map[string]string{fakeID(0): "docs", fakeID(1): "trunk"})
	for i, ps := range commits {
		c := git.LogCommit{ID: fakeID(i), Subject: fmt.Sprint(i), AuthorName: "a"}
		for _, p := range ps {
			c.Parents = append(c.Parents, fakeID(p))
		}
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	rows := g.Rows(0, len(commits))
	want := map[int]string{0: "docs", 2: "docs", 4: "docs", 1: "trunk", 3: "trunk", 5: "trunk", 6: ""}
	for i, k := range want {
		if rows[i].Kind != k {
			t.Errorf("row %d kind = %q, want %q", i, rows[i].Kind, k)
		}
	}
	for _, s := range rows[1].Down {
		if s.From == rows[1].Col && s.To != rows[1].Col && s.Kind != "" {
			t.Errorf("the merged side line should carry no kind: %+v", s)
		}
	}
}

// kindsOf lays out commits (parents by index) and returns each row's kind, by commit.
func kindsOf(t *testing.T, commits [][]int, pins []int, kinds map[int]string) map[int]string {
	t.Helper()
	g := New()
	var pinned []string
	for _, i := range pins {
		pinned = append(pinned, fakeID(i))
	}
	g.Pin(pinned)
	tips := map[string]string{}
	for i, k := range kinds {
		tips[fakeID(i)] = k
	}
	g.SetTipKinds(tips)
	for i, ps := range commits {
		c := git.LogCommit{ID: fakeID(i), Subject: fmt.Sprint(i), AuthorName: "a"}
		for _, p := range ps {
			c.Parents = append(c.Parents, fakeID(p))
		}
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	out := map[int]string{}
	for i, r := range g.Rows(0, len(commits)) {
		out[i] = r.Kind
	}
	return out
}

func TestKindedTipOnTrunkLeavesTrunkItsKind(t *testing.T) {
	// main: 0 → 1 → 2 → 3 → 4, with a release tip on 1 and a docs tip on 3.
	got := kindsOf(t, [][]int{{1}, {2}, {3}, {4}, {}}, []int{0}, map[int]string{0: "trunk", 1: "release", 3: "docs"})
	for i, k := range got {
		if k != "trunk" {
			t.Errorf("row %d kind = %q, want trunk", i, k)
		}
	}
}

func TestKindArrivingBeforeTrunkStopsAtTrunk(t *testing.T) {
	// A release tip ahead of main: release → main's tip → main's history.
	got := kindsOf(t, [][]int{{1}, {2}, {}}, []int{1}, map[int]string{0: "release", 1: "trunk"})
	want := map[int]string{0: "release", 1: "trunk", 2: "trunk"}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("row %d kind = %q, want %q", i, got[i], k)
		}
	}
}

func TestKindedTipOffTrunkStillStartsItsKind(t *testing.T) {
	// A feature branch off a release branch: the release's commits stay the release's.
	got := kindsOf(t, [][]int{{1}, {2}, {}}, nil, map[int]string{0: "feature", 1: "release"})
	want := map[int]string{0: "feature", 1: "release", 2: "release"}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("row %d kind = %q, want %q", i, got[i], k)
		}
	}
}

func TestTipKindsSurviveCheckpoints(t *testing.T) {
	commits := randomDAG(1200, 3)
	g := New()
	kinds := map[string]string{}
	for i := 0; i < len(commits); i += 37 {
		kinds[fakeID(i)] = []string{"docs", "bot", "release"}[i%3]
	}
	g.SetTipKinds(kinds)
	for i, ps := range commits {
		c := git.LogCommit{ID: fakeID(i), Subject: fmt.Sprint(i), AuthorName: "a"}
		for _, p := range ps {
			c.Parents = append(c.Parents, fakeID(p))
		}
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	full := g.Rows(0, g.Len())
	seen := false
	for _, r := range full {
		seen = seen || r.Kind != ""
	}
	if !seen {
		t.Fatal("no row got a kind; the fixture should have kinded tips")
	}
	for _, start := range []int{1, 127, 128, 300, 1190} {
		for i, r := range g.Rows(start, 15) {
			w := full[start+i]
			if r.Kind != w.Kind || r.Color != w.Color || !slices.Equal(r.Up, w.Up) || !slices.Equal(r.Down, w.Down) {
				t.Fatalf("row %d differs when read from window at %d", start+i, start)
			}
		}
	}
}

// Each lane says which commit started it: a tip nothing above points to, or a merge.
func TestLaneStartNamesTheCommitThatStartedALane(t *testing.T) {
	// 0 is a tip; 1 a second tip; 2 merges 4 into its line; 3 and 4 are older.
	//   0 → 3, 1 → 3, 3 → 5 is skipped: keep it small.
	g := build(t, [][]int{{2}, {2}, {3, 4}, {}, {}})
	rows := g.Rows(0, 10)
	// Row 0's lane starts at row 0, and is its own tip.
	if row, tip, ok := g.LaneStart(rows[0].Color); !ok || row != 0 || !tip {
		t.Fatalf("lane of row 0 starts at %d (tip %v, ok %v)", row, tip, ok)
	}
	// Row 1 is a second tip on its own lane.
	if rows[1].Color == rows[0].Color {
		t.Fatalf("two tips share colour %d", rows[0].Color)
	}
	if row, tip, ok := g.LaneStart(rows[1].Color); !ok || row != 1 || !tip {
		t.Fatalf("lane of row 1 starts at %d (tip %v, ok %v)", row, tip, ok)
	}
	// Row 2 continues row 0's lane; its second parent, row 4, sits on a lane the merge opened.
	if rows[2].Color != rows[0].Color || rows[3].Color != rows[0].Color {
		t.Fatalf("first parents changed lane: colours %d %d %d", rows[0].Color, rows[2].Color, rows[3].Color)
	}
	if row, tip, ok := g.LaneStart(rows[4].Color); !ok || row != 2 || tip {
		t.Fatalf("the merged lane starts at %d (tip %v, ok %v), want the merge at row 2, not a tip", row, tip, ok)
	}
	if _, _, ok := g.LaneStart(99); ok {
		t.Fatal("a colour no lane has was found")
	}
	if g.ID(2) != fakeID(2) || g.ID(99) != "" {
		t.Fatalf("ID(2) = %q, ID(99) = %q", g.ID(2), g.ID(99))
	}
}

// The first Trunk's lane has colour 0 before any row exists; it starts at that Trunk's tip.
func TestLaneStartOfThePinnedTrunk(t *testing.T) {
	// 0 is a feature tip, 1 is main's tip (pinned), both on 2.
	g := buildPinned(t, [][]int{{2}, {2}, {}}, 1)
	rows := g.Rows(0, 10)
	if rows[1].Color != 0 || rows[1].Col != 0 {
		t.Fatalf("pinned tip: colour %d col %d", rows[1].Color, rows[1].Col)
	}
	if row, tip, ok := g.LaneStart(0); !ok || row != 1 || !tip {
		t.Fatalf("trunk lane starts at %d (tip %v, ok %v), want row 1", row, tip, ok)
	}
	if row, tip, ok := g.LaneStart(rows[0].Color); !ok || row != 0 || !tip {
		t.Fatalf("feature lane starts at %d (tip %v, ok %v)", row, tip, ok)
	}
}

// packAfter packs rows the long way, from the Segs of rows that weren't packed as they were
// laid out: a lane that is a straight line of one colour and kind in both halves moves to
// Pass. It is what Page's rows are checked against.
func packAfter(rows []Row) {
	var downAt []int // per column, where its straight line is in Down, or -1
	for i := range rows {
		r := &rows[i]
		downAt = downAt[:0]
		for j, s := range r.Down {
			if s.From != s.To {
				continue
			}
			for len(downAt) <= s.From {
				downAt = append(downAt, -1)
			}
			if downAt[s.From] < 0 {
				downAt[s.From] = j
			}
		}
		up := r.Up[:0]
		moved := 0
		last, run := -2, -1 // the column of the lane before, and where its run's length is
		for _, s := range r.Up {
			j := -1
			if s.From == s.To && s.From < len(downAt) {
				j = downAt[s.From]
			}
			if j < 0 || r.Down[j].Color != s.Color || r.Down[j].Kind != s.Kind {
				up = append(up, s)
				continue
			}
			r.Down[j].From = -1 // marks it as moved
			moved++
			if run < 0 || s.From != last+1 || r.PassKind[len(r.PassKind)-1] != s.Kind {
				run = len(r.Pass) + 1
				r.Pass = append(r.Pass, s.From, 0)
				r.PassKind = append(r.PassKind, s.Kind)
			}
			r.Pass[run]++
			r.Pass = append(r.Pass, s.Color)
			last = s.From
		}
		r.Up = up
		if moved == 0 {
			continue
		}
		down := r.Down[:0]
		for _, s := range r.Down {
			if s.From >= 0 {
				down = append(down, s)
			}
		}
		r.Down = down
	}
}

// unpacked puts a packed row's Pass lanes back as the Segs they were, in each half.
func unpacked(t *testing.T, r Row) (up, down []Seg) {
	t.Helper()
	up, down = slices.Clone(r.Up), slices.Clone(r.Down)
	last := -1
	for i, run := 0, 0; i < len(r.Pass); run++ {
		if i+2 > len(r.Pass) || run >= len(r.PassKind) {
			t.Fatalf("row %d: Pass %v doesn't parse with %d kinds", r.Index, r.Pass, len(r.PassKind))
		}
		col, n := r.Pass[i], r.Pass[i+1]
		if n < 1 || i+2+n > len(r.Pass) {
			t.Fatalf("row %d: a run of %d lanes in Pass %v", r.Index, n, r.Pass)
		}
		if col <= last {
			t.Fatalf("row %d: Pass isn't in column order: %v", r.Index, r.Pass)
		}
		for k := range n {
			s := Seg{From: col + k, To: col + k, Color: r.Pass[i+2+k], Kind: r.PassKind[run]}
			up, down = append(up, s), append(down, s)
		}
		last = col + n - 1
		i += 2 + n
	}
	return up, down
}

func sortedSegs(segs []Seg) []Seg {
	out := slices.Clone(segs)
	slices.SortFunc(out, func(a, b Seg) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To), cmp.Compare(a.Color, b.Color), cmp.Compare(a.Kind, b.Kind))
	})
	return out
}

// sameLines checks that packed rows draw the lines plain ones do, and returns how many runs
// of lanes they pack.
func sameLines(t *testing.T, what string, plain, packed []Row) (runs int) {
	t.Helper()
	if len(packed) != len(plain) {
		t.Fatalf("%s: %d rows, want %d", what, len(packed), len(plain))
	}
	for i, want := range plain {
		got := packed[i]
		if got.Up == nil || got.Down == nil {
			t.Fatalf("%s row %d: a packed row has a nil half, which is null to the UI", what, i)
		}
		if got.Col != want.Col || got.Color != want.Color || got.Width != want.Width || got.Kind != want.Kind || got.ID != want.ID {
			t.Fatalf("%s row %d: the commit is %+v, want %+v", what, i, got, want)
		}
		up, down := unpacked(t, got)
		if !slices.Equal(sortedSegs(up), sortedSegs(want.Up)) {
			t.Fatalf("%s row %d up:\n got %v (pass %v %q)\nwant %v", what, i, sortedSegs(up), got.Pass, got.PassKind, sortedSegs(want.Up))
		}
		if !slices.Equal(sortedSegs(down), sortedSegs(want.Down)) {
			t.Fatalf("%s row %d down:\n got %v (pass %v %q)\nwant %v", what, i, sortedSegs(down), got.Pass, got.PassKind, sortedSegs(want.Down))
		}
		runs += len(got.PassKind)
	}
	return runs
}

// packedLikePlain checks Page against Rows over the whole of g, and against packing Rows
// afterwards, which must leave no straight lane behind.
func packedLikePlain(t *testing.T, g *Graph) (runs int) {
	t.Helper()
	plain := g.Rows(0, g.Len())
	after := g.Rows(0, g.Len())
	packAfter(after)
	sameLines(t, "packed afterwards", plain, after)
	for i, r := range after {
		for _, s := range r.Up {
			if s.From == s.To && slices.Contains(r.Down, s) {
				t.Fatalf("row %d: lane %d runs straight through but wasn't packed", i, s.From)
			}
		}
	}
	// A window that starts mid-graph replays from a checkpoint, as scrolling does.
	if g.Len() > 700 {
		sameLines(t, "a window", g.Rows(513, 150), g.Page(513, 150))
	}
	return sameLines(t, "Page", plain, g.Page(0, g.Len()))
}

func TestPageDrawsTheLinesRowsDoes(t *testing.T) {
	for seed, shape := range []struct {
		pins  []int
		kinds map[int]string
	}{
		{},
		// Trunk pins and kinds change which lanes share a colour and a kind across a row, and
		// a pinned lane takes its column from whatever was passing through it.
		{pins: []int{0, 7}, kinds: map[int]string{3: "docs", 11: "release", 40: "bot"}},
		{pins: []int{5, 2, 90}, kinds: map[int]string{0: "feature", 1: "feature", 2: "trunk", 9: "docs"}},
	} {
		g := New()
		var pins []string
		for _, p := range shape.pins {
			pins = append(pins, fakeID(p))
		}
		g.Pin(pins)
		kinds := map[string]string{}
		for c, k := range shape.kinds {
			kinds[fakeID(c)] = k
		}
		g.SetTipKinds(kinds)
		for i, ps := range randomDAG(3000+seed, 4) {
			c := git.LogCommit{ID: fakeID(i), AuthorName: "a", Subject: "s"}
			for _, p := range ps {
				c.Parents = append(c.Parents, fakeID(p))
			}
			if err := g.Add(c); err != nil {
				t.Fatal(err)
			}
		}
		if packedLikePlain(t, g) == 0 {
			t.Fatal("nothing was packed in a graph with lanes passing through")
		}
	}
}

// No history lays out this way, but the layout allows for it: a pinned lane's tip arriving
// while another lane holds its column. That lane's line stops at the commit, and must still
// be drawn down to it.
func TestPageKeepsTheLineOfALaneThatIsReplaced(t *testing.T) {
	var passing, tip, parent oid
	passing[0], tip[0], parent[0] = 1, 2, 3
	cfg := layout{pins: &pinning{tips: map[oid]int{tip: 0}, n: 1}}
	lanes := func() *state {
		return &state{lanes: []lane{{want: passing, color: 4, used: true}, {want: passing, color: 5, used: true}}, nextColor: 6}
	}
	plain, packed := Row{Up: []Seg{}, Down: []Seg{}}, Row{Up: []Seg{}, Down: []Seg{}}
	step(lanes(), &cfg, tip, []oid{parent}, 0, &plain, false)
	step(lanes(), &cfg, tip, []oid{parent}, 0, &packed, true)
	if !slices.Contains(plain.Up, Seg{From: 0, To: 0, Color: 4}) || slices.Contains(plain.Down, Seg{From: 0, To: 0, Color: 4}) {
		t.Fatalf("the replaced lane should stop at the commit: up %v, down %v", plain.Up, plain.Down)
	}
	sameLines(t, "a replaced lane", []Row{plain}, []Row{packed})
}

// wideGraph is w lines of history side by side: every row has w-1 lanes passing through.
func wideGraph(tb testing.TB, n, w int) *Graph {
	tb.Helper()
	g := New()
	for i := range n {
		c := git.LogCommit{ID: fakeID(i), AuthorName: "Some Author", Subject: "A commit subject of ordinary length, about sixty chars"}
		if i+w < n {
			c.Parents = []string{fakeID(i + w)}
		}
		if err := g.Add(c); err != nil {
			tb.Fatal(err)
		}
	}
	return g
}

func TestPageOfAWideGraphIsSmall(t *testing.T) {
	const w = 800
	g := wideGraph(t, 4000, w)
	packedLikePlain(t, g)
	before, err := json.Marshal(g.Rows(1000, 200))
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(g.Page(1000, 200))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "null") {
		t.Fatal("a page's JSON contains null")
	}
	t.Logf("200 rows, %d lanes: %d kB as Segs, %d kB packed", w, len(before)/1000, len(after)/1000)
	if len(after)*8 > len(before) {
		t.Fatalf("packing saved too little: %d bytes from %d", len(after), len(before))
	}
}

// BenchmarkPageJSON is what one 200-row page costs the backend, laid out and encoded, by how
// many lanes wide the history is: as Segs (what was sent before lanes were packed) and packed.
// It reports the size of the JSON, which the UI then has to parse.
func BenchmarkPageJSON(b *testing.B) {
	for _, w := range []int{4, 30, 100, 800, 2000} {
		g := wideGraph(b, 12000, w)
		for _, form := range []struct {
			name string
			rows func(start, count int) []Row
		}{{"segs", g.Rows}, {"packed", g.Page}} {
			b.Run(fmt.Sprintf("lanes=%d/%s", w, form.name), func(b *testing.B) {
				b.ReportAllocs()
				var size int
				for i := 0; b.Loop(); i++ {
					out, err := json.Marshal(form.rows(5000+(i%20)*200, 200))
					if err != nil {
						b.Fatal(err)
					}
					size = len(out)
				}
				b.ReportMetric(float64(size)/1e6, "MB/page")
			})
		}
	}
}
