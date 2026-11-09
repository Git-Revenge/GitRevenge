package git

import (
	"context"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestCountLines(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fx := gittest.New(t)
	fx.WriteFile("a.txt", "one\ntwo\n")
	fx.WriteFile("b.txt", "one\n")
	fx.Commit("start")
	fx.WriteFile("a.txt", "one\nTWO\nthree\n")
	fx.Git("add", "a.txt")
	fx.WriteFile("a.txt", "one\nTWO\nthree\nfour\n")
	fx.WriteFile("b.txt", "one\nmore\n")
	fx.WriteFile("new.txt", "untracked\n")

	repo := OpenRepo(newTestRunner(t), fx.Dir)
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repo.CountLines(ctx, &st)
	got := map[string]FileStatus{}
	for _, f := range st.Files {
		got[f.Path] = f
	}
	// Staged and unstaged together, against HEAD.
	if f := got["a.txt"]; f.Added != 3 || f.Deleted != 1 {
		t.Errorf("a.txt: +%d -%d", f.Added, f.Deleted)
	}
	if f := got["b.txt"]; f.Added != 1 || f.Deleted != 0 {
		t.Errorf("b.txt: +%d -%d", f.Added, f.Deleted)
	}
	if f := got["new.txt"]; f.Added != 0 || f.Deleted != 0 {
		t.Errorf("new.txt: +%d -%d", f.Added, f.Deleted)
	}
}

func TestCountLinesWithoutCommitsLeavesZeros(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fx := gittest.New(t)
	fx.WriteFile("a.txt", "one\n")
	fx.Git("add", "a.txt")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repo.CountLines(ctx, &st)
	if f := st.Files[0]; f.Added != 0 || f.Deleted != 0 {
		t.Errorf("got +%d -%d", f.Added, f.Deleted)
	}
}
