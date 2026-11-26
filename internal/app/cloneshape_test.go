package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// shallowSource is a repository with history to clone shallow, over file:// as from a server.
func shallowSource(t *testing.T) string {
	t.Helper()
	src := gittest.New(t)
	src.Linear(4)
	src.Branch("side")
	src.CommitFile("side.txt", "side\n", "side work")
	src.Checkout("main")
	src.Git("config", "uploadpack.allowFilter", "true")
	return "file://" + filepath.ToSlash(src.Dir)
}

// #1006: an edge commit's parent wasn't fetched, so its files are all its own, as a first
// commit's are, and the panel says its parent isn't here.
func TestAShallowEdgeCommitOpens(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	dir, err := a.Clone(shallowSource(t), t.TempDir(), "edge", git.CloneOptions{Depth: 2, Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	tab, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.openRepo(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	edge, _ := r.Git.ReadString(context.Background(), "rev-parse", "HEAD~1")
	tip, _ := r.Git.ReadString(context.Background(), "rev-parse", "HEAD")
	info, err := a.CommitDetail(tab.Path, edge)
	if err != nil {
		t.Fatalf("the edge commit didn't open: %v", err)
	}
	if !info.Edge || len(info.Commit.Parents) != 1 || len(info.Files) != 1 || info.Files[0].Status != "A" {
		t.Fatalf("edge %v, parents %v, files %+v; want its one file as added", info.Edge, info.Commit.Parents, info.Files)
	}
	head, err := a.CommitDetail(tab.Path, tip)
	if err != nil || head.Edge || len(head.Files) != 1 || head.Files[0].Status != "M" {
		t.Fatalf("HEAD: %+v, %v", head, err)
	}
}

func TestThisCloneSaysWhatsMissingAndFetchesIt(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	dir, err := a.Clone(shallowSource(t), t.TempDir(), "shallow", git.CloneOptions{Depth: 1, Partial: true, Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	tab, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.CloneShape(tab.Path)
	if err != nil || s.Shallow == nil || s.Partial == nil || s.OneBranch == nil {
		t.Fatalf("shape %+v, %v; want shallow, partial and one branch", s, err)
	}

	// The remote it all comes from is the clone's.
	if err := a.FetchMoreHistory(tab.Path, git.MoreHistory{All: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.FetchEveryBranch(tab.Path, s.OneBranch.Remote); err != nil {
		t.Fatal(err)
	}
	if s, _ = a.CloneShape(tab.Path); s.Shallow != nil || s.OneBranch != nil || s.Partial == nil {
		t.Fatalf("after fetching: %+v; want only partial left", s)
	}
	var friendly userError
	if err := a.FetchMoreHistory(tab.Path, git.MoreHistory{All: true}); !errors.As(err, &friendly) {
		t.Errorf("no more history: %v", err)
	}

	cost, err := a.BackfillCost(tab.Path)
	if err != nil || cost.Missing == 0 {
		t.Fatalf("cost %+v, %v; the old versions are missing", cost, err)
	}
	if err := a.Backfill(tab.Path); err != nil {
		t.Fatal(err)
	}
	if cost, err = a.BackfillCost(tab.Path); err != nil || cost.Missing != 0 {
		t.Fatalf("after backfill: %+v, %v", cost, err)
	}
	a.StopBackfillCost(tab.Path) // nothing running: does nothing

	if err := a.SetSparseFolders(tab.Path, []string{"../outside"}); !errors.As(err, &friendly) {
		t.Errorf("a folder outside: %v", err)
	}
}
