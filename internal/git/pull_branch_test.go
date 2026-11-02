package git

import (
	"context"
	"errors"
	"testing"

	"github.com/risadams/gitrevenge/internal/gittest"
)

// pullBranchFixture is a clone on main with a local feat tracking origin/feat, and a second
// clone that can add to the remote.
func pullBranchFixture(t *testing.T) (*Repo, *gittest.Repo, *gittest.Repo) {
	t.Helper()
	local, remote := gittest.WithRemote(t, 1)
	local.Git("checkout", "-q", "-b", "feat")
	local.CommitFile("feat.txt", "1", "feat 1")
	local.Git("push", "-q", "-u", "origin", "feat")
	local.Git("checkout", "-q", "main")
	other := gittest.Clone(t, remote)
	other.Git("checkout", "-q", "feat")
	return OpenRepo(newTestRunner(t), local.Dir), local, other
}

func TestPullBranchFastForwardsWithoutTouchingTheWorkingTree(t *testing.T) {
	repo, local, other := pullBranchFixture(t)
	tip := other.CommitFile("feat2.txt", "2", "feat 2")
	other.Git("push", "-q")

	// A staged change, and an untracked file, on the checked-out branch.
	local.WriteFile("staged.txt", "s")
	local.Git("add", "staged.txt")
	local.WriteFile("loose.txt", "u")
	before := local.Git("rev-parse", "feat")
	status := local.Git("status", "--short")

	from, to, err := repo.PullBranch(context.Background(), "feat", NetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if from != before || to != tip {
		t.Fatalf("moved %s..%s, want %s..%s", from, to, before, tip)
	}
	if local.Git("rev-parse", "feat") != tip {
		t.Fatal("feat did not move")
	}
	if got := local.Git("symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Fatalf("HEAD moved to %s", got)
	}
	if got := local.Git("status", "--short"); got != status {
		t.Fatalf("status changed:\n%s\nwas:\n%s", got, status)
	}
}

func TestPullBranchFollowsTheUpstreamName(t *testing.T) {
	repo, local, other := pullBranchFixture(t)
	other.Git("checkout", "-q", "-b", "team/other-name")
	tip := other.CommitFile("o.txt", "o", "other name")
	other.Git("push", "-q", "origin", "team/other-name")
	local.Git("fetch", "-q", "origin")
	local.Git("branch", "-q", "--set-upstream-to=origin/team/other-name", "feat")

	if _, to, err := repo.PullBranch(context.Background(), "feat", NetOptions{}); err != nil || to != tip {
		t.Fatalf("to = %s, want %s, err %v", to, tip, err)
	}
}

func TestPullBranchLeavesABranchWithNothingToPull(t *testing.T) {
	repo, local, _ := pullBranchFixture(t)
	ctx := context.Background()
	if from, to, err := repo.PullBranch(ctx, "feat", NetOptions{}); err != nil || from != to {
		t.Fatalf("up to date: %s..%s, %v", from, to, err)
	}
	// Commits only on the local side: nothing to pull, and it hasn't diverged.
	local.Git("checkout", "-q", "feat")
	ahead := local.CommitFile("mine.txt", "m", "mine")
	local.Git("checkout", "-q", "main")
	if from, to, err := repo.PullBranch(ctx, "feat", NetOptions{}); err != nil || from != ahead || to != ahead {
		t.Fatalf("ahead: %s..%s, %v", from, to, err)
	}
}

func TestPullBranchRefuses(t *testing.T) {
	ctx := context.Background()

	t.Run("diverged", func(t *testing.T) {
		repo, local, other := pullBranchFixture(t)
		other.CommitFile("theirs.txt", "t", "theirs")
		other.Git("push", "-q")
		local.Git("checkout", "-q", "feat")
		mine := local.CommitFile("mine.txt", "m", "mine")
		local.Git("checkout", "-q", "main")
		if _, _, err := repo.PullBranch(ctx, "feat", NetOptions{}); !errors.Is(err, ErrBranchDiverged) {
			t.Fatalf("err = %v", err)
		}
		if local.Git("rev-parse", "feat") != mine {
			t.Fatal("feat moved")
		}
	})

	t.Run("checked out", func(t *testing.T) {
		repo, local, other := pullBranchFixture(t)
		other.CommitFile("theirs.txt", "t", "theirs")
		other.Git("push", "-q")
		local.Git("checkout", "-q", "feat")
		if _, _, err := repo.PullBranch(ctx, "feat", NetOptions{}); !errors.Is(err, ErrBranchInUse) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("being rebased", func(t *testing.T) {
		repo, local, other := pullBranchFixture(t)
		other.CommitFile("theirs.txt", "t", "theirs")
		other.Git("push", "-q")
		local.CommitFile("clash.txt", "main", "main side")
		local.Git("checkout", "-q", "feat")
		local.CommitFile("clash.txt", "feat", "feat side")
		if _, err := local.TryGit("rebase", "main"); err == nil {
			t.Fatal("expected the rebase to stop on a conflict")
		}
		if _, _, err := repo.PullBranch(ctx, "feat", NetOptions{}); !errors.Is(err, ErrBranchInUse) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("no upstream", func(t *testing.T) {
		repo, local, _ := pullBranchFixture(t)
		local.Git("branch", "-q", "alone")
		if _, _, err := repo.PullBranch(ctx, "alone", NetOptions{}); !errors.Is(err, ErrNoUpstream) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("upstream deleted on the remote", func(t *testing.T) {
		repo, _, other := pullBranchFixture(t)
		other.Git("checkout", "-q", "main")
		other.Git("push", "-q", "origin", "--delete", "feat")
		if _, _, err := repo.PullBranch(ctx, "feat", NetOptions{}); !errors.Is(err, ErrUpstreamGone) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestMoveBranch(t *testing.T) {
	repo, local, _ := pullBranchFixture(t)
	ctx := context.Background()
	tip := local.Git("rev-parse", "feat")
	base := local.Git("rev-parse", "feat~1")
	if err := repo.MoveBranch(ctx, "feat", tip, base); err != nil {
		t.Fatal(err)
	}
	if local.Git("rev-parse", "feat") != base {
		t.Fatal("feat did not move")
	}
	if err := repo.MoveBranch(ctx, "feat", tip, base); !errors.Is(err, ErrBranchMoved) {
		t.Fatalf("stale move: %v", err)
	}
	if local.Git("rev-parse", "feat") != base {
		t.Fatal("a stale move changed the branch")
	}
}
