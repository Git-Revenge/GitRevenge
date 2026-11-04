package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestIntegrate(t *testing.T) {
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
