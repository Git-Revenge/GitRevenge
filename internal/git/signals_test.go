package git

import (
	"context"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestRenamedTo(t *testing.T) {
	for in, want := range map[string]string{
		"a.txt":                 "a.txt",
		"old.txt => new.txt":    "new.txt",
		"src/{a => b}/file.go":  "src/b/file.go",
		"src/{ => sub}/file.go": "src/sub/file.go",
		"{old => new}.txt":      "new.txt",
		"dir/{x.js => y.js}":    "dir/y.js",
	} {
		if got := renamedTo(in); got != want {
			t.Errorf("renamedTo(%q) = %q, want %q", in, got, want)
		}
	}
}

// branchesFixture makes origin with main and three branches, clones it, and adds two local
// branches: one tracking a remote, one with no upstream.
func branchesFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	origin := gittest.New(t)
	origin.CommitFile("a.txt", "base\n", "Start")
	origin.Branch("conflicting")
	origin.Branch("clean")
	origin.Branch("merged")
	origin.Checkout("conflicting")
	origin.CommitFile("a.txt", "theirs\n", "Change a on a branch")
	origin.Checkout("clean")
	origin.CommitFile("b.txt", "one\ntwo\n", "Add b")
	origin.Checkout("merged")
	origin.CommitFile("c.txt", "c\n", "Add c")
	origin.Checkout("main")
	origin.Git("merge", "--no-ff", "-m", "Merge merged", "merged")
	origin.CommitFile("a.txt", "ours\n", "Change a on main")

	fx := gittest.Clone(t, origin)
	fx.Git("branch", "--track", "tracking", "origin/clean")
	// The same name and commit as a remote branch, but no upstream: the same work.
	fx.Git("branch", "--no-track", "clean", "origin/clean")
	fx.Git("branch", "local-only", "origin/main~1")
	fx.Checkout("local-only")
	fx.CommitFile("d.txt", "d\n", "Local work")
	fx.Checkout("main")
	return fx
}

func TestOpenBranches(t *testing.T) {
	fx := branchesFixture(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	bs, err := repo.OpenBranches(ctx, "origin/main", MaxSignalBranches, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, b := range bs {
		names = append(names, b.Name)
	}
	// Merged branches, the target, origin/HEAD and a tracking local branch are left out.
	slices.Sort(names)
	if want := []string{"local-only", "origin/clean", "origin/conflicting"}; !slices.Equal(names, want) {
		t.Fatalf("branches = %v, want %v", names, want)
	}
	by := func(n string) OpenBranch {
		return bs[slices.IndexFunc(bs, func(b OpenBranch) bool { return b.Name == n })]
	}

	clean := by("origin/clean")
	if clean.Local || clean.Author != "Test User" || clean.Added != 2 || clean.Removed != 0 || !slices.Equal(clean.Paths, []string{"b.txt"}) {
		t.Errorf("clean = %+v", clean)
	}
	// main gained a merge, the merged branch's commit, and "Change a on main" since.
	if clean.Behind != 3 {
		t.Errorf("clean behind = %d, want 3", clean.Behind)
	}
	if clean.FirstCommit == 0 || clean.FirstCommit > clean.LastCommit {
		t.Errorf("clean dates = %d..%d", clean.FirstCommit, clean.LastCommit)
	}
	if lo := by("local-only"); !lo.Local || lo.Behind != 1 || !slices.Equal(lo.Paths, []string{"d.txt"}) {
		t.Errorf("local-only = %+v", lo)
	}

	// The most recent come first, ties by name, and the cap keeps the first.
	for i := 1; i < len(bs); i++ {
		if bs[i-1].LastCommit < bs[i].LastCommit || (bs[i-1].LastCommit == bs[i].LastCommit && bs[i-1].Name > bs[i].Name) {
			t.Errorf("order: %s before %s", bs[i-1].Name, bs[i].Name)
		}
	}
	two, err := repo.OpenBranches(ctx, "origin/main", 2, nil)
	if err != nil || len(two) != 2 || two[0].Name != bs[0].Name || two[1].Name != bs[1].Name {
		t.Errorf("capped = %+v, %v", two, err)
	}
}

func TestWouldConflict(t *testing.T) {
	fx := branchesFixture(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	head := fx.Head()

	c, err := repo.WouldConflict(ctx, "origin/main", "origin/conflicting")
	if err != nil || !c.Conflicts || !slices.Equal(c.Files, []string{"a.txt"}) {
		t.Errorf("conflicting = %+v, %v", c, err)
	}
	c, err = repo.WouldConflict(ctx, "origin/main", "origin/clean")
	if err != nil || c.Conflicts || len(c.Files) != 0 {
		t.Errorf("clean = %+v, %v", c, err)
	}
	// A dry run leaves the work tree and HEAD alone.
	if fx.Head() != head || fx.Git("status", "--porcelain") != "" {
		t.Error("merge-tree touched the repository")
	}
}

func TestHotFiles(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "Start")
	since := fx.Commit("Mark")
	fx.CommitFile("a.txt", "1\n2\n3\n", "Grow a")
	fx.CommitFile("b.txt", "b\n", "Add b")
	fx.Git("mv", "b.txt", "c.txt")
	fx.Commit("Rename b")
	fx.CommitFile("a.txt", "1\n", "Shrink a")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	mark, err := repo.Tip(ctx, since)
	if err != nil {
		t.Fatal(err)
	}
	tip, _ := repo.Tip(ctx, "main")

	files, err := repo.HotFiles(ctx, ShippedQuery{Target: "main", Since: mark.Time + 1, Until: tip.Time})
	if err != nil {
		t.Fatal(err)
	}
	// A rename counts against the new path in its own commit; history before it keeps the
	// old one.
	want := []HotFile{{Path: "a.txt", Lines: 4, Items: 2}, {Path: "b.txt", Lines: 1, Items: 1}, {Path: "c.txt", Lines: 0, Items: 1}}
	if !slices.Equal(files, want) {
		t.Errorf("hot files = %+v, want %+v", files, want)
	}
}
