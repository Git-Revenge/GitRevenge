package git

import (
	"context"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestForecastPlanFollowsTheOrderAndLeavesOutDrops(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	base := fx.Head()
	one := fx.CommitFile("f.txt", "base\none\n", "One")
	two := fx.CommitFile("f.txt", "base\none\ntwo\n", "Two")
	other := fx.CommitFile("g.txt", "g\n", "Elsewhere")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	before := untouched(t, fx)
	pick := func(ids ...string) []RebaseItem {
		var items []RebaseItem
		for _, id := range ids {
			items = append(items, RebaseItem{Commit: id, Action: "pick"})
		}
		return items
	}

	// As they are, they apply.
	f, err := repo.ForecastPlan(ctx, base, pick(one, two, other))
	if err != nil || f.StopsAt != "" || f.Incomplete || len(f.Dropped) != 0 {
		t.Fatalf("in order: %+v, %v", f, err)
	}

	// Two before One: Two was written on top of One's line, so it can't go first.
	f, err = repo.ForecastPlan(ctx, base, pick(two, one, other))
	if err != nil || f.StopsAt != two || len(f.Conflicts) != 1 || f.Conflicts[0].Path != "f.txt" {
		t.Fatalf("reordered: %+v, %v", f, err)
	}

	// Dropping One leaves Two with nothing to stand on, and says what the drop throws away.
	plan := pick(one, two, other)
	plan[0].Action = "drop"
	f, err = repo.ForecastPlan(ctx, base, plan)
	if err != nil || f.StopsAt != two {
		t.Fatalf("with One dropped: %+v, %v", f, err)
	}
	if len(f.Dropped) != 1 || f.Dropped[0].Commit != one || !slices.Equal(f.Dropped[0].Files, []string{"f.txt"}) || f.Dropped[0].FileCount != 1 || f.Dropped[0].Added != 1 || f.Dropped[0].Deleted != 0 {
		t.Fatalf("dropped = %+v", f.Dropped)
	}

	// A fixup is tried like the pick it folds into; moving Elsewhere first changes nothing.
	plan = pick(other, one, two)
	plan[2].Action = "fixup"
	f, err = repo.ForecastPlan(ctx, base, plan)
	if err != nil || f.StopsAt != "" || f.Pushed || f.Upstream != "" {
		t.Fatalf("with a fixup: %+v, %v", f, err)
	}

	if after := untouched(t, fx); after != before {
		t.Fatalf("the forecast changed the repository:\n%s\n%s", before, after)
	}

	// The real rebase agrees about the reordering.
	if err := repo.InteractiveRebase(ctx, base, pick(two, one, other)); err == nil {
		t.Fatal("the real rebase was expected to stop")
	}
	if stopped := fx.Git("rev-parse", "REBASE_HEAD"); stopped != two {
		t.Fatalf("the real rebase stopped at %s, the forecast said %s", stopped, two)
	}
}

func TestForecastPlanSaysWhenTheBranchIsPushed(t *testing.T) {
	t.Parallel()
	local, _ := gittest.WithRemote(t, 1)
	base := local.Head()
	local.Branch("topic")
	a := local.CommitFile("a.txt", "a\n", "Add a")
	local.Git("push", "-q", "-u", "origin", "topic")
	b := local.CommitFile("b.txt", "b\n", "Add b")
	repo := OpenRepo(newTestRunner(t), local.Dir)

	f, err := repo.ForecastPlan(context.Background(), base, []RebaseItem{{Commit: a, Action: "pick"}, {Commit: b, Action: "pick"}})
	if err != nil || !f.Pushed || f.Upstream != "origin/topic" || f.StopsAt != "" {
		t.Fatalf("forecast = %+v, %v", f, err)
	}
}
