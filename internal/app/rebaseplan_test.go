package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestPreviewRebasePlanForecastsTheEditorsOrder(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	base := fx.Head()
	one := fx.CommitFile("f.txt", "base\none\n", "One")
	two := fx.CommitFile("f.txt", "base\none\ntwo\n", "Two")
	debug := fx.CommitFile("debug.log", "x\ny\n", "debugging")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := a.RebasePlan(tab.Path, base)
	if err != nil || len(plan) != 3 || plan[0].Commit != one {
		t.Fatalf("plan = %+v, %v", plan, err)
	}

	// Drop the debugging commit: nothing conflicts, and the preview says what goes with it.
	plan[2].Action = "drop"
	p, err := a.PreviewRebasePlan(tab.Path, base, plan)
	if err != nil {
		t.Fatal(err)
	}
	if p.StopsAt != "" || p.Incomplete || p.Pushed || len(p.Conflicts) != 0 {
		t.Fatalf("preview = %+v", p)
	}
	if len(p.Dropped) != 1 || p.Dropped[0].Commit != debug || p.Dropped[0].FileCount != 1 || p.Dropped[0].Files[0] != "debug.log" || p.Dropped[0].Added != 2 {
		t.Fatalf("dropped = %+v", p.Dropped)
	}

	// Swap the first two: Two can't go before the line it was written under.
	swapped := []git.RebaseItem{plan[1], plan[0], plan[2]}
	p, err = a.PreviewRebasePlan(tab.Path, base, swapped)
	if err != nil || p.StopsAt != two || len(p.Conflicts) != 1 || p.Conflicts[0].Path != "f.txt" {
		t.Fatalf("swapped: %+v, %v", p, err)
	}
	if fx.Head() != debug || fx.Git("status", "--porcelain") != "" {
		t.Fatal("previewing a plan changed the repository")
	}
}
