package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestInteractiveRebaseAndUndo(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	base := fx.CommitFile("base.txt", "b\n", "base")
	fx.CommitFile("a.txt", "a\n", "add a")
	fx.CommitFile("c.txt", "c\n", "add c")
	before := fx.Head()
	tab, _ := a.OpenRepo(fx.Dir)

	plan, err := a.RebasePlan(tab.Path, base)
	if err != nil || len(plan) != 2 {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	plan[1].Action = "squash"
	plan[1].Message = "add a and c"
	if err := a.InteractiveRebase(tab.Path, base, plan); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("log", "-1", "--format=%s"); got != "add a and c" {
		t.Fatalf("subject = %q", got)
	}
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	if fx.Head() != before {
		t.Fatal("undo should restore the original commits")
	}

	plan[0].Action = "fixup"
	if err := a.InteractiveRebase(tab.Path, base, plan); err == nil || !strings.Contains(err.Error(), "first commit") {
		t.Fatalf("err = %v", err)
	}
}
