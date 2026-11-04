package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func undoLabel(t *testing.T, a *App, path string) UndoInfo {
	t.Helper()
	info, err := a.UndoState(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestUndoRedoCommit(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	base := fx.CommitFile("a.txt", "one\n", "base")
	tab, _ := a.OpenRepo(fx.Dir)

	fx.WriteFile("a.txt", "two\n")
	fx.Git("add", "a.txt")
	commit, err := a.CommitChanges(tab.Path, "Change a\n\nbody", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := undoLabel(t, a, tab.Path).Undo; got != "Commit “Change a”" {
		t.Fatalf("undo label = %q", got)
	}

	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != base {
		t.Fatal("undo should move HEAD back")
	}
	if _, err := fx.TryGit("diff", "--cached", "--quiet"); err == nil {
		t.Fatal("undoing a commit should keep its changes staged")
	}
	if got := undoLabel(t, a, tab.Path); got.Undo != "" || got.Redo != "Commit “Change a”" {
		t.Fatalf("after undo: %+v", got)
	}

	if err := a.Redo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != commit {
		t.Fatal("redo should restore the commit")
	}
	if _, err := fx.TryGit("diff", "--cached", "--quiet"); err != nil {
		t.Fatal("after redo nothing should remain staged")
	}
}

func TestUndoRefusesWhenHistoryMovedElsewhere(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "base")
	tab, _ := a.OpenRepo(fx.Dir)
	fx.WriteFile("a.txt", "two\n")
	fx.Git("add", "a.txt")
	if _, err := a.CommitChanges(tab.Path, "mine", false); err != nil {
		t.Fatal(err)
	}
	// Someone commits from a terminal.
	terminal := fx.CommitFile("b.txt", "b\n", "from the terminal")

	err := a.Undo(tab.Path)
	if err == nil || !strings.Contains(err.Error(), "can't be undone safely") {
		t.Fatalf("err = %v", err)
	}
	if fx.Head() != terminal {
		t.Fatal("a refused undo must not change anything")
	}
}

func TestUndoMergeCheckoutAndReset(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "base")
	fx.Branch("feature")
	fx.CommitFile("f.txt", "f\n", "feature")
	fx.Checkout("main")
	before := fx.CommitFile("m.txt", "m\n", "main")
	tab, _ := a.OpenRepo(fx.Dir)

	if err := a.Integrate(tab.Path, "merge", []string{"feature"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != before {
		t.Fatal("undo merge should restore main")
	}

	if err := a.BranchAction(tab.Path, "checkout", []string{"feature"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if cur, _ := fx.TryGit("symbolic-ref", "--short", "HEAD"); cur != "main" {
		t.Fatalf("undo checkout: on %q", cur)
	}

	// Mixed reset back one commit, then undo: branch returns, uncommitted state intact.
	fx.WriteFile("dirty.txt", "local\n")
	if err := a.Integrate(tab.Path, "reset", []string{"HEAD~1", "mixed"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != before {
		t.Fatal("undo reset should restore the branch")
	}
	if out := fx.Git("status", "--porcelain"); out != "?? dirty.txt" {
		t.Fatalf("status after undoing a mixed reset = %q", out)
	}
}

func TestUndoBranchCreateAndDelete(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	tab, _ := a.OpenRepo(fx.Dir)
	exists := func(b string) bool {
		_, err := fx.TryGit("rev-parse", "--verify", "-q", "refs/heads/"+b)
		return err == nil
	}

	if err := a.BranchAction(tab.Path, "create", []string{"topic"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Undo(tab.Path); err != nil || exists("topic") {
		t.Fatalf("undo create: %v (exists %v)", err, exists("topic"))
	}
	if err := a.Redo(tab.Path); err != nil || !exists("topic") {
		t.Fatalf("redo create: %v", err)
	}
	if err := a.BranchAction(tab.Path, "delete", []string{"topic"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Undo(tab.Path); err != nil || !exists("topic") {
		t.Fatalf("undo delete should restore the branch: %v", err)
	}
}

func TestNewOperationClearsRedo(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	tab, _ := a.OpenRepo(fx.Dir)
	_ = a.BranchAction(tab.Path, "create", []string{"one"})
	_ = a.Undo(tab.Path)
	_ = a.BranchAction(tab.Path, "create", []string{"two"})
	if got := undoLabel(t, a, tab.Path).Redo; got != "" {
		t.Fatalf("redo should be cleared, got %q", got)
	}
	if err := a.Redo(tab.Path); err == nil {
		t.Fatal("Redo with nothing to redo should fail")
	}
}

func TestClosingRepoDropsUndoHistory(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	tab, _ := a.OpenRepo(fx.Dir)
	_ = a.BranchAction(tab.Path, "create", []string{"one"})
	_ = a.BranchAction(tab.Path, "create", []string{"two"})
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if got := undoLabel(t, a, tab.Path); got.Undo == "" || got.Redo == "" {
		t.Fatalf("before closing: %+v", got)
	}

	if err := a.CloseRepo(tab.Path); err != nil {
		t.Fatal(err)
	}
	tab, _ = a.OpenRepo(fx.Dir)
	if got := undoLabel(t, a, tab.Path); got != (UndoInfo{}) {
		t.Fatalf("after reopening: %+v, want no undo or redo", got)
	}
	if err := a.Redo(tab.Path); err == nil {
		t.Fatal("Redo after reopening should have nothing to redo")
	}
}

func TestUndoHistoryChangesAreAnnounced(t *testing.T) {
	a := newTestApp(t)
	var mu sync.Mutex
	var got []string
	a.emitHook = func(name string, data any) {
		if c, ok := data.(UndoChange); ok && name == "undo:changed" {
			mu.Lock()
			got = append(got, c.Path)
			mu.Unlock()
		}
	}
	fx := gittest.New(t)
	fx.Linear(1)
	tab, _ := a.OpenRepo(fx.Dir)
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(got)
	}

	// The event follows the recorded entry, so a UI refreshing on it sees the new label.
	_ = a.BranchAction(tab.Path, "create", []string{"one"})
	if count() != 1 || undoLabel(t, a, tab.Path).Undo == "" {
		t.Fatalf("after an operation: %d events, %+v", count(), undoLabel(t, a, tab.Path))
	}
	fx.WriteFile("b.txt", "b")
	fx.Git("add", "b.txt")
	if _, err := a.CommitChanges(tab.Path, "Add b", false); err != nil {
		t.Fatal(err)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if err := a.Redo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if count() != 4 {
		t.Fatalf("events = %v, want one per change", got)
	}
	for _, p := range got {
		if p != tab.Path {
			t.Fatalf("event for %q, want %q", p, tab.Path)
		}
	}
}
