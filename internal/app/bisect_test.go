package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func openForBisect(t *testing.T, commits int) (*App, *gittest.Repo, string, []string) {
	t.Helper()
	a := newTestApp(t)
	fx := gittest.New(t)
	ids := fx.Linear(commits)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return a, fx, tab.Path, ids
}

func TestPlanBisectSuggestsTheLastTagAndCountsTheRange(t *testing.T) {
	t.Parallel()
	a, fx, path, ids := openForBisect(t, 9)
	fx.Git("tag", "v1", ids[0])
	plan, err := a.PlanBisect(path, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Bad != ids[8] || plan.Good != ids[0] || plan.Suggested != "v1" || plan.Problem != "" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Commits != 8 || plan.Steps != 3 || !slices.Contains(ids[1:8], plan.First) || plan.FirstSubject == "" {
		t.Fatalf("commits %d, steps %d, first %s %q", plan.Commits, plan.Steps, plan.First, plan.FirstSubject)
	}
	if plan.BadSubject == "" || plan.GoodSubject == "" {
		t.Fatalf("subjects = %q, %q", plan.BadSubject, plan.GoodSubject)
	}
}

func TestPlanBisectSaysWhatIsWrong(t *testing.T) {
	t.Parallel()
	a, _, path, ids := openForBisect(t, 4)
	// No tag to suggest: the plan has a bad commit and waits for a good one.
	plan, err := a.PlanBisect(path, "", "")
	if err != nil || plan.Good != "" || plan.Problem != "" || plan.Commits != 0 {
		t.Fatalf("with nothing to suggest: %+v, %v", plan, err)
	}
	for _, c := range []struct{ bad, good, want string }{
		{ids[0], ids[3], "isn't in the bad commit's history"},
		{ids[2], ids[2], "same commit"},
		{"nope", ids[0], "can't find"},
		{ids[3], "nope", "can't find"},
	} {
		plan, err := a.PlanBisect(path, c.bad, c.good)
		if err != nil || !strings.Contains(plan.Problem, c.want) {
			t.Errorf("PlanBisect(%s, %s): problem %q, err %v; want it to mention %q", c.bad, c.good, plan.Problem, err, c.want)
		}
	}
}

func TestBisectFromStartToAnswer(t *testing.T) {
	t.Parallel()
	a, fx, path, ids := openForBisect(t, 8)
	const broke = 5
	if info, err := a.Bisect(path); err != nil || info.Running {
		t.Fatalf("before starting: %+v, %v", info, err)
	}
	if err := a.MarkBisect(path, "good"); err == nil {
		t.Fatal("answering with no bisect running should be refused")
	}
	if err := a.StartBisect(path, ids[7], ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := a.StartBisect(path, ids[7], ids[0]); err == nil || !strings.Contains(err.Error(), "bisect in progress") {
		t.Fatalf("starting a second bisect: %v", err)
	}
	info, _ := a.Bisect(path)
	if !info.Running || info.Current == "" || info.Steps != 0 || len(info.Marks) != 2 || info.Marks[0].Subject == "" || info.Origin != "main" {
		t.Fatalf("after starting: %+v", info)
	}
	if err := a.UndoBisect(path); err == nil {
		t.Fatal("undo with no answer given should be refused")
	}

	first := info.Current
	if err := a.MarkBisect(path, "skip"); err != nil {
		t.Fatal(err)
	}
	if err := a.UndoBisect(path); err != nil {
		t.Fatal(err)
	}
	if info, _ = a.Bisect(path); info.Current != first || info.Steps != 0 {
		t.Fatalf("after undoing a skip: current %s (want %s), steps %d", info.Current, first, info.Steps)
	}

	for i := 0; i < 8; i++ {
		info, _ = a.Bisect(path)
		if info.FirstBad != "" {
			break
		}
		term := "good"
		if slices.Index(ids, info.Current) >= broke {
			term = "bad"
		}
		if err := a.MarkBisect(path, term); err != nil {
			t.Fatal(err)
		}
	}
	if info.FirstBad != ids[broke] || info.Current != "" || info.Steps < 2 {
		t.Fatalf("answer = %s (want %s), steps %d", info.FirstBad, ids[broke], info.Steps)
	}

	// Stopping is the operation banner's action, and returns to the branch.
	if err := a.RunOperationAction(path, "abort"); err != nil {
		t.Fatal(err)
	}
	if info, _ = a.Bisect(path); info.Running || fx.Head() != ids[7] {
		t.Fatalf("after stopping: running %v, HEAD %s", info.Running, fx.Head())
	}
}

func TestStartBisectNeedsBothEnds(t *testing.T) {
	t.Parallel()
	a, _, path, ids := openForBisect(t, 3)
	if err := a.StartBisect(path, ids[2], ""); err == nil {
		t.Fatal("a bisect with no good commit should be refused")
	}
	if err := a.StartBisect(path, ids[0], ids[2]); err == nil || !strings.Contains(err.Error(), "history") {
		t.Fatalf("good after bad: %v", err)
	}
	if info, _ := a.Bisect(path); info.Running {
		t.Fatal("a refused start mustn't leave a bisect running")
	}
}
