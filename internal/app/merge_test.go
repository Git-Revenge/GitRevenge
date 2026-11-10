package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestIntegrate(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("c.txt", "base\n", "base")
	fx.Branch("feature")
	pick := fx.CommitFile("f.txt", "f\n", "feature file")
	fx.Checkout("main")
	tab, _ := a.OpenRepo(fx.Dir)

	if err := a.Integrate(tab.Path, "cherry-pick", []string{pick}); err != nil {
		t.Fatal(err)
	}
	if err := a.Integrate(tab.Path, "merge", []string{"feature", "no-ff"}); err != nil {
		t.Fatal(err)
	}

	fx.Branch("conflict")
	fx.CommitFile("c.txt", "theirs\n", "theirs")
	fx.Checkout("main")
	fx.CommitFile("c.txt", "ours\n", "ours")
	err := a.Integrate(tab.Path, "merge", []string{"conflict"})
	if err == nil || !strings.Contains(err.Error(), "conflicts to resolve") {
		t.Fatalf("err = %v", err)
	}
	if st, _ := a.RepoState(tab.Path); st.Operation != "merge" {
		t.Fatalf("banner state = %+v", st)
	}
}

func TestResetAndRevertActions(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	base := fx.CommitFile("a.txt", "one\n", "base")
	change := fx.CommitFile("b.txt", "b\n", "add b")
	tab, _ := a.OpenRepo(fx.Dir)

	if err := a.Integrate(tab.Path, "revert", []string{change, ""}); err != nil {
		t.Fatal(err)
	}
	if err := a.Integrate(tab.Path, "reset", []string{base, "hard"}); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != base {
		t.Fatal("hard reset should move HEAD to base")
	}
	if err := a.Integrate(tab.Path, "reset", []string{base, "sideways"}); err == nil {
		t.Fatal("bad mode accepted")
	}
}

func TestIntegrateSquashAndAutosquash(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("c.txt", "base\n", "base")
	fx.Branch("feature")
	fx.CommitFile("f.txt", "f\n", "Add f")
	fx.CommitFile("g.txt", "g\n", "Add g")
	fx.CommitFile("f.txt", "f2\n", "fixup! Add f")
	fx.Checkout("main")
	head := fx.CommitFile("m.txt", "m\n", "On main")
	tab, _ := a.OpenRepo(fx.Dir)

	// A squash stages the branch's changes and commits nothing.
	if err := a.Integrate(tab.Path, "merge", []string{"feature", "squash"}); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != head {
		t.Fatal("a squash merge moved the branch")
	}
	if got := fx.Git("diff", "--cached", "--name-only"); got != "f.txt\ng.txt" {
		t.Fatalf("staged = %q", got)
	}
	fx.Git("reset", "--hard", "-q")

	// A rebase that folds fixups leaves two commits where there were three.
	fx.Checkout("feature")
	if err := a.Integrate(tab.Path, "rebase", []string{"main", "autosquash"}); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("log", "--format=%s", "main..HEAD"); got != "Add g\nAdd f" {
		t.Fatalf("after the rebase: %q", got)
	}
	if got := fx.Git("show", "HEAD~1:f.txt"); got != "f2" {
		t.Fatalf("the fixup wasn't folded in: f.txt = %q", got)
	}
	if info, _ := a.UndoState(tab.Path); info.Undo == "" {
		t.Fatal("the rebase should be undoable")
	}
}
