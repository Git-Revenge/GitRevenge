package git

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestOperationNamesBothSidesOfAMerge(t *testing.T) {
	t.Parallel()
	fx := conflictingBranches(t, 1)
	head := fx.Head()
	theirs := fx.Git("rev-parse", "other")
	if _, err := fx.TryGit("merge", "other"); err == nil {
		t.Fatal("expected a merge conflict")
	}

	st := stateOf(t, fx)
	if st.Ours != "main" || st.Theirs != "other" {
		t.Fatalf("sides = %q and %q, want main and other", st.Ours, st.Theirs)
	}
	if st.Commit != theirs || st.Subject != "theirs" || st.OrigHead != head || len(st.Written) != 0 {
		t.Fatalf("state = %+v", st)
	}
}

func TestOperationNamesAMergedCommitByWhatPointsAtIt(t *testing.T) {
	t.Parallel()
	fx := conflictingBranches(t, 1)
	// Merging by id: git's message names the id, and the branch that points there reads better.
	theirs := fx.Git("rev-parse", "other")
	if _, err := fx.TryGit("merge", theirs); err == nil {
		t.Fatal("expected a merge conflict")
	}
	if st := stateOf(t, fx); st.Theirs != "other" || st.Ours != "main" {
		t.Fatalf("sides = %q and %q", st.Ours, st.Theirs)
	}
}

func TestOperationDescribesARebaseInProgress(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("topic")
	fx.CommitFile("a.txt", "a\n", "Add a")
	stops := fx.CommitFile("f.txt", "theirs\n", "Change f")
	tip := fx.CommitFile("b.txt", "b\n", "Add b")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "ours\n", "Change f on main")
	fx.Checkout("topic")
	if _, err := fx.TryGit("rebase", "main"); err == nil {
		t.Fatal("expected a rebase conflict")
	}

	st := stateOf(t, fx)
	if st.Op != Rebase || st.Step != 2 || st.Total != 3 {
		t.Fatalf("state = %+v", st)
	}
	if st.Ours != "topic" || st.Theirs != "main" {
		t.Fatalf("sides = %q onto %q, want topic onto main", st.Ours, st.Theirs)
	}
	if st.Commit != stops || st.Subject != "Change f" || st.OrigHead != tip {
		t.Fatalf("in hand %s %q, was at %s; want %s, Change f, %s", st.Commit, st.Subject, st.OrigHead, stops, tip)
	}
	// One commit has been written again so far: "Add a", now on top of main.
	if len(st.Written) != 1 || fx.Git("log", "-1", "--format=%s", st.Written[0]) != "Add a" {
		t.Fatalf("written = %v", st.Written)
	}
}

func TestOperationDescribesACherryPickAndARevert(t *testing.T) {
	t.Parallel()
	fx := conflictingBranches(t, 1)
	head := fx.Head()
	pick := fx.Git("rev-parse", "other")
	if _, err := fx.TryGit("cherry-pick", pick); err == nil {
		t.Fatal("expected a cherry-pick conflict")
	}
	st := stateOf(t, fx)
	if st.Op != CherryPick || st.Ours != "main" || st.Theirs != "" || st.Commit != pick || st.Subject != "theirs" || st.OrigHead != head {
		t.Fatalf("state = %+v", st)
	}
	fx.Git("cherry-pick", "--abort")

	// Reverting the base under a later change to the same file conflicts.
	base := fx.Git("rev-parse", "HEAD~1")
	if _, err := fx.TryGit("revert", "--no-edit", base); err == nil {
		t.Fatal("expected a revert conflict")
	}
	st = stateOf(t, fx)
	if st.Op != Revert || st.Commit != base || st.Subject != "base" || st.Ours != "main" {
		t.Fatalf("state = %+v", st)
	}
}

func TestIdleDescribesNothing(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	if st := stateOf(t, fx); st.Ours != "" || st.Commit != "" || st.OrigHead != "" {
		t.Fatalf("state = %+v", st)
	}
}
