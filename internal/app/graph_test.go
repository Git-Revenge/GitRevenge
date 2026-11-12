package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestGraphRows(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.WithMergeHistory(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var page GraphPage
	deadline := time.Now().Add(10 * time.Second)
	for {
		if page, err = a.GraphRows(tab.Path, 0, 50); err != nil {
			t.Fatal(err)
		}
		if !page.Loading || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if page.Total != 6 || len(page.Rows) != 6 || page.Rows[0].Subject == "" {
		t.Fatalf("page = total %d, rows %d, loading %v", page.Total, len(page.Rows), page.Loading)
	}
	if page, _ := a.GraphRows(tab.Path, 4, 50); len(page.Rows) != 2 || page.Rows[0].Index != 4 {
		t.Fatalf("offset page = %+v", page.Rows)
	}
	if _, err := a.GraphRows("C:/not/open", 0, 10); err == nil {
		t.Fatal("expected error for a repo that isn't open")
	}
}

func TestGraphRefsAndCommitRow(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.WithMergeHistory(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var page GraphPage
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if page, _ = a.GraphRows(tab.Path, 0, 50); !page.Loading {
			break
		}
	}
	head := fx.Head()
	labels := page.Refs[head]
	if len(labels) != 1 || labels[0].Short != "main" || !labels[0].Head {
		t.Fatalf("labels on HEAD = %+v", labels)
	}
	refs, err := a.Refs(tab.Path)
	if err != nil || len(refs) != 4 { // main, topic, feature, v0.1.0
		t.Fatalf("refs = %+v, %v", refs, err)
	}
	if row, _ := a.CommitRow(tab.Path, head); row != 0 {
		t.Fatalf("CommitRow(HEAD) = %d, want 0", row)
	}
	if row, _ := a.CommitRow(tab.Path, "0000000000000000000000000000000000000000"); row != -1 {
		t.Fatalf("unknown commit row = %d", row)
	}
}

func TestSearchGraph(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.WithMergeHistory(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if page, _ := a.GraphRows(tab.Path, 0, 1); !page.Loading {
			break
		}
	}
	res, err := a.SearchGraph(tab.Path, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 { // "feature 1", "feature 2", "Merge branch 'feature'"
		t.Fatalf("res = %+v", res)
	}
}

func TestSearchGraphByTrailer(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(2)
	paired := fx.CommitFile("a.txt", "a", "Pair work\n\nCo-authored-by: Ada <ada@example.com>")
	fx.CommitFile("b.txt", "b", "fix: the crash\n\nCo-authored-by: Bo <bo@example.com>")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := a.SearchGraph(tab.Path, "co-authored-by: ada")
	if err != nil {
		t.Fatal(err)
	}
	row, _ := a.CommitRow(tab.Path, paired)
	if res.Total != 1 || res.Rows[0] != row {
		t.Fatalf("trailer search = %+v, want only row %d", res, row)
	}
	// A conventional-commit prefix is a text search, not a trailer key.
	if res, _ := a.SearchGraph(tab.Path, "fix: the"); res.Total != 1 {
		t.Fatalf("text search for %q = %+v", "fix: the", res)
	}
}

func TestSearchesCancelThePreviousOne(t *testing.T) {
	t.Parallel()
	var s searches
	first, doneFirst := s.start("/r")
	second, doneSecond := s.start("/r")
	if first.Err() == nil {
		t.Fatal("starting a second search didn't cancel the first")
	}
	other, doneOther := s.start("/other")
	doneFirst() // finishing the superseded search must not drop the newer one
	if second.Err() != nil || other.Err() != nil {
		t.Fatal("a search was cancelled by another repo's or an older one's end")
	}
	doneSecond()
	doneOther()
	if len(s.running) != 0 {
		t.Fatalf("searches left running: %v", s.running)
	}
}

func TestCommitSignaturesCachesUnsigned(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(2)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 { // the second call is served from the cache
		got, err := a.CommitSignatures(tab.Path, shas)
		if err != nil || len(got) != 2 || got[0].Status != "none" || got[1].Status != "none" {
			t.Fatalf("signatures = %+v, %v", got, err)
		}
	}
	if n := len(a.sigs.byRe[tab.Path]); n != 2 {
		t.Fatalf("cache holds %d entries, want 2", n)
	}
}

func TestWorktreeBindings(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	fx.Git("branch", "existing")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	if err := a.AddWorktree(tab.Path, filepath.Join(base, "a"), "existing"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddWorktree(tab.Path, filepath.Join(base, "b"), "brand-new"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddWorktree(tab.Path, "relative/path", "x"); err == nil {
		t.Fatal("a relative path was accepted")
	}
	// The same branch can't be checked out twice; git's reason comes through.
	if err := a.AddWorktree(tab.Path, filepath.Join(base, "c"), "existing"); err == nil || !strings.Contains(err.Error(), "existing") {
		t.Fatalf("second checkout of a branch = %v", err)
	}
	list, err := a.Worktrees(tab.Path)
	if err != nil || len(list) != 3 || list[1].Branch != "existing" || list[2].Branch != "brand-new" {
		t.Fatalf("worktrees = %+v, %v", list, err)
	}
	if _, err := a.RemoveWorktree(tab.Path, list[2].Path, false); err != nil {
		t.Fatal(err)
	}
}

func TestSearchGraphForNotes(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(3)
	fx.Git("notes", "add", "-m", "deployed", shas[0])
	fx.Git("notes", "--ref=review", "add", "-m", "looks good", shas[2])
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	noted, err := a.NotedCommits(tab.Path)
	if err != nil || len(noted) != 2 {
		t.Fatalf("NotedCommits = %v, %v", noted, err)
	}
	for _, q := range []string{"has:note", " HAS:Notes "} {
		res, err := a.SearchGraph(tab.Path, q)
		if err != nil {
			t.Fatal(err)
		}
		first, _ := a.CommitRow(tab.Path, shas[0])
		last, _ := a.CommitRow(tab.Path, shas[2])
		if res.Total != 2 || !slices.Contains(res.Rows, first) || !slices.Contains(res.Rows, last) {
			t.Fatalf("search %q = %+v, want rows %d and %d", q, res, first, last)
		}
	}
	// Anything else that starts the same is a text search.
	if res, _ := a.SearchGraph(tab.Path, "has:nothing"); res.Total != 0 {
		t.Fatalf("text search = %+v", res)
	}
}

func TestLaneNamesSayWhichBranchALaneIs(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(3)
	fx.Git("checkout", "-q", "-b", "topic", shas[1])
	fx.CommitFile("t.txt", "t", "topic work")
	fx.Git("checkout", "-q", "main")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var page GraphPage
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if page, _ = a.GraphRows(tab.Path, 0, 10); !page.Loading {
			break
		}
	}
	colors := map[string]int{}
	for _, row := range page.Rows {
		colors[row.Subject] = row.Color
	}
	names, err := a.LaneNames(tab.Path, []int{colors["topic work"], colors["commit 2"], 987})
	if err != nil || len(names) != 3 {
		t.Fatalf("names = %+v, %v", names, err)
	}
	short := func(n LaneName) string {
		if len(n.Refs) == 0 {
			return ""
		}
		return n.Refs[0].Short
	}
	if short(names[0]) != "topic" || short(names[1]) != "main" {
		t.Fatalf("lanes named %q and %q, want topic and main", short(names[0]), short(names[1]))
	}
	if names[2].Row != -1 || names[2].Refs == nil || len(names[2].Refs) != 0 {
		t.Fatalf("unknown lane = %+v", names[2])
	}
	if none, err := a.LaneNames(fx.Dir+"-elsewhere", []int{0}); err == nil || none == nil {
		t.Fatalf("not open: %+v, %v", none, err)
	}
}

func TestLaneNamesGiveAMergedLaneItsMergesSubject(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.WithMergeHistory(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var page GraphPage
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if page, _ = a.GraphRows(tab.Path, 0, 50); !page.Loading {
			break
		}
	}
	// The lane a merge opens: the colour of a line that leaves a two-parent commit for another lane.
	merged, subject := -1, ""
	for _, row := range page.Rows {
		if row.Parents < 2 {
			continue
		}
		for _, s := range row.Down {
			if s.From != s.To && s.Color != row.Color {
				merged, subject = s.Color, row.Subject
			}
		}
	}
	if merged < 0 {
		t.Skip("the fixture's merged branch still has its name: no lane was opened by a merge")
	}
	names, err := a.LaneNames(tab.Path, []int{merged})
	if err != nil || len(names) != 1 {
		t.Fatalf("names = %+v, %v", names, err)
	}
	if names[0].Merge != subject || len(names[0].Refs) != 0 {
		t.Fatalf("merged lane = %+v, want the merge's subject %q and no refs", names[0], subject)
	}
}
