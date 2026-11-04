package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestStashBindings(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "base\n", "base")
	tab, _ := a.OpenRepo(fx.Dir)

	if err := a.StashSave(tab.Path, "x", false); err == nil || !strings.Contains(err.Error(), "no changes") {
		t.Fatalf("err = %v", err)
	}
	fx.WriteFile("a.txt", "wip\n")
	if err := a.StashSave(tab.Path, "my work", false); err != nil {
		t.Fatal(err)
	}
	id := fx.Git("rev-parse", "refs/stash")
	if err := a.StashAction(tab.Path, id, "apply"); err != nil {
		t.Fatal(err)
	}
	// Applying again over the same local change is refused by git; its message comes through.
	if err := a.StashAction(tab.Path, id, "apply"); err == nil {
		t.Fatal("expected an error when local changes would be overwritten")
	}
	if err := a.StashAction(tab.Path, id, "drop"); err != nil {
		t.Fatal(err)
	}
	if err := a.StashAction(tab.Path, id, "drop"); err == nil || err.Error() != "That stash no longer exists." {
		t.Fatalf("err = %v", err)
	}
}
