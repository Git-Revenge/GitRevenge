package git

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestRevertAllNewestFirst(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "base")
	first := fx.CommitFile("a.txt", "two\n", "two")
	second := fx.CommitFile("a.txt", "three\n", "three")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	// Newest first: each revert undoes the change the one before it left on top.
	if err := repo.RevertAll(context.Background(), second, first); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(fx.Path("a.txt")); string(got) != "one\n" {
		t.Fatalf("a.txt = %q, want it back as it was before both", got)
	}
	if got := fx.Git("log", "-2", "--format=%s"); got != "Revert \"two\"\nRevert \"three\"" {
		t.Fatalf("subjects = %q", got)
	}
	if err := repo.RevertAll(context.Background()); err == nil {
		t.Fatal("reverting nothing should fail")
	}
}

func TestFilesByCommit(t *testing.T) {
	t.Parallel()
	fx := gittest.WithMergeHistory(t)
	one := fx.CommitFile("dir/x.txt", "x\n", "add x")
	fx.WriteFile("dir/x.txt", "x2\n")
	fx.WriteFile("y.txt", "y\n")
	fx.Git("add", "-A")
	fx.Git("commit", "-q", "-m", "two files")
	two := fx.Head()
	merge := fx.Git("rev-list", "--merges", "-1", "HEAD")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	got, err := repo.FilesByCommit(context.Background(), []string{one, two, merge})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got[one], []string{"dir/x.txt"}) || !slices.Equal(got[two], []string{"dir/x.txt", "y.txt"}) {
		t.Fatalf("files = %v", got)
	}
	if _, ok := got[merge]; ok {
		t.Fatalf("a merge should have no entry: %v", got[merge])
	}
	if none, err := repo.FilesByCommit(context.Background(), nil); err != nil || len(none) != 0 {
		t.Fatalf("no commits = %v, %v", none, err)
	}
	if _, err := repo.FilesByCommit(context.Background(), []string{"--output=x"}); err == nil {
		t.Fatal("an option for a commit should be refused")
	}
}

func TestChangesBetweenTwoCommits(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	base := fx.CommitFile("a.txt", "one\ntwo\n", "base")
	fx.CommitFile("b.txt", "b\n", "add b")
	fx.Git("mv", "a.txt", "c.txt")
	fx.Git("commit", "-q", "-m", "rename a")
	tip := fx.CommitFile("c.txt", "one\ntwo\nthree\n", "grow c")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	changes, missing, err := repo.ChangesBetween(ctx, base, tip)
	if err != nil || len(missing) != 0 {
		t.Fatalf("changes = %v, missing %v, %v", changes, missing, err)
	}
	byPath := map[string]FileChange{}
	for _, c := range changes {
		byPath[c.Path] = c
	}
	if b := byPath["b.txt"]; b.Status != "A" || b.Added != 1 {
		t.Fatalf("b.txt = %+v, want added with 1 line", b)
	}
	if c := byPath["c.txt"]; c.OldPath != "a.txt" || c.Added != 1 || c.Deleted != 0 {
		t.Fatalf("c.txt = %+v, want a.txt renamed and grown by a line", c)
	}
	// Swapped, it goes the other way.
	back, _, err := repo.ChangesBetween(ctx, tip, base)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range back {
		if c.Path == "b.txt" && c.Status != "D" {
			t.Fatalf("swapped, b.txt should be deleted: %+v", c)
		}
	}
	// From nothing: the first commit's files, all added.
	first, _, err := repo.ChangesBetween(ctx, "", base)
	if err != nil || len(first) != 1 || first[0].Path != "a.txt" || first[0].Status != "A" {
		t.Fatalf("from nothing = %+v, %v", first, err)
	}
	if _, _, err := repo.ChangesBetween(ctx, "--output=x", tip); err == nil {
		t.Fatal("an option for a commit should be refused")
	}
}

func TestDiverged(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "base")
	fx.Git("checkout", "-q", "-b", "side")
	side := fx.CommitFile("s.txt", "s\n", "side")
	fx.Git("checkout", "-q", "-")
	fx.CommitFile("m1.txt", "1\n", "main one")
	main := fx.CommitFile("m2.txt", "2\n", "main two")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	onlySide, onlyMain, err := repo.Diverged(context.Background(), side, main)
	if err != nil || onlySide != 1 || onlyMain != 2 {
		t.Fatalf("diverged = %d, %d, %v; want 1, 2", onlySide, onlyMain, err)
	}
}
