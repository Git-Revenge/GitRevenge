package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestGraphRows(t *testing.T) {
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
	if err := a.RemoveWorktree(tab.Path, list[2].Path, false); err != nil {
		t.Fatal(err)
	}
}
