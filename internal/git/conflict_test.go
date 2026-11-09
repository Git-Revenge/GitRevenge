package git

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestConflictStages(t *testing.T) {
	t.Parallel()
	fx := gittest.WithConflict(t) // conflict.txt: base / ours / theirs
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	cf, err := repo.Conflict(context.Background(), "conflict.txt")
	if err != nil {
		t.Fatal(err)
	}
	if cf.Base.Text != "base\n" || cf.Ours.Text != "ours\n" || cf.Theirs.Text != "theirs\n" || cf.Binary {
		t.Fatalf("stages = %q / %q / %q", cf.Base.Text, cf.Ours.Text, cf.Theirs.Text)
	}
	if !strings.Contains(cf.Merged.Text, "<<<<<<<") || !strings.Contains(cf.Merged.Text, ">>>>>>>") {
		t.Fatalf("work tree should hold conflict markers: %q", cf.Merged.Text)
	}
}

func TestResolveWithStagesResult(t *testing.T) {
	t.Parallel()
	fx := gittest.WithConflict(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if err := repo.ResolveWith(ctx, "conflict.txt", "ours\ntheirs\n"); err != nil {
		t.Fatal(err)
	}
	st := status(t, repo)["conflict.txt"]
	if st.Conflicted || !st.Staged() {
		t.Fatalf("after resolve: %+v", st)
	}
	if got := fx.Git("show", ":conflict.txt"); got != "ours\ntheirs" {
		t.Fatalf("staged = %q", got)
	}
}

func TestTakeSide(t *testing.T) {
	t.Parallel()
	for _, side := range []ConflictSide{Ours, Theirs} {
		t.Run(string(side), func(t *testing.T) {
			fx := gittest.WithConflict(t)
			repo := OpenRepo(newTestRunner(t), fx.Dir)
			if err := repo.TakeSide(context.Background(), "conflict.txt", side); err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(fx.Path("conflict.txt"))
			if string(b) != string(side)+"\n" {
				t.Fatalf("content = %q", b)
			}
			if status(t, repo)["conflict.txt"].Conflicted {
				t.Fatal("still conflicted")
			}
		})
	}
}

func TestTakeSideWhereThatSideDeleted(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("d.txt", "base\n", "base")
	fx.Branch("theirs")
	fx.Git("rm", "-q", "d.txt")
	fx.Commit("delete it")
	fx.Checkout("main")
	fx.CommitFile("d.txt", "edited\n", "edit it")
	if err := fx.Merge("theirs"); err == nil {
		t.Fatal("expected a modify/delete conflict")
	}
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	cf, err := repo.Conflict(context.Background(), "d.txt")
	if err != nil || cf.Theirs.Exists || !cf.Ours.Exists {
		t.Fatalf("stages: %+v, %v", cf, err)
	}
	if err := repo.TakeSide(context.Background(), "d.txt", Theirs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fx.Path("d.txt")); !os.IsNotExist(err) {
		t.Fatal("taking the deleting side should delete the file")
	}
	if _, ok := status(t, repo)["d.txt"]; ok && status(t, repo)["d.txt"].Conflicted {
		t.Fatal("still conflicted")
	}
}

func TestTakeSideRequiresConflict(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a", "base")
	err := OpenRepo(newTestRunner(t), fx.Dir).TakeSide(context.Background(), "a.txt", Ours)
	if !errors.Is(err, ErrNotConflicted) {
		t.Fatalf("err = %v", err)
	}
}
