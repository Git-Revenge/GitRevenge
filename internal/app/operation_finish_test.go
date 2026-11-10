package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// resolve settles a conflicted file the way the conflict tool would: write it, stage it.
func resolve(t *testing.T, fx *gittest.Repo, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fx.Dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fx.Git("add", name)
}

func TestRebaseSaysWhatItIsDoingStopsAgainQuietlyAndCanBeUndone(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("topic")
	first := fx.CommitFile("f.txt", "one\n", "First change")
	second := fx.CommitFile("f.txt", "two\n", "Second change")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "ours\n", "Change on main")
	fx.Checkout("topic")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := a.Integrate(tab.Path, "rebase", []string{"main"}); err == nil {
		t.Fatal("the rebase should stop on a conflict")
	}
	st, err := a.RepoState(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Operation != "rebase" || st.Ours != "topic" || st.Theirs != "main" || st.Commit != first || st.Subject != "First change" || st.OrigHead != second {
		t.Fatalf("state = %+v", st)
	}
	if st.Step != 1 || st.Total != 2 || len(st.Written) != 0 || len(st.Conflicts) != 1 {
		t.Fatalf("state = %+v", st)
	}

	// Continuing runs into the second commit's conflict. That's the next stop, not a failure.
	resolve(t, fx, "f.txt", "ours and one\n")
	if err := a.RunOperationAction(tab.Path, "continue"); err != nil {
		t.Fatalf("stopping on the next conflict was reported as an error: %v", err)
	}
	st, _ = a.RepoState(tab.Path)
	if st.Operation != "rebase" || st.Step != 2 || st.Commit != second || len(st.Written) != 1 || len(st.Conflicts) != 1 {
		t.Fatalf("at the second stop: %+v", st)
	}
	if info, _ := a.UndoState(tab.Path); info.Undo != "" {
		t.Fatalf("nothing to undo while it's still going, got %q", info.Undo)
	}

	// Continuing with the conflict still there is a real error.
	if err := a.RunOperationAction(tab.Path, "continue"); err == nil {
		t.Fatal("continuing with unresolved conflicts should fail")
	}

	resolve(t, fx, "f.txt", "ours and two\n")
	if err := a.RunOperationAction(tab.Path, "continue"); err != nil {
		t.Fatal(err)
	}
	if st, _ := a.RepoState(tab.Path); st.Operation != "" {
		t.Fatalf("the rebase should be finished: %+v", st)
	}
	if fx.Head() == second || fx.Git("rev-parse", "--abbrev-ref", "HEAD") != "topic" {
		t.Fatal("topic should have been rewritten")
	}

	// One Undo puts the branch back where it was before the rebase began.
	if info, _ := a.UndoState(tab.Path); info.Undo != "Rebase onto main" {
		t.Fatalf("undo = %q", info.Undo)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != second {
		t.Fatalf("after Undo topic is at %s, want %s", fx.Head(), second)
	}
}

func TestMergeFinishedFromTheStripCanBeUndone(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.WithConflict(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := a.RepoState(tab.Path)
	if st.Operation != "merge" || st.Ours == "" || st.Theirs == "" || len(st.Conflicts) != 1 {
		t.Fatalf("state = %+v", st)
	}
	before := fx.Head()
	resolve(t, fx, st.Conflicts[0], "settled\n")
	if err := a.RunOperationAction(tab.Path, "continue"); err != nil {
		t.Fatal(err)
	}
	if fx.Head() == before {
		t.Fatal("the merge should have been committed")
	}
	if info, _ := a.UndoState(tab.Path); info.Undo != "Merge "+st.Theirs {
		t.Fatalf("undo = %q", info.Undo)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != before {
		t.Fatal("Undo should put the branch back where it was before the merge")
	}

	// An abort leaves nothing to undo.
	fx2 := gittest.WithConflict(t)
	tab2, _ := a.OpenRepo(fx2.Dir)
	if err := a.RunOperationAction(tab2.Path, "abort"); err != nil {
		t.Fatal(err)
	}
	if info, _ := a.UndoState(tab2.Path); info.Undo != "" {
		t.Fatalf("undo after an abort = %q", info.Undo)
	}
}
