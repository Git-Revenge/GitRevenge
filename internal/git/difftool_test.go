package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// fakeDiffTool sets a diff tool that copies the two sides git hands it into out, as local and
// remote, so a test can read what it was given.
func fakeDiffTool(t *testing.T, fx *gittest.Repo) (out string) {
	t.Helper()
	out = t.TempDir()
	o := filepath.ToSlash(out)
	fx.Git("config", "diff.tool", "fake")
	fx.Git("config", "difftool.fake.cmd", `cat "$LOCAL" > "`+o+`/local"; cat "$REMOTE" > "`+o+`/remote"`)
	return out
}

// sides reads what the fake tool was given, and clears it for the next run.
func sides(t *testing.T, out string) (local, remote string) {
	t.Helper()
	l, err := os.ReadFile(filepath.Join(out, "local"))
	if err != nil {
		t.Fatalf("the tool didn't run: %v", err)
	}
	r, _ := os.ReadFile(filepath.Join(out, "remote"))
	_ = os.Remove(filepath.Join(out, "local"))
	_ = os.Remove(filepath.Join(out, "remote"))
	return string(l), string(r)
}

func TestRunDiffToolOpensWhatTheViewShows(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	first := fx.CommitFile("a.txt", "one\n", "one")
	second := fx.CommitFile("a.txt", "two\n", "two")
	fx.WriteFile("a.txt", "three\n")
	fx.Git("add", "a.txt")
	fx.WriteFile("a.txt", "four\n")
	out := fakeDiffTool(t, fx)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	for _, c := range []struct {
		target        DiffToolTarget
		local, remote string
	}{
		{DiffToolTarget{Mode: "unstaged", File: "a.txt"}, "three\n", "four\n"},
		{DiffToolTarget{Mode: "staged", File: "a.txt"}, "two\n", "three\n"},
		{DiffToolTarget{Mode: "commit", File: "a.txt", Commit: second}, "one\n", "two\n"},
		// A first commit is compared with nothing, as the view shows it.
		{DiffToolTarget{Mode: "commit", File: "a.txt", Commit: first}, "", "one\n"},
		{DiffToolTarget{Mode: "compare", File: "a.txt", Commit: first + ".." + second}, "one\n", "two\n"},
		{DiffToolTarget{Mode: "compare", File: "a.txt", Commit: ".." + second}, "", "two\n"},
	} {
		if err := repo.RunDiffTool(ctx, c.target); err != nil {
			t.Fatalf("%+v: %v", c.target, err)
		}
		if local, remote := sides(t, out); local != c.local || remote != c.remote {
			t.Errorf("%+v: got %q → %q, want %q → %q", c.target, local, remote, c.local, c.remote)
		}
	}
}

func TestRunDiffToolPairsARename(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("old.txt", "line one\nline two\nline three\nline four\n", "one")
	fx.Git("mv", "old.txt", "new.txt")
	fx.WriteFile("new.txt", "line one\nline two\nline three\nline 4\n")
	renamed := fx.Commit("rename")
	out := fakeDiffTool(t, fx)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if err := repo.RunDiffTool(context.Background(), DiffToolTarget{Mode: "commit", File: "new.txt", OldPath: "old.txt", Commit: renamed}); err != nil {
		t.Fatal(err)
	}
	if local, remote := sides(t, out); local != "line one\nline two\nline three\nline four\n" || remote != "line one\nline two\nline three\nline 4\n" {
		t.Errorf("got %q → %q", local, remote)
	}
}

func TestDiffToolSetIsGitsChoice(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "a")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	if got := repo.DiffToolSet(ctx); got != (DiffTool{}) {
		t.Fatalf("none set: %+v", got)
	}
	if err := repo.RunDiffTool(ctx, DiffToolTarget{Mode: "unstaged", File: "a.txt"}); !errors.Is(err, ErrNoDiffTool) {
		t.Fatalf("none set: %v", err)
	}
	// difftool falls back to merge.tool, then diff.tool comes first, then diff.guitool, for an
	// app with a window.
	for _, step := range []struct{ key, name string }{{"merge.tool", "kdiff3"}, {"diff.tool", "meld"}, {"diff.guitool", "bc"}} {
		fx.Git("config", step.key, step.name)
		if got := repo.DiffToolSet(ctx); got != (DiffTool{Name: step.name, Key: step.key}) {
			t.Errorf("after %s: %+v", step.key, got)
		}
	}
	if err := repo.RunDiffTool(ctx, DiffToolTarget{Mode: "commit", File: "a.txt", Commit: "-x"}); err == nil {
		t.Error("an option as a commit was taken")
	}
}
