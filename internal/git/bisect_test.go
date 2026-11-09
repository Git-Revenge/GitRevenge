package git

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func bisectState(t *testing.T, fx *gittest.Repo) BisectState {
	t.Helper()
	st, err := OpenRepo(newTestRunner(t), fx.Dir).BisectState(context.Background(), fx.Path(".git"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// answer marks the commit under test by where it sits: at or after the breaking commit is bad.
func answer(t *testing.T, fx *gittest.Repo, repo *Repo, commits []string, broke int) {
	t.Helper()
	st := bisectState(t, fx)
	term := BisectGood
	if slices.Index(commits, st.Current) >= broke {
		term = BisectBad
	}
	if err := repo.BisectMark(context.Background(), fx.Path(".git"), term); err != nil {
		t.Fatal(err)
	}
}

func TestBisectFindsTheFirstBadCommit(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	commits := fx.Linear(10)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	const broke = 6

	if st := bisectState(t, fx); st.Running {
		t.Fatal("a fresh repository isn't bisecting")
	}
	if err := repo.BisectStart(ctx, commits[9], commits[0]); err != nil {
		t.Fatal(err)
	}
	st := bisectState(t, fx)
	if !st.Running || st.Current == "" || st.FirstBad != "" {
		t.Fatalf("after starting: %+v", st)
	}
	if len(st.Marks) != 2 || st.Marks[0] != (BisectMark{Commit: commits[9], Term: BisectBad}) || st.Marks[1] != (BisectMark{Commit: commits[0], Term: BisectGood}) {
		t.Fatalf("marks = %+v", st.Marks)
	}
	if st.Remaining < 1 || st.StepsLeft < 2 || st.Origin != "main" {
		t.Fatalf("remaining %d, steps left %d, origin %q", st.Remaining, st.StepsLeft, st.Origin)
	}
	// Everything after the good commit up to the bad one could be it, newest first.
	if want := []string{commits[9], commits[8], commits[7], commits[6], commits[5], commits[4], commits[3], commits[2], commits[1]}; !slices.Equal(st.InQuestion, want) || st.RangeCapped {
		t.Fatalf("in question = %v", st.InQuestion)
	}

	for i := 0; i < 10 && bisectState(t, fx).FirstBad == ""; i++ {
		answer(t, fx, repo, commits, broke)
	}
	st = bisectState(t, fx)
	if st.FirstBad != commits[broke] {
		t.Fatalf("first bad = %s, want %s", st.FirstBad, commits[broke])
	}
	if !st.Running || st.Current != "" || st.Steps() < 2 || st.Steps() > 4 {
		t.Fatalf("after finding it: running %v, current %q, steps %d", st.Running, st.Current, st.Steps())
	}
	// Each answer is numbered in the order it was given.
	for i, m := range st.Marks[2:] {
		if m.Step != i+1 {
			t.Fatalf("mark %d has step %d: %+v", i, m.Step, st.Marks)
		}
	}

	// Stopping goes back to where it started, and leaves nothing behind.
	if err := repo.RunAction(ctx, fx.Path(".git"), Abort); err != nil {
		t.Fatal(err)
	}
	if st := bisectState(t, fx); st.Running {
		t.Fatalf("after stopping: %+v", st)
	}
	if fx.Head() != commits[9] {
		t.Fatalf("HEAD = %s, want the commit it started on", fx.Head())
	}
}

func TestBisectUndoTakesBackTheLastAnswer(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	commits := fx.Linear(10)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	gitDir := fx.Path(".git")
	if err := repo.BisectStart(ctx, commits[9], commits[0]); err != nil {
		t.Fatal(err)
	}
	if err := repo.BisectUndo(ctx, gitDir); !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("undo before any answer: %v, want ErrNothingToUndo", err)
	}
	first := bisectState(t, fx).Current
	if err := repo.BisectMark(ctx, gitDir, BisectBad); err != nil {
		t.Fatal(err)
	}
	if got := bisectState(t, fx); got.Steps() != 1 || got.Current == first {
		t.Fatalf("after one answer: steps %d, current %s", got.Steps(), got.Current)
	}
	if err := repo.BisectUndo(ctx, gitDir); err != nil {
		t.Fatal(err)
	}
	st := bisectState(t, fx)
	if st.Steps() != 0 || st.Current != first || len(st.Marks) != 2 {
		t.Fatalf("after undo: steps %d, current %s (want %s), marks %+v", st.Steps(), st.Current, first, st.Marks)
	}
}

func TestBisectSkipCanLeaveSeveralCandidates(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	commits := fx.Linear(3)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	if err := repo.BisectStart(ctx, commits[2], commits[0]); err != nil {
		t.Fatal(err)
	}
	// The only commit to test can't be tested, so git can't say which of two it is.
	if err := repo.BisectMark(ctx, fx.Path(".git"), BisectSkip); err != nil {
		t.Fatal(err)
	}
	st := bisectState(t, fx)
	if st.FirstBad != "" || len(st.Candidates) != 2 {
		t.Fatalf("first bad %q, candidates %v; want two candidates", st.FirstBad, st.Candidates)
	}
	if !slices.Contains(st.Candidates, commits[1]) || !slices.Contains(st.Candidates, commits[2]) {
		t.Fatalf("candidates = %v", st.Candidates)
	}
}

func TestBisectStartRefusesAGoodCommitThatIsNotBefore(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	commits := fx.Linear(3)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if err := repo.BisectStart(context.Background(), commits[0], commits[2]); !errors.Is(err, ErrBisectOrder) {
		t.Fatalf("err = %v, want ErrBisectOrder", err)
	}
	if st := bisectState(t, fx); st.Running {
		t.Fatal("a refused start mustn't leave a bisect running")
	}
}

func TestBisectRangeAndSuggestion(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	commits := fx.Linear(9)
	fx.Git("tag", "v1", commits[2])
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	n, steps, err := repo.BisectRange(ctx, commits[8], commits[0])
	if err != nil || n != 8 || steps != 3 {
		t.Fatalf("range = %d commits, %d steps, %v; want 8 and 3", n, steps, err)
	}
	if got := repo.BisectSuggestGood(ctx, commits[8]); got != "v1" {
		t.Fatalf("suggestion = %q, want v1", got)
	}
	// A tag on the bad commit itself isn't a place where it worked.
	if got := repo.BisectSuggestGood(ctx, commits[2]); got != "" {
		t.Fatalf("suggestion from the tagged commit = %q, want none", got)
	}
}

func TestParseBisectLog(t *testing.T) {
	t.Parallel()
	log := `git bisect start 'bbbb' 'aaaa' 'a2a2'
# status: waiting for both good and bad commits
# bad: [bbbb] Break it
git bisect good cccc
git bisect skip dddd
git bisect bad eeee
# first bad commit: [eeee] Break it
`
	cmds, first, candidates := parseBisectLog(log)
	if first != "eeee" || len(candidates) != 0 {
		t.Fatalf("first bad %q, candidates %v", first, candidates)
	}
	// Since git 2.39 the term is quoted.
	if _, first, _ := parseBisectLog("# first 'bad' commit: [ffff] Break it\n"); first != "ffff" {
		t.Fatalf("quoted first bad = %q", first)
	}
	if _, _, c := parseBisectLog("# possible first 'bad' commit: [1111] a\n# possible first bad commit: [2222] b\n"); !slices.Equal(c, []string{"1111", "2222"}) {
		t.Fatalf("candidates = %v", c)
	}
	want := []BisectMark{
		{Commit: "bbbb", Term: BisectBad},
		{Commit: "aaaa", Term: BisectGood},
		{Commit: "a2a2", Term: BisectGood},
		{Commit: "cccc", Term: BisectGood, Step: 1},
		{Commit: "dddd", Term: BisectSkip, Step: 2},
		{Commit: "eeee", Term: BisectBad, Step: 3},
	}
	if got := marksFrom(cmds); !slices.Equal(got, want) {
		t.Fatalf("marks = %+v", got)
	}
}

func TestBisectLeft(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ n, remaining, steps int }{{1, 0, 0}, {2, 0, 1}, {3, 1, 2}, {8, 3, 3}, {9, 4, 4}} {
		if r, s := bisectLeft(c.n); r != c.remaining || s != c.steps {
			t.Errorf("bisectLeft(%d) = %d, %d; want %d, %d", c.n, r, s, c.remaining, c.steps)
		}
	}
}
