package git

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// mergeInProgress leaves main merging "topic": f.txt conflicts on its second line, and
// clean.txt comes in from topic without one.
func mergeInProgress(t *testing.T) *gittest.Repo {
	t.Helper()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "one\ntwo\nthree\n", "Add the file")
	fx.Branch("topic")
	fx.CommitFile("f.txt", "one\ntwo theirs\nthree\n", "Change two on topic")
	fx.CommitFile("clean.txt", "clean\n", "Add a clean file")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "one\ntwo ours\nthree\n", "Change two on main")
	if _, err := fx.TryGit("merge", "topic"); err == nil {
		t.Fatal("expected a merge conflict")
	}
	return fx
}

func TestPlaceOriginNamesWhoLastChangedEachSide(t *testing.T) {
	t.Parallel()
	fx := mergeInProgress(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	gitDir := fx.Path(".git")

	for side, want := range map[string]string{"ours": "Change two on main", "theirs": "Change two on topic", "base": "Add the file"} {
		o, err := repo.PlaceOrigin(ctx, gitDir, "f.txt", side, 2, 2)
		if err != nil || o.Subject != want || o.Author == "" || len(o.Commit) != 40 {
			t.Errorf("%s: origin = %+v, %v; want %q", side, o, err, want)
		}
	}
	// Line one was never touched after the file was added, on either side.
	if o, _ := repo.PlaceOrigin(ctx, gitDir, "f.txt", "theirs", 1, 1); o.Subject != "Add the file" {
		t.Errorf("line 1 on theirs: %+v", o)
	}
	// A range of several lines names the newest commit among them.
	if o, _ := repo.PlaceOrigin(ctx, gitDir, "f.txt", "ours", 1, 3); o.Subject != "Change two on main" {
		t.Errorf("lines 1-3 on ours: %+v", o)
	}
	// Lines the side doesn't have, a file it doesn't have, and no range at all: nothing to say.
	for _, o := range []PlaceOrigin{
		must(repo.PlaceOrigin(ctx, gitDir, "f.txt", "ours", 40, 41)),
		must(repo.PlaceOrigin(ctx, gitDir, "clean.txt", "ours", 1, 1)),
		must(repo.PlaceOrigin(ctx, gitDir, "f.txt", "ours", 0, 0)),
	} {
		if o != (PlaceOrigin{}) {
			t.Errorf("expected no origin, got %+v", o)
		}
	}
}

func must(o PlaceOrigin, err error) PlaceOrigin {
	if err != nil {
		panic(err)
	}
	return o
}

func TestPlaceOriginDuringARebaseAndWithNothingInProgress(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "one\ntwo\n", "Add the file")
	fx.Branch("topic")
	fx.CommitFile("f.txt", "one\ntwo topic\n", "Change two on topic")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "one\ntwo main\n", "Change two on main")
	fx.Checkout("topic")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	gitDir := fx.Path(".git")

	if o, err := repo.PlaceOrigin(ctx, gitDir, "f.txt", "ours", 2, 2); err != nil || o != (PlaceOrigin{}) {
		t.Fatalf("with nothing in progress: %+v, %v", o, err)
	}
	if _, err := fx.TryGit("rebase", "main"); err == nil {
		t.Fatal("expected a rebase conflict")
	}
	// In a rebase git's "ours" is what's being rebased onto, and "theirs" the commit replayed.
	for side, want := range map[string]string{"ours": "Change two on main", "theirs": "Change two on topic", "base": "Add the file"} {
		if o, err := repo.PlaceOrigin(ctx, gitDir, "f.txt", side, 2, 2); err != nil || o.Subject != want {
			t.Errorf("%s: origin = %+v, %v; want %q", side, o, err, want)
		}
	}
}

func TestMergedCleanlyLeavesOutTheConflictedFiles(t *testing.T) {
	t.Parallel()
	fx := mergeInProgress(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if got := repo.MergedCleanly(context.Background()); !slices.Equal(got, []string{"clean.txt"}) {
		t.Fatalf("merged cleanly = %v", got)
	}
}

func TestMergeToolRunsTheConfiguredToolAndStagesItsResult(t *testing.T) {
	t.Parallel()
	fx := mergeInProgress(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if name := repo.MergeToolName(ctx); name != "" {
		t.Fatalf("merge tool = %q before any is set", name)
	}
	if err := repo.RunMergeTool(ctx, "f.txt"); !errors.Is(err, ErrNoMergeTool) {
		t.Fatalf("err = %v, want ErrNoMergeTool", err)
	}

	// A stand-in tool that takes their version, as a real one would after the user chose it.
	fx.Git("config", "merge.tool", "takeit")
	fx.Git("config", "mergetool.takeit.cmd", `cp -- "$REMOTE" "$MERGED"`)
	fx.Git("config", "mergetool.takeit.trustExitCode", "true")
	fx.Git("config", "mergetool.keepBackup", "false")
	if name := repo.MergeToolName(ctx); name != "takeit" {
		t.Fatalf("merge tool = %q", name)
	}
	if err := repo.RunMergeTool(ctx, "f.txt"); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("diff", "--name-only", "--diff-filter=U"); got != "" {
		t.Fatalf("still unmerged: %q", got)
	}
	if got := fx.Git("show", ":f.txt"); got != "one\ntwo theirs\nthree" {
		t.Fatalf("staged result = %q", got)
	}
}
