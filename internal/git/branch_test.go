package git

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func currentBranch(t *testing.T, fx *gittest.Repo) string {
	t.Helper()
	out, _ := fx.TryGit("symbolic-ref", "--short", "-q", "HEAD")
	return out
}

func TestCreateRenameDeleteBranch(t *testing.T) {
	fx := gittest.New(t)
	shas := fx.Linear(2)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if err := repo.CreateBranch(ctx, "bad..name", "", false); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("invalid name err = %v", err)
	}
	if err := repo.CreateBranch(ctx, "-oops", "", false); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("leading dash err = %v", err)
	}
	if err := repo.CreateBranch(ctx, "feature/a", shas[0], false); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("rev-parse", "feature/a"); got != shas[0] || currentBranch(t, fx) != "main" {
		t.Fatalf("feature/a at %s, on %s", got, currentBranch(t, fx))
	}
	if err := repo.CreateBranch(ctx, "work", "", true); err != nil {
		t.Fatal(err)
	}
	if currentBranch(t, fx) != "work" {
		t.Fatal("create with checkout should switch")
	}
	if err := repo.RenameBranch(ctx, "feature/a", "feature/b"); err != nil {
		t.Fatal(err)
	}
	if !repo.refExists(ctx, "refs/heads/feature/b") || repo.refExists(ctx, "refs/heads/feature/a") {
		t.Fatal("rename failed")
	}
	if err := repo.DeleteBranch(ctx, "work", false); !errors.Is(err, ErrCurrentBranch) {
		t.Fatalf("deleting the current branch: err = %v", err)
	}
	if err := repo.DeleteBranch(ctx, "feature/b", false); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteUnmergedNeedsForce(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	fx.Branch("topic")
	fx.CommitFile("t.txt", "t", "topic only")
	fx.Checkout("main")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if err := repo.DeleteBranch(ctx, "topic", false); !errors.Is(err, ErrNotMerged) {
		t.Fatalf("err = %v", err)
	}
	if err := repo.DeleteBranch(ctx, "topic", true); err != nil {
		t.Fatal(err)
	}
}

func TestCheckoutKinds(t *testing.T) {
	local, remote := gittest.WithRemote(t, 2)
	seed := gittest.Clone(t, remote)
	seed.Branch("feature")
	seed.CommitFile("f.txt", "f", "feature work")
	seed.Git("push", "-q", "origin", "feature")
	local.Git("fetch", "-q")
	repo := OpenRepo(newTestRunner(t), local.Dir)
	ctx := context.Background()

	// A remote branch creates a tracking local branch.
	if err := repo.Checkout(ctx, "origin/feature"); err != nil {
		t.Fatal(err)
	}
	if currentBranch(t, local) != "feature" || local.Git("rev-parse", "--abbrev-ref", "feature@{upstream}") != "origin/feature" {
		t.Fatal("expected local feature tracking origin/feature")
	}
	// A local branch is switched to.
	if err := repo.Checkout(ctx, "main"); err != nil || currentBranch(t, local) != "main" {
		t.Fatalf("checkout main: %v", err)
	}
	// Checking out the remote again reuses the existing local branch.
	if err := repo.Checkout(ctx, "origin/feature"); err != nil || currentBranch(t, local) != "feature" {
		t.Fatalf("second checkout: %v", err)
	}
	// A commit detaches.
	sha := local.Git("rev-parse", "HEAD~1")
	if err := repo.Checkout(ctx, sha); err != nil || currentBranch(t, local) != "" || local.Head() != sha {
		t.Fatalf("detached checkout: %v", err)
	}
}

func TestCheckoutRefusesToLoseLocalChanges(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "main\n", "base")
	fx.Branch("other")
	fx.CommitFile("a.txt", "other\n", "other change")
	fx.Checkout("main")
	fx.WriteFile("a.txt", "local edit\n")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	var gerr *Error
	if err := repo.Checkout(context.Background(), "other"); !errors.As(err, &gerr) {
		t.Fatalf("err = %v", err)
	}
	if currentBranch(t, fx) != "main" {
		t.Fatal("checkout must not happen when it would overwrite local changes")
	}
}

func TestSetAndUnsetUpstream(t *testing.T) {
	local, _ := gittest.WithRemote(t, 1)
	local.Branch("topic")
	repo := OpenRepo(newTestRunner(t), local.Dir)
	ctx := context.Background()
	if err := repo.SetUpstream(ctx, "topic", "origin/main"); err != nil {
		t.Fatal(err)
	}
	if got := local.Git("rev-parse", "--abbrev-ref", "topic@{upstream}"); got != "origin/main" {
		t.Fatalf("upstream = %q", got)
	}
	if err := repo.SetUpstream(ctx, "topic", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := local.TryGit("rev-parse", "--abbrev-ref", "topic@{upstream}"); err == nil {
		t.Fatal("upstream should be unset")
	}
}

func TestTags(t *testing.T) {
	fx := gittest.New(t)
	shas := fx.Linear(2)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if err := repo.CreateTag(ctx, "light", shas[0], ""); err != nil {
		t.Fatal(err)
	}
	if fx.Git("cat-file", "-t", "light") != "commit" {
		t.Fatal("lightweight tag should point straight at the commit")
	}
	if err := repo.CreateTag(ctx, "v1.0", "", "Release 1.0\n\n#1 notes kept"); err != nil {
		t.Fatal(err)
	}
	if fx.Git("cat-file", "-t", "v1.0") != "tag" || fx.Git("rev-parse", "v1.0^{commit}") != shas[1] {
		t.Fatal("annotated tag at HEAD expected")
	}
	if msg := fx.Git("tag", "-l", "-n9", "v1.0"); !strings.Contains(msg, "Release 1.0") || !strings.Contains(msg, "#1 notes kept") {
		t.Fatalf("tag message = %q", msg)
	}
	if err := repo.CreateTag(ctx, "bad tag", "", ""); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("invalid tag err = %v", err)
	}
	if err := repo.DeleteTag(ctx, "light"); err != nil {
		t.Fatal(err)
	}
	if repo.refExists(ctx, "refs/tags/light") {
		t.Fatal("tag not deleted")
	}
}
