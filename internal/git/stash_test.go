package git

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestStashSaveAndPop(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "base\n", "base")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if err := repo.StashSave(ctx, "nothing", false); !errors.Is(err, ErrNothingToStash) {
		t.Fatalf("empty stash err = %v", err)
	}
	fx.WriteFile("a.txt", "changed\n")
	fx.WriteFile("untracked.txt", "u\n")
	if err := repo.StashSave(ctx, "wip one", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fx.Path("untracked.txt")); err != nil {
		t.Fatal("untracked file should stay when not included")
	}
	if b, _ := os.ReadFile(fx.Path("a.txt")); string(b) != "base\n" {
		t.Fatal("tracked change should be stashed")
	}
	ids, _ := repo.StashIDs(ctx)
	if len(ids) != 1 {
		t.Fatalf("stash ids = %v", ids)
	}
	if err := repo.StashPop(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(fx.Path("a.txt")); string(b) != "changed\n" {
		t.Fatal("pop should restore the change")
	}
	if ids, _ := repo.StashIDs(ctx); len(ids) != 0 {
		t.Fatal("pop should drop the stash")
	}
}

func TestStashIncludeUntrackedApplyDrop(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	fx.WriteFile("new.txt", "n\n")
	if err := repo.StashSave(ctx, "", false); !errors.Is(err, ErrNothingToStash) {
		t.Fatalf("only untracked without include: err = %v", err)
	}
	if err := repo.StashSave(ctx, "with untracked", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fx.Path("new.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file should be stashed")
	}
	ids, _ := repo.StashIDs(ctx)
	if err := repo.StashApply(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fx.Path("new.txt")); err != nil {
		t.Fatal("apply should restore untracked files")
	}
	if ids, _ := repo.StashIDs(ctx); len(ids) != 1 {
		t.Fatal("apply must keep the stash")
	}
	if err := repo.StashDrop(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	if ids, _ := repo.StashIDs(ctx); len(ids) != 0 {
		t.Fatal("drop should remove the stash")
	}
}

func TestStashActionsFollowRenumbering(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "0\n", "base")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	fx.WriteFile("a.txt", "first\n")
	_ = repo.StashSave(ctx, "first", false)
	first, _ := repo.StashIDs(ctx)
	fx.WriteFile("a.txt", "second\n")
	_ = repo.StashSave(ctx, "second", false)

	// "first" is now stash@{1}; dropping it by ID must not drop "second".
	if err := repo.StashDrop(ctx, first[0]); err != nil {
		t.Fatal(err)
	}
	if msg := fx.Git("stash", "list"); msg == "" || !strings.Contains(msg, "second") || strings.Contains(msg, "first") {
		t.Fatalf("stash list = %q", msg)
	}
	if err := repo.StashDrop(ctx, first[0]); !errors.Is(err, ErrStashNotFound) {
		t.Fatalf("dropping again: err = %v", err)
	}
}

func TestStashPopConflictKeepsStash(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "base\n", "base")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	fx.WriteFile("a.txt", "stashed\n")
	_ = repo.StashSave(ctx, "s", false)
	fx.CommitFile("a.txt", "committed meanwhile\n", "conflicting")

	ids, _ := repo.StashIDs(ctx)
	if err := repo.StashPop(ctx, ids[0]); !errors.Is(err, ErrStashConflict) {
		t.Fatalf("err = %v", err)
	}
	if ids, _ := repo.StashIDs(ctx); len(ids) != 1 {
		t.Fatal("a conflicted pop must keep the stash")
	}
}
