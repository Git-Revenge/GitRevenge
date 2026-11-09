package git

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// status returns `git status --porcelain` and HEAD, to show a forecast changed nothing.
func untouched(t *testing.T, fx *gittest.Repo) string {
	t.Helper()
	return fx.Git("status", "--porcelain") + "|" + fx.Head() + "|" + fx.Git("for-each-ref", "--format=%(refname) %(objectname)")
}

func TestForecastMergeClean(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "base")
	fx.Branch("topic")
	fx.CommitFile("b.txt", "b\n", "Add b")
	fx.CommitFile("c.txt", "c\n", "Add c")
	fx.Checkout("main")
	fx.CommitFile("d.txt", "d\n", "Add d")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	before := untouched(t, fx)

	f, err := repo.ForecastMerge(context.Background(), "main", "topic")
	if err != nil {
		t.Fatal(err)
	}
	if f.AlreadyMerged || f.FastForward || f.Incomplete || len(f.Conflicts) != 0 {
		t.Fatalf("forecast = %+v", f)
	}
	if f.CommitCount != 2 || f.Files != 2 || len(f.Commits) != 2 || f.Commits[0].Subject != "Add c" || f.Commits[0].Author == "" {
		t.Fatalf("commits %d, files %d, list %+v", f.CommitCount, f.Files, f.Commits)
	}
	if after := untouched(t, fx); after != before {
		t.Fatalf("the forecast changed the repository:\n%s\n%s", before, after)
	}
}

func TestForecastMergeFastForwardAndAlreadyMerged(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "base")
	fx.Branch("topic")
	fx.CommitFile("b.txt", "b\n", "Add b")
	fx.Checkout("main")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	f, err := repo.ForecastMerge(ctx, "main", "topic")
	if err != nil || !f.FastForward || f.CommitCount != 1 || f.Files != 1 || len(f.Conflicts) != 0 {
		t.Fatalf("fast-forward forecast = %+v, %v", f, err)
	}
	// The other way round there's nothing to bring in.
	f, err = repo.ForecastMerge(ctx, "topic", "main")
	if err != nil || !f.AlreadyMerged || f.CommitCount != 0 {
		t.Fatalf("already-merged forecast = %+v, %v", f, err)
	}
}

func TestForecastMergeConflicts(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\n", "base")
	fx.CommitFile("g.txt", "g\n", "g")
	fx.Branch("topic")
	fx.CommitFile("f.txt", "ONE theirs\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nNINE theirs\n", "theirs")
	fx.CommitFile("g.txt", "g theirs\n", "g theirs")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "ONE ours\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nNINE ours\n", "ours")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	before := untouched(t, fx)

	f, err := repo.ForecastMerge(context.Background(), "main", "topic")
	if err != nil {
		t.Fatal(err)
	}
	if f.FastForward || f.Incomplete || len(f.Conflicts) != 1 {
		t.Fatalf("forecast = %+v", f)
	}
	// Two separate places in the one file; g.txt changed on one side only and merges.
	if c := f.Conflicts[0]; c.Path != "f.txt" || c.Places != 2 {
		t.Fatalf("conflict = %+v, want f.txt in 2 places", c)
	}
	if after := untouched(t, fx); after != before {
		t.Fatalf("the forecast changed the repository:\n%s\n%s", before, after)
	}
}

func TestForecastRebaseStopsWhereTheRealOneWould(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("topic")
	first := fx.CommitFile("a.txt", "a\n", "Add a")
	fx.CommitFile("b.txt", "b\n", "Add b")
	third := fx.CommitFile("f.txt", "theirs\n", "Change f")
	fx.CommitFile("c.txt", "c\n", "Add c")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "ours\n", "Change f on main")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	before := untouched(t, fx)

	f, err := repo.ForecastRebase(context.Background(), "topic", "main")
	if err != nil {
		t.Fatal(err)
	}
	if f.UpToDate || f.Incomplete || f.CommitCount != 4 || len(f.Commits) != 4 || f.Commits[0].ID != first {
		t.Fatalf("forecast = %+v", f)
	}
	if f.StopsAt != third || len(f.Conflicts) != 1 || f.Conflicts[0].Path != "f.txt" || f.Conflicts[0].Places != 1 {
		t.Fatalf("stops at %s (want %s) on %+v", f.StopsAt, third, f.Conflicts)
	}
	if f.Pushed || f.Upstream != "" {
		t.Fatalf("pushed %v, upstream %q; the branch has no upstream", f.Pushed, f.Upstream)
	}
	if after := untouched(t, fx); after != before {
		t.Fatalf("the forecast changed the repository:\n%s\n%s", before, after)
	}

	// The real rebase stops on the same commit.
	fx.Checkout("topic")
	if _, err := fx.TryGit("rebase", "main"); err == nil {
		t.Fatal("the real rebase was expected to stop on a conflict")
	}
	if stopped := fx.Git("rev-parse", "REBASE_HEAD"); stopped != third {
		t.Fatalf("the real rebase stopped at %s, the forecast said %s", stopped, third)
	}
}

func TestForecastRebaseSeesLaterCommitsOnTopOfEarlierOnes(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("topic")
	// Each commit builds on the one before in the same file: tried one by one against main
	// alone they'd conflict, but in order they apply.
	fx.CommitFile("f.txt", "base\none\n", "One")
	fx.CommitFile("f.txt", "base\none\ntwo\n", "Two")
	fx.CommitFile("f.txt", "base\none\ntwo\nthree\n", "Three")
	fx.Checkout("main")
	fx.CommitFile("other.txt", "x\n", "Elsewhere")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	f, err := repo.ForecastRebase(context.Background(), "topic", "main")
	if err != nil {
		t.Fatal(err)
	}
	if f.StopsAt != "" || len(f.Conflicts) != 0 || f.Incomplete || f.CommitCount != 3 {
		t.Fatalf("forecast = %+v, want a clean rebase of 3", f)
	}
}

func TestForecastRebaseFixupsUpToDateAndPushed(t *testing.T) {
	t.Parallel()
	local, _ := gittest.WithRemote(t, 1)
	local.Branch("topic")
	target := local.CommitFile("a.txt", "a\n", "Cache readings")
	local.CommitFile("b.txt", "b\n", "wip")
	local.CommitFile("a.txt", "a2\n", "fixup! Cache readings")
	local.Git("push", "-q", "-u", "origin", "topic")
	local.CommitFile("c.txt", "c\n", "Not pushed yet")
	repo := OpenRepo(newTestRunner(t), local.Dir)
	ctx := context.Background()

	// On top of main already: nothing to replay, but the list and the fixups are still useful.
	f, err := repo.ForecastRebase(ctx, "topic", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !f.UpToDate || f.CommitCount != 4 || f.StopsAt != "" {
		t.Fatalf("forecast = %+v", f)
	}
	var fixups []string
	for _, c := range f.Commits {
		fixups = append(fixups, c.FixupOf)
	}
	if want := []string{"", "", target, ""}; !slices.Equal(fixups, want) {
		t.Fatalf("fixups = %v, want %v", fixups, want)
	}
	if !f.Pushed || f.Upstream != "origin/topic" {
		t.Fatalf("pushed %v, upstream %q", f.Pushed, f.Upstream)
	}

	// Nothing to rebase at all.
	f, err = repo.ForecastRebase(ctx, "main", "topic")
	if err != nil || !f.UpToDate || f.CommitCount != 0 {
		t.Fatalf("empty forecast = %+v, %v", f, err)
	}
}

func TestForecastRebaseGivesUpPastTheCap(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("topic")
	for i := range ForecastReplayCap + 3 {
		fx.CommitFile(fmt.Sprintf("n%d.txt", i), "x\n", fmt.Sprintf("Commit %d", i))
	}
	fx.Checkout("main")
	fx.CommitFile("other.txt", "x\n", "Elsewhere")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	f, err := repo.ForecastRebase(context.Background(), "topic", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Incomplete || f.StopsAt != "" || f.CommitCount != ForecastReplayCap+3 {
		t.Fatalf("forecast = incomplete %v, stops %q, count %d", f.Incomplete, f.StopsAt, f.CommitCount)
	}
}

func TestFixupTarget(t *testing.T) {
	t.Parallel()
	for subject, want := range map[string]string{
		"fixup! Cache readings":        "Cache readings",
		"squash! Cache readings":       "Cache readings",
		"amend! Cache readings":        "Cache readings",
		"fixup! fixup! Cache readings": "Cache readings",
		"Cache readings":               "",
		"fixup!":                       "",
	} {
		if got := fixupTarget(subject); got != want {
			t.Errorf("fixupTarget(%q) = %q, want %q", subject, got, want)
		}
	}
}
