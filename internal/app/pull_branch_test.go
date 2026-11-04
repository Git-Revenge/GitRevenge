package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// pullBranchApp opens a clone on main with a local feat tracking origin/feat, and returns a
// second clone that can add to the remote.
func pullBranchApp(t *testing.T) (*App, string, *gittest.Repo, *gittest.Repo) {
	t.Helper()
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	local.Git("checkout", "-q", "-b", "feat")
	local.CommitFile("feat.txt", "1", "feat 1")
	local.Git("push", "-q", "-u", "origin", "feat")
	local.Git("checkout", "-q", "main")
	other := gittest.Clone(t, remote)
	other.Git("checkout", "-q", "feat")
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return a, tab.Path, local, other
}

func TestPullBranchNoticesAndUndo(t *testing.T) {
	a, path, local, other := pullBranchApp(t)
	var notices []string
	a.emitHook = func(name string, data any) {
		if n, ok := data.(Notice); ok && name == "notice" {
			notices = append(notices, n.Message)
		}
	}
	start := local.Git("rev-parse", "feat")
	tip := other.CommitFile("two.txt", "2", "two")
	other.Git("push", "-q")

	if err := a.PullBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}
	if local.Git("rev-parse", "feat") != tip {
		t.Fatal("feat did not move")
	}
	state, _ := a.UndoState(path)
	if state.Undo != "Pull feat" {
		t.Fatalf("undo = %q", state.Undo)
	}

	if err := a.Undo(path); err != nil {
		t.Fatal(err)
	}
	if local.Git("rev-parse", "feat") != start {
		t.Fatal("undo did not move feat back")
	}
	if err := a.Redo(path); err != nil {
		t.Fatal(err)
	}
	if local.Git("rev-parse", "feat") != tip {
		t.Fatal("redo did not move feat forward")
	}

	if err := a.PullBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}
	if len(notices) != 2 || !strings.Contains(notices[0], "into feat") || notices[1] != "feat is already up to date." {
		t.Fatalf("notices = %q", notices)
	}
}

func TestUndoPullBranchRefusesWhenTheBranchMoved(t *testing.T) {
	a, path, local, other := pullBranchApp(t)
	other.CommitFile("two.txt", "2", "two")
	other.Git("push", "-q")
	if err := a.PullBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}

	local.Git("checkout", "-q", "feat")
	if err := a.Undo(path); err == nil || !strings.Contains(err.Error(), "while it is checked out") {
		t.Fatalf("checked out: %v", err)
	}
	mine := local.CommitFile("mine.txt", "m", "mine")
	local.Git("checkout", "-q", "main")
	if err := a.Undo(path); !errors.Is(err, errMoved) {
		t.Fatalf("moved: %v", err)
	}
	if local.Git("rev-parse", "feat") != mine {
		t.Fatal("refused undo moved the branch")
	}
}

func TestPullBranchExplainsRefusals(t *testing.T) {
	t.Run("diverged", func(t *testing.T) {
		a, path, local, other := pullBranchApp(t)
		other.CommitFile("theirs.txt", "t", "theirs")
		other.Git("push", "-q")
		local.Git("checkout", "-q", "feat")
		local.CommitFile("mine.txt", "m", "mine")
		local.Git("checkout", "-q", "main")
		want := "feat has diverged from its upstream. Check it out to merge or rebase."
		if err := a.PullBranch(path, "feat"); err == nil || err.Error() != want {
			t.Fatalf("err = %v", err)
		}
		if state, _ := a.UndoState(path); state.Undo != "" {
			t.Fatalf("a refused pull left undo %q", state.Undo)
		}
	})

	t.Run("checked out here", func(t *testing.T) {
		a, path, _, _ := pullBranchApp(t)
		if err := a.PullBranch(path, "main"); err == nil || !strings.Contains(err.Error(), "Use Pull in the toolbar") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("checked out in another worktree", func(t *testing.T) {
		a, path, local, _ := pullBranchApp(t)
		dir := t.TempDir() + "/wt"
		local.Git("worktree", "add", "-q", dir, "feat")
		err := a.PullBranch(path, "feat")
		if err == nil || !strings.Contains(err.Error(), "is checked out in the worktree at") || !strings.Contains(err.Error(), "wt") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("no upstream", func(t *testing.T) {
		a, path, local, _ := pullBranchApp(t)
		local.Git("branch", "-q", "alone")
		if err := a.PullBranch(path, "alone"); err == nil || !strings.Contains(err.Error(), "no upstream") {
			t.Fatalf("err = %v", err)
		}
	})
}

// Cleanup on pull deletes branches whose upstream is gone. Pulling one other branch shouldn't.
func TestPullBranchDoesNotCleanUp(t *testing.T) {
	a, path, local, other := pullBranchApp(t)
	p, _ := a.RepoPreferences(path)
	p.CleanupOnPull = true
	if err := a.SetRepoPreferences(path, p); err != nil {
		t.Fatal(err)
	}
	local.Git("checkout", "-q", "-b", "done")
	local.CommitFile("done.txt", "d", "done")
	local.Git("push", "-q", "-u", "origin", "done")
	local.Git("checkout", "-q", "main")
	if err := a.Fetch(path, ""); err != nil {
		t.Fatal(err)
	}
	local.Git("push", "-q", "origin", "--delete", "done")
	other.CommitFile("two.txt", "2", "two")
	other.Git("push", "-q")

	if err := a.PullBranch(path, "feat"); err != nil {
		t.Fatal(err)
	}
	if _, err := local.TryGit("rev-parse", "--verify", "-q", "refs/heads/done"); err != nil {
		t.Fatal("done was deleted")
	}
}
