package git

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func stateOf(t *testing.T, fx *gittest.Repo) OperationState {
	t.Helper()
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	st, err := repo.OperationState(context.Background(), fx.Path(".git"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// conflictingBranches makes main and "other" both change f.txt, leaving main checked out.
func conflictingBranches(t *testing.T, commitsOnOther int) *gittest.Repo {
	t.Helper()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("other")
	for i := range commitsOnOther {
		fx.CommitFile("f.txt", "theirs "+string(rune('a'+i))+"\n", "theirs")
	}
	fx.Checkout("main")
	fx.CommitFile("f.txt", "ours\n", "ours")
	return fx
}

func TestIdle(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	if st := stateOf(t, fx); st.Op != None || len(st.Conflicts) != 0 {
		t.Fatalf("state = %+v", st)
	}
}

func TestMergeConflictThenAbort(t *testing.T) {
	fx := gittest.WithConflict(t)
	st := stateOf(t, fx)
	if st.Op != Merge || !slices.Equal(st.Conflicts, []string{"conflict.txt"}) {
		t.Fatalf("state = %+v", st)
	}

	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if err := repo.RunAction(context.Background(), fx.Path(".git"), Abort); err != nil {
		t.Fatal(err)
	}
	if st := stateOf(t, fx); st.Op != None {
		t.Fatalf("after abort: %+v", st)
	}
}

func TestMergeContinueAfterResolving(t *testing.T) {
	fx := gittest.WithConflict(t)
	fx.WriteFile("conflict.txt", "resolved\n")
	fx.Git("add", "conflict.txt")

	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if err := repo.RunAction(context.Background(), fx.Path(".git"), Continue); err != nil {
		t.Fatal(err)
	}
	if st := stateOf(t, fx); st.Op != None {
		t.Fatalf("after continue: %+v", st)
	}
	if parents := fx.Git("rev-list", "--parents", "-n1", "HEAD"); len(parents) < 3*40 {
		t.Fatalf("HEAD should be a merge commit: %s", parents)
	}
}

func TestRebaseProgressAndSkip(t *testing.T) {
	fx := conflictingBranches(t, 1)
	fx.Checkout("other")
	fx.CommitFile("g.txt", "more\n", "second")
	if _, err := fx.TryGit("rebase", "main"); err == nil {
		t.Fatal("expected rebase conflict")
	}

	st := stateOf(t, fx)
	if st.Op != Rebase || st.Step != 1 || st.Total != 2 || len(st.Conflicts) != 1 {
		t.Fatalf("state = %+v", st)
	}
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if err := repo.RunAction(context.Background(), fx.Path(".git"), Skip); err != nil {
		t.Fatal(err)
	}
	if st := stateOf(t, fx); st.Op != None {
		t.Fatalf("second commit applies cleanly, so the rebase should finish: %+v", st)
	}
}

func TestCherryPickAndRevertDetected(t *testing.T) {
	fx := conflictingBranches(t, 1)
	if _, err := fx.TryGit("cherry-pick", "other"); err == nil {
		t.Fatal("expected cherry-pick conflict")
	}
	if st := stateOf(t, fx); st.Op != CherryPick {
		t.Fatalf("state = %+v", st)
	}
	fx.Git("cherry-pick", "--abort")

	fx.CommitFile("f.txt", "ours again\n", "change again")
	if _, err := fx.TryGit("revert", "--no-edit", "HEAD~1"); err == nil {
		t.Fatal("expected revert conflict")
	}
	if st := stateOf(t, fx); st.Op != Revert {
		t.Fatalf("state = %+v", st)
	}
}

func TestBisectDetectedAndReset(t *testing.T) {
	fx := gittest.New(t)
	shas := fx.Linear(4)
	fx.Git("bisect", "start", shas[3], shas[0])
	if st := stateOf(t, fx); st.Op != Bisect {
		t.Fatalf("state = %+v", st)
	}
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if err := repo.RunAction(context.Background(), fx.Path(".git"), Continue); !errors.As(err, new(*ErrActionNotAllowed)) {
		t.Fatalf("continue on bisect: err = %v", err)
	}
	if err := repo.RunAction(context.Background(), fx.Path(".git"), Abort); err != nil {
		t.Fatal(err)
	}
	if st := stateOf(t, fx); st.Op != None {
		t.Fatalf("after reset: %+v", st)
	}
}

func TestActionWithNothingInProgress(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	var notAllowed *ErrActionNotAllowed
	if err := repo.RunAction(context.Background(), fx.Path(".git"), Abort); !errors.As(err, &notAllowed) || notAllowed.Op != None {
		t.Fatalf("err = %v", err)
	}
}
