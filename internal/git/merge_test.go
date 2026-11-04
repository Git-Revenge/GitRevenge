package git

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestMergeModes(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a", "base")
	fx.Branch("feature")
	fx.CommitFile("f.txt", "f", "feature")
	fx.Checkout("main")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if err := repo.Merge(ctx, "feature", MergeNoFF); err != nil {
		t.Fatal(err)
	}
	if parents := strings.Fields(fx.Git("rev-list", "--parents", "-n1", "HEAD")); len(parents) != 3 {
		t.Fatalf("no-ff should create a merge commit: %v", parents)
	}

	fx.Checkout("feature")
	fx.CommitFile("g.txt", "g", "more feature")
	fx.Checkout("main")
	fx.CommitFile("m.txt", "m", "main moved")
	if err := repo.Merge(ctx, "feature", MergeFFOnly); err == nil || errors.Is(err, ErrConflicts) {
		t.Fatalf("ff-only on diverged branches should fail plainly: %v", err)
	}
}

func TestMergeFastForward(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a", "base")
	fx.Branch("feature")
	tip := fx.CommitFile("f.txt", "f", "feature")
	fx.Checkout("main")
	if err := OpenRepo(newTestRunner(t), fx.Dir).Merge(context.Background(), "feature", MergeFFOnly); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != tip {
		t.Fatal("fast-forward should move main to feature")
	}
}

func TestMergeConflictLeavesOperation(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("c.txt", "base\n", "base")
	fx.Branch("theirs")
	fx.CommitFile("c.txt", "theirs\n", "theirs")
	fx.Checkout("main")
	fx.CommitFile("c.txt", "ours\n", "ours")

	err := OpenRepo(newTestRunner(t), fx.Dir).Merge(context.Background(), "theirs", MergeDefault)
	if !errors.Is(err, ErrConflicts) {
		t.Fatalf("err = %v", err)
	}
	if op, _, _ := DetectOperation(fx.Path(".git")); op != Merge {
		t.Fatalf("operation = %q, want merge in progress", op)
	}
}

func TestCherryPickAndRebase(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a", "base")
	fx.Branch("feature")
	one := fx.CommitFile("one.txt", "1", "one")
	two := fx.CommitFile("two.txt", "2", "two")
	fx.Checkout("main")
	fx.CommitFile("m.txt", "m", "main work")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if err := repo.CherryPick(ctx, one, two); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("log", "-2", "--format=%s"); got != "two\none" {
		t.Fatalf("cherry-picked = %q", got)
	}

	fx.Checkout("feature")
	if err := repo.Rebase(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	// main already has one and two (cherry-picked), so the rebase drops them as already
	// applied; what matters is that feature now builds on main.
	if _, err := fx.TryGit("merge-base", "--is-ancestor", "main", "feature"); err != nil {
		t.Fatal("feature should now sit on top of main")
	}
}

func TestCherryPickConflict(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("c.txt", "base\n", "base")
	fx.Branch("other")
	pick := fx.CommitFile("c.txt", "other\n", "other")
	fx.Checkout("main")
	fx.CommitFile("c.txt", "main\n", "main")

	err := OpenRepo(newTestRunner(t), fx.Dir).CherryPick(context.Background(), pick)
	if !errors.Is(err, ErrConflicts) {
		t.Fatalf("err = %v", err)
	}
	if op, _, _ := DetectOperation(fx.Path(".git")); op != CherryPick {
		t.Fatalf("operation = %q", op)
	}
}
