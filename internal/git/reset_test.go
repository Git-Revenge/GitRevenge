package git

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestResetModes(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		mode             ResetMode
		staged, worktree bool // is the undone change still staged / in the work tree?
	}{
		{ResetSoft, true, true},
		{ResetMixed, false, true},
		{ResetHard, false, false},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			fx := gittest.New(t)
			base := fx.CommitFile("a.txt", "one\n", "base")
			fx.CommitFile("a.txt", "two\n", "change")
			repo := OpenRepo(newTestRunner(t), fx.Dir)

			if err := repo.Reset(ctx, base, tc.mode); err != nil {
				t.Fatal(err)
			}
			if fx.Head() != base {
				t.Fatal("HEAD should move to the target")
			}
			_, stagedErr := fx.TryGit("diff", "--cached", "--quiet")
			if staged := stagedErr != nil; staged != tc.staged {
				t.Fatalf("staged = %v, want %v", staged, tc.staged)
			}
			b, _ := os.ReadFile(fx.Path("a.txt"))
			if inWorktree := string(b) == "two\n"; inWorktree != tc.worktree {
				t.Fatalf("work tree has change = %v, want %v", inWorktree, tc.worktree)
			}
		})
	}
	if err := OpenRepo(newTestRunner(t), gittest.New(t).Dir).Reset(ctx, "HEAD", "bogus"); err == nil {
		t.Fatal("unknown mode accepted")
	}
}

func TestRevert(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "base")
	bad := fx.CommitFile("b.txt", "oops\n", "add b")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if err := repo.Revert(ctx, bad, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fx.Path("b.txt")); !os.IsNotExist(err) {
		t.Fatal("revert should remove the added file")
	}
	if got := fx.Git("log", "-1", "--format=%s"); got != `Revert "add b"` {
		t.Fatalf("subject = %q", got)
	}
}

func TestRevertMergeNeedsMainline(t *testing.T) {
	fx := gittest.WithMergeHistory(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	merge := fx.Head()

	var gerr *Error
	if err := repo.Revert(ctx, merge, 0); !errors.As(err, &gerr) {
		t.Fatalf("reverting a merge without a parent should fail: %v", err)
	}
	if err := repo.Revert(ctx, merge, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fx.Path("feature.txt")); !os.IsNotExist(err) {
		t.Fatal("reverting the merge against parent 1 should drop the feature's changes")
	}
}

func TestRevertConflict(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("c.txt", "one\n", "base")
	target := fx.CommitFile("c.txt", "two\n", "change")
	fx.CommitFile("c.txt", "three\n", "change again")

	err := OpenRepo(newTestRunner(t), fx.Dir).Revert(context.Background(), target, 0)
	if !errors.Is(err, ErrConflicts) {
		t.Fatalf("err = %v", err)
	}
	if op, _, _ := DetectOperation(fx.Path(".git")); op != Revert {
		t.Fatalf("operation = %q", op)
	}
}
