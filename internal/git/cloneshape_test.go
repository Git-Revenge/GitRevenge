package git

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// shapeSource is a repository to clone in different shapes: five commits on main, two
// folders, and a side branch.
func shapeSource(t *testing.T) *gittest.Repo {
	t.Helper()
	src := gittest.New(t)
	src.CommitFile("README.md", "readme\n", "one")
	src.CommitFile("docs/guide.md", "guide\n", "two")
	src.CommitFile("src/main.go", "package main\n", "three")
	src.CommitFile("src/util.go", "package main\n", "four")
	src.Branch("side")
	src.CommitFile("side.txt", "side\n", "side work")
	src.Checkout("main")
	src.CommitFile("docs/guide.md", "guide, longer\n", "five")
	src.Git("config", "uploadpack.allowFilter", "true")
	return src
}

func cloneShaped(t *testing.T, src *gittest.Repo, args ...string) *Repo {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "clone")
	src.Git(append(append([]string{"clone", "-q"}, args...), "file://"+filepath.ToSlash(src.Dir), dest)...)
	return OpenRepo(newTestRunner(t), dest)
}

func TestCloneShapeOfAWholeClone(t *testing.T) {
	t.Parallel()
	s, err := cloneShaped(t, shapeSource(t)).CloneShape(context.Background())
	if err != nil || !s.Whole() {
		t.Fatalf("shape %+v, %v; want whole", s, err)
	}
}

func TestShallowCloneFetchesMoreHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := shapeSource(t)
	repo := cloneShaped(t, src, "--depth=2")
	s, err := repo.CloneShape(ctx)
	if err != nil || s.Shallow == nil || s.Shallow.Commits != 2 {
		t.Fatalf("shape %+v, %v; want shallow with 2 commits", s, err)
	}
	if want, _ := strconv.ParseInt(src.Git("log", "-1", "--format=%ct", "main~1"), 10, 64); s.Shallow.Since != want {
		t.Errorf("since %d, want %d: the edge's date", s.Shallow.Since, want)
	}
	// --depth clones one branch.
	if s.OneBranch == nil || s.OneBranch.Branch != "main" || s.OneBranch.Remote != "origin" {
		t.Errorf("one branch = %+v", s.OneBranch)
	}

	if err := repo.FetchMoreHistory(ctx, "origin", MoreHistory{Commits: 1}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s, _ = repo.CloneShape(ctx); s.Shallow == nil || s.Shallow.Commits != 3 {
		t.Fatalf("after one more: %+v", s.Shallow)
	}
	first, _ := strconv.ParseInt(src.Git("log", "-1", "--format=%ct", "main~3"), 10, 64)
	if err := repo.FetchMoreHistory(ctx, "origin", MoreHistory{Since: first}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s, _ = repo.CloneShape(ctx); s.Shallow == nil || s.Shallow.Since > first {
		t.Fatalf("after since %d: %+v", first, s.Shallow)
	}
	if err := repo.FetchMoreHistory(ctx, "origin", MoreHistory{All: true}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s, _ = repo.CloneShape(ctx); s.Shallow != nil {
		t.Fatalf("after all of it: still shallow %+v", s.Shallow)
	}
	if err := repo.FetchMoreHistory(ctx, "origin", MoreHistory{All: true}, NetOptions{}); !errors.Is(err, ErrNotShallow) {
		t.Errorf("again: %v", err)
	}
}

func TestPartialCloneBackfills(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := cloneShaped(t, shapeSource(t), "--filter=blob:none")
	s, err := repo.CloneShape(ctx)
	if err != nil || s.Partial == nil || s.Partial.Remote != "origin" || s.Partial.Filter != "blob:none" || s.Shallow != nil || s.OneBranch != nil {
		t.Fatalf("shape %+v, %v; want partial from origin, blob:none, and nothing else", s, err)
	}
	cost, err := repo.BackfillCost(ctx)
	// The first guide.md is the one version of main's history the checkout didn't write.
	if err != nil || cost.Missing != 1 || cost.Size <= 0 {
		t.Fatalf("cost %+v, %v; want one version missing", cost, err)
	}
	if err := repo.Backfill(ctx, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if cost, err = repo.BackfillCost(ctx); err != nil || cost.Missing != 0 {
		t.Fatalf("after backfill: %+v, %v", cost, err)
	}
	whole := cloneShaped(t, shapeSource(t))
	if err := whole.Backfill(ctx, NetOptions{}); !errors.Is(err, ErrNotPartial) {
		t.Errorf("a whole clone backfilled: %v", err)
	}
}

// In a sparse checkout, backfill downloads only its folders' versions, so the cost counts
// only those.
func TestBackfillCostInASparseCheckoutCountsItsFolders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := gittest.New(t)
	src.CommitFile("docs/guide.md", "guide\n", "one")
	src.CommitFile("src/main.go", "package main\n", "two")
	src.CommitFile("docs/guide.md", "guide, longer\n", "three")
	src.CommitFile("src/main.go", "package main // longer\n", "four")
	src.Git("config", "uploadpack.allowFilter", "true")
	repo := cloneShaped(t, src, "--filter=blob:none")
	if err := repo.SetSparseFolders(ctx, []string{"docs"}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	cost, err := repo.BackfillCost(ctx)
	// The first guide.md; not the first main.go, outside the folders.
	if err != nil || cost.Missing != 1 {
		t.Fatalf("cost %+v, %v; want docs' one missing version", cost, err)
	}
	if err := repo.Backfill(ctx, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if cost, err = repo.BackfillCost(ctx); err != nil || cost.Missing != 0 {
		t.Fatalf("after backfill: %+v, %v; want nothing left in the folders", cost, err)
	}
}

func TestInCone(t *testing.T) {
	t.Parallel()
	folders := []string{"arch/x86", "mm"}
	for path, want := range map[string]bool{
		"Makefile":           true,  // the top level
		"mm/slab.c":          true,  // in a folder
		"arch/x86/boot/a.S":  true,  // deep in one
		"arch/Kconfig":       true,  // directly in a folder above one
		"arch/arm/boot/b.S":  false, // beside one
		"kernel/fork.c":      false,
		"mmx/notes.txt":      false, // a name that starts the same
		"arch/x86_64/oops.c": false,
	} {
		if got := inCone(path, folders); got != want {
			t.Errorf("inCone(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSparseCloneChangesFolders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := cloneShaped(t, shapeSource(t))
	if err := repo.SetSparseFolders(ctx, []string{"docs"}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	s, err := repo.CloneShape(ctx)
	// README.md at the top, and docs/guide.md: two of four.
	if err != nil || s.Sparse == nil || !s.Sparse.Cone || !slices.Equal(s.Sparse.Folders, []string{"docs"}) || s.Sparse.Files != 2 || s.Sparse.Total != 4 {
		t.Fatalf("shape %+v (%+v), %v; want docs, 2 of 4 files", s, s.Sparse, err)
	}
	if err := repo.SetSparseFolders(ctx, []string{"docs", "src"}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s, _ = repo.CloneShape(ctx); s.Sparse == nil || s.Sparse.Files != 4 || len(s.Sparse.Folders) != 2 {
		t.Fatalf("docs and src: %+v", s.Sparse)
	}
	if err := repo.SetSparseFolders(ctx, nil, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s, _ = repo.CloneShape(ctx); s.Sparse != nil {
		t.Fatalf("every folder: still sparse %+v", s.Sparse)
	}
	if err := repo.SetSparseFolders(ctx, []string{"../out"}, NetOptions{}); err == nil {
		t.Error("a folder outside the repository was taken")
	}
}

func TestOneBranchCloneFetchesEveryBranch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := cloneShaped(t, shapeSource(t), "--single-branch")
	s, err := repo.CloneShape(ctx)
	if err != nil || s.OneBranch == nil || s.OneBranch.Branch != "main" || s.Shallow != nil {
		t.Fatalf("shape %+v, %v; want one branch, main", s, err)
	}
	if err := repo.FetchEveryBranch(ctx, "origin", NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s, _ = repo.CloneShape(ctx); s.OneBranch != nil {
		t.Fatalf("still one branch: %+v", s.OneBranch)
	}
	if _, err := repo.ReadString(ctx, "rev-parse", "--verify", "origin/side"); err != nil {
		t.Errorf("origin/side wasn't fetched: %v", err)
	}
}
