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
