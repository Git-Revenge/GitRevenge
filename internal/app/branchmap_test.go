package app

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// openForMap opens the fixture and waits for its graph, which is when the refs are read.
func openForMap(t *testing.T, a *App, fx *gittest.Repo) string {
	t.Helper()
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if page, _ := a.GraphRows(tab.Path, 0, 1); !page.Loading {
			return tab.Path
		}
	}
	t.Fatal("graph didn't finish loading")
	return ""
}

func mapNode(t *testing.T, m BranchMap, name string) MapNode {
	t.Helper()
	for _, n := range m.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no node %q in %+v", name, m.Nodes)
	return MapNode{}
}

func TestBranchMapSizesBranchesByWhatTrunkLacks(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(5)
	// topic: two commits of its own. merged: sits on an old commit of main.
	fx.Git("checkout", "-q", "-b", "topic", shas[2])
	fx.CommitFile("t1.txt", "1", "topic one")
	fx.CommitFile("t2.txt", "2", "topic two")
	fx.Git("checkout", "-q", "main")
	fx.Git("branch", "merged", shas[1])

	m, err := a.BranchMap(openForMap(t, a, fx))
	if err != nil {
		t.Fatal(err)
	}
	if m.Trunk.Name != "main" || m.Trunk.Kind != "trunk" || m.Trunk.Commits != 5 || !m.Trunk.Head {
		t.Fatalf("trunk = %+v, want main with all 5 commits, checked out", m.Trunk)
	}
	if got := mapNode(t, m, "topic"); got.Kind != "branch" || got.Commits != 2 || got.Parent != "" || got.Updated == 0 {
		t.Fatalf("topic = %+v, want a branch with 2 commits of its own", got)
	}
	if got := mapNode(t, m, "merged"); got.Commits != 0 {
		t.Fatalf("merged = %+v, want nothing of its own", got)
	}
	if m.Branches != 3 || m.Tags != 0 || m.MoreBranches != 0 {
		t.Fatalf("counts = %d branches, %d tags, %d more", m.Branches, m.Tags, m.MoreBranches)
	}
}

func TestBranchMapSizesTagsBySinceTheLastAndHangsThemOffTheirBranch(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(6)
	fx.Git("tag", "v1", shas[1])
	fx.Git("tag", "v2", shas[4])
	// A release branch with a commit main lacks, tagged there.
	fx.Git("checkout", "-q", "-b", "release/v3", shas[5])
	side := fx.CommitFile("r.txt", "r", "release only")
	fx.Git("tag", "v3", side)
	fx.Git("checkout", "-q", "main")

	m, err := a.BranchMap(openForMap(t, a, fx))
	if err != nil {
		t.Fatal(err)
	}
	if got := mapNode(t, m, "v3"); got.Kind != "tag" || got.Parent != "release/v3" || got.Commits != 2 {
		t.Fatalf("v3 = %+v, want a tag on release/v3, 2 commits after v2", got)
	}
	if got := mapNode(t, m, "v2"); got.Parent != "" || got.Commits != 3 {
		t.Fatalf("v2 = %+v, want a tag on the trunk, 3 commits after v1", got)
	}
	if got := mapNode(t, m, "v1"); got.Commits != 2 {
		t.Fatalf("v1 = %+v, want the 2 commits up to it", got)
	}
	if m.Tags != 3 {
		t.Fatalf("tags = %d, want 3", m.Tags)
	}
}

func TestBranchMapFoldsOlderTagsAndPreReleases(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(9)
	for i, sha := range shas {
		fx.Git("tag", "v"+strconv.Itoa(i), sha)
	}
	// A release candidate rides inside its release; one with no release stands for itself.
	fx.Git("tag", "v8-rc.1", shas[8])
	fx.Git("tag", "v9-rc.1", shas[8])

	m, err := a.BranchMap(openForMap(t, a, fx))
	if err != nil {
		t.Fatal(err)
	}
	var tags, folds int
	for _, n := range m.Nodes {
		switch n.Kind {
		case "tag":
			tags++
			if n.Name == "v8-rc.1" {
				t.Fatalf("v8-rc.1 has a node of its own beside v8: %+v", m.Nodes)
			}
		case "tags":
			folds++
			// Ten tags count (v0 to v8 and v9-rc.1); six show; four fold, up to v3's commit.
			if n.Tags != 4 || n.Name != "4 older tags" || n.Commits != 4 || n.Commit != shas[3] {
				t.Fatalf("fold = %+v, want 4 older tags holding the 4 commits up to v3", n)
			}
		}
	}
	if tags != 6 || folds != 1 || m.Tags != 10 {
		t.Fatalf("%d tag nodes, %d folds, %d tags counted; want 6, 1, 10", tags, folds, m.Tags)
	}
}

func TestBranchMapShowsARemoteBranchOnlyWhenNoLocalStandsForIt(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 3)
	// Someone else pushes a branch this clone never checks out.
	other := gittest.Clone(t, remote)
	other.Branch("pages")
	other.CommitFile("site.txt", "s", "publish")
	other.Git("push", "-q", "origin", "pages")
	local.Git("fetch", "-q", "origin")

	m, err := a.BranchMap(openForMap(t, a, local))
	if err != nil {
		t.Fatal(err)
	}
	if got := mapNode(t, m, "origin/pages"); got.Kind != "remote" || got.Commits != 1 {
		t.Fatalf("origin/pages = %+v, want a remote-only branch with 1 commit", got)
	}
	for _, n := range m.Nodes {
		if n.Name == "origin/"+m.Trunk.Name || n.Name == "origin/HEAD" {
			t.Fatalf("%s has a node, though the trunk stands for it", n.Name)
		}
	}
}

func TestBranchMapWithNothingToShow(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	if _, err := a.BranchMap(t.TempDir()); err == nil {
		t.Fatal("a repository that isn't open answered")
	}
	// A repository with no commits has no trunk: an empty map, and no error.
	fx := gittest.New(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.BranchMap(tab.Path)
	if err != nil || m.Trunk.Name != "" || m.Nodes == nil || len(m.Nodes) != 0 {
		t.Fatalf("empty repository: %+v, %v", m, err)
	}
}

func TestBranchMapHangsAStackedBranchOffItsParentAndSaysWhereAWorktreeIs(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(2)
	fx.Branch("base")
	fx.CommitFile("a.txt", "a", "base work")
	fx.Branch("top")
	fx.CommitFile("b.txt", "b", "top work")
	fx.Checkout("main")
	// base is checked out in a second worktree.
	fx.Git("worktree", "add", "-q", filepath.Join(t.TempDir(), "wt"), "base")

	m, err := a.BranchMap(openForMap(t, a, fx))
	if err != nil {
		t.Fatal(err)
	}
	top, base := mapNode(t, m, "top"), mapNode(t, m, "base")
	if top.Parent != "base" || !top.Stacked {
		t.Fatalf("top = %+v, want it hanging off base as a stacked branch", top)
	}
	if base.Parent != "" || base.Stacked {
		t.Fatalf("base = %+v, want it on the trunk", base)
	}
	if base.Worktree == "" || top.Worktree != "" {
		t.Fatalf("worktrees: base %q, top %q; want base's folder and nothing for top", base.Worktree, top.Worktree)
	}
}
