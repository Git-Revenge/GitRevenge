package git

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestPartialKnowsItsPromisor(t *testing.T) {
	t.Parallel()
	repo := OpenRepo(newTestRunner(t), partialClone(t))
	if !repo.Partial(context.Background()) || repo.promisorRemote(context.Background()) != "origin" {
		t.Fatalf("partial = %v, remote = %q", repo.Partial(context.Background()), repo.promisorRemote(context.Background()))
	}
	whole := gittest.New(t)
	whole.CommitFile("a.txt", "a\n", "one")
	if OpenRepo(newTestRunner(t), whole.Dir).Partial(context.Background()) {
		t.Error("a whole repository says it's partial")
	}
}

func TestContentsInAPartialCloneSaysWhatsMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := OpenRepo(newTestRunner(t), partialClone(t))
	old, err := repo.ReadString(ctx, "rev-parse", "HEAD~1:old.txt")
	if err != nil {
		t.Fatal(err)
	}

	fc, err := repo.Contents(ctx, "old.txt", FileRef{Rev: "HEAD~1", Path: "old.txt"}, FileRef{Rev: "HEAD", Path: "old.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if fc.Kind != "missing" || !slices.Equal(fc.Missing, []string{old}) {
		t.Fatalf("kind %q, missing %v; want missing [%s]", fc.Kind, fc.Missing, old)
	}
	if !stillMissing(t, repo) {
		t.Fatal("showing the diff downloaded old.txt")
	}
	// A path that isn't in a version is absent, as before, not missing.
	fc, err = repo.Contents(ctx, "new.txt", FileRef{Rev: "HEAD~1", Path: "new.txt"}, FileRef{Rev: "HEAD", Path: "old.txt"}, false)
	if err != nil || fc.Kind != "text" || fc.Old.Exists {
		t.Fatalf("absent path: %+v, %v", fc, err)
	}
}

func TestDownloadFetchesWhatsMissingInOneRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := OpenRepo(newTestRunner(t), partialClone(t))
	ids, err := repo.FileVersions(ctx, "old.txt", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("versions = %v, want 2", ids)
	}
	missing, err := repo.MissingObjects(ctx, ids)
	if err != nil || len(missing) != 1 {
		t.Fatalf("missing = %v, %v; want the older one", missing, err)
	}
	if err := repo.Download(ctx, missing, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if stillMissing(t, repo) {
		t.Fatal("old.txt is still missing")
	}
	fc, err := repo.Contents(ctx, "old.txt", FileRef{Rev: "HEAD~1", Path: "old.txt"}, FileRef{Rev: "HEAD", Path: "old.txt"}, false)
	if err != nil || fc.Kind != "text" || fc.Old.Text != "km/h once\n" {
		t.Fatalf("after the download: %+v, %v", fc, err)
	}
}

func TestDownloadNeedsAPartialClone(t *testing.T) {
	t.Parallel()
	whole := gittest.New(t)
	whole.CommitFile("a.txt", "a\n", "one")
	err := OpenRepo(newTestRunner(t), whole.Dir).Download(context.Background(), []string{whole.Head()}, NetOptions{})
	if !errors.Is(err, ErrNotPartial) {
		t.Fatalf("err = %v", err)
	}
}

func TestBlameInAPartialCloneDownloadsOnlyWhenAllowed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := OpenRepo(newTestRunner(t), partialClone(t))
	old, err := repo.ReadString(ctx, "rev-parse", "HEAD~1:old.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Blame(ctx, "old.txt", "", false)
	if ids, ok := NotDownloaded(err); !ok || !slices.Equal(ids, []string{old}) {
		t.Fatalf("err = %v; want old.txt's first version named", err)
	}
	if !stillMissing(t, repo) {
		t.Fatal("blame downloaded without being allowed")
	}
	lines, err := repo.Blame(ctx, "old.txt", "", true)
	if err != nil || len(lines) != 1 {
		t.Fatalf("allowed: %v, %v", lines, err)
	}
}

func TestFileVersionsCountsMergesToo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "a\n", "one")
	fx.Branch("side")
	fx.CommitFile("f.txt", "a\nside\n", "side")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "main\na\n", "main")
	// The merge resolves by hand, making a version no other commit has.
	if err := fx.Merge("side"); err != nil {
		fx.WriteFile("f.txt", "resolved\n")
		fx.Git("add", "f.txt")
		fx.Git("commit", "-q", "--no-edit")
	}
	resolved := fx.Git("rev-parse", "HEAD:f.txt")
	ids, err := OpenRepo(newTestRunner(t), fx.Dir).FileVersions(ctx, "f.txt", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ids, resolved) || len(ids) != 4 {
		t.Fatalf("versions = %v; want 4, with the merge's %s", ids, resolved)
	}
	if _, err := OpenRepo(newTestRunner(t), fx.Dir).FileVersions(ctx, "f.txt", "--output=x"); err == nil {
		t.Error("a revision git would read as an option was taken")
	}
}
