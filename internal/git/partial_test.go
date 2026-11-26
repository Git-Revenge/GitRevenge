package git

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
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

func TestMergeNeedsWhatBothSidesChanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := gittest.New(t)
	src.CommitFile("f.txt", "base\n", "base")
	src.CommitFile("only-main.txt", "m\n", "main only")
	src.Branch("side")
	src.CommitFile("f.txt", "side\n", "side")
	src.Checkout("main")
	src.CommitFile("f.txt", "main\n", "main")
	src.Git("config", "uploadpack.allowFilter", "true")
	dest := filepath.Join(t.TempDir(), "partial")
	src.Git("clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(src.Dir), dest)
	repo := OpenRepo(newTestRunner(t), dest)

	needs, err := repo.MergeNeeds(ctx, "HEAD", "origin/side")
	if err != nil {
		t.Fatal(err)
	}
	// f.txt at the base and on side; main's is checked out. only-main.txt isn't needed.
	want := []string{src.Git("rev-parse", "main~2:f.txt"), src.Git("rev-parse", "side:f.txt")}
	slices.Sort(want)
	if !slices.Equal(needs, want) {
		t.Fatalf("needs %v, want %v", needs, want)
	}
	if err := repo.Download(ctx, needs, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if needs, err := repo.MergeNeeds(ctx, "HEAD", "origin/side"); err != nil || len(needs) != 0 {
		t.Fatalf("after the download: %v, %v", needs, err)
	}
	// With them in, the forecast reads nothing more.
	f, err := repo.ForecastMerge(NoDownloads(ctx), "HEAD", "origin/side")
	if err != nil || f.Incomplete || len(f.Conflicts) != 1 || f.Conflicts[0].Path != "f.txt" {
		t.Fatalf("forecast %+v, %v; want f.txt in conflict", f, err)
	}
	// A whole clone needs nothing.
	if needs, err := OpenRepo(newTestRunner(t), src.Dir).MergeNeeds(ctx, "main", "side"); err != nil || needs != nil {
		t.Fatalf("whole clone: %v, %v", needs, err)
	}
}

func TestMergeNeedsWhereARenameWent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := gittest.New(t)
	src.CommitFile("f.txt", "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\n", "base")
	src.Branch("side")
	src.Git("mv", "f.txt", "g.txt")
	src.WriteFile("g.txt", "one\ntwo\nthree\nfour\nfive\nsix\nseven\nEIGHT\n")
	src.Git("add", "g.txt")
	src.Commit("rename and change")
	src.Checkout("main")
	src.CommitFile("f.txt", "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\n", "main")
	src.Git("config", "uploadpack.allowFilter", "true")
	dest := filepath.Join(t.TempDir(), "partial")
	src.Git("clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(src.Dir), dest)
	repo := OpenRepo(newTestRunner(t), dest)

	needs, err := repo.MergeNeeds(ctx, "HEAD", "origin/side")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(needs, src.Git("rev-parse", "side:g.txt")) {
		t.Fatalf("needs %v; want side's g.txt, where f.txt went", needs)
	}
	if err := repo.Download(ctx, needs, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	f, err := repo.ForecastMerge(NoDownloads(ctx), "HEAD", "origin/side")
	if err != nil || f.Incomplete || len(f.Conflicts) != 0 {
		t.Fatalf("forecast %+v, %v; want a clean merge, found without downloading", f, err)
	}
}

func TestInsightsInAPartialCloneReadNamesUntilAskedForLines(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := gittest.New(t)
	src.CommitFile("f.txt", "one\n", "first")
	src.Branch("feature")
	src.CommitFile("f.txt", "one\nfeature\n", "feature work")
	src.Checkout("main")
	src.CommitFile("f.txt", "one\ntwo\n", "second")
	src.CommitFile("g.txt", "g\n", "third")
	src.Git("config", "uploadpack.allowFilter", "true")
	dest := filepath.Join(t.TempDir(), "partial")
	src.Git("clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(src.Dir), dest)
	repo := OpenRepo(newTestRunner(t), dest)
	off := NoDownloads(ctx)
	q := ShippedQuery{Target: "origin/main", Since: 0, Until: 1 << 40}

	needs, err := repo.WindowNeeds(ctx, q)
	if err != nil || !slices.Contains(needs, src.Git("rev-parse", "main~2:f.txt")) {
		t.Fatalf("needs %v, %v; want the first f.txt, the old side of the second commit", needs, err)
	}
	q.Names = true
	items, err := repo.Shipped(off, q)
	if err != nil || len(items) != 3 {
		t.Fatalf("items %+v, %v", items, err)
	}
	for _, it := range items {
		if it.Files != 1 || it.Added != 0 || it.Removed != 0 {
			t.Errorf("%s: files %d, +%d -%d; want one file and no lines", it.Title, it.Files, it.Added, it.Removed)
		}
	}
	hot, err := repo.HotFiles(off, q)
	if err != nil || len(hot) != 2 || hot[0].Path != "f.txt" || hot[0].Items != 2 {
		t.Fatalf("hot files %+v, %v; want f.txt first, in two items", hot, err)
	}
	open, err := repo.OpenBranches(off, "origin/main", 10, nil)
	if err != nil || len(open) != 1 || !open[0].LinesUnknown || !slices.Equal(open[0].Paths, []string{"f.txt"}) {
		t.Fatalf("open branches %+v, %v; want feature's f.txt, lines unknown", open, err)
	}
	if again, _ := repo.WindowNeeds(ctx, ShippedQuery{Target: "origin/main", Until: 1 << 40}); !slices.Equal(again, needs) {
		t.Fatalf("reading Insights downloaded something: %v missing, was %v", again, needs)
	}

	branch, err := repo.BranchNeeds(ctx, "origin/main", open[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Download(ctx, append(needs, branch...), NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if needs, err := repo.WindowNeeds(ctx, q); err != nil || len(needs) != 0 {
		t.Fatalf("after the download: %v, %v", needs, err)
	}
	q.Names = false
	items, err = repo.Shipped(off, q)
	if err != nil || items[1].Title != "second" || items[1].Added != 1 {
		t.Fatalf("after the download: %+v, %v; want the second commit's line", items, err)
	}
	open, err = repo.OpenBranches(off, "origin/main", 10, nil)
	if err != nil || open[0].LinesUnknown || open[0].Added != 1 {
		t.Fatalf("after the download: %+v, %v", open, err)
	}
	c, err := repo.WouldConflict(off, "origin/main", open[0].SHA)
	if err != nil || !c.Conflicts {
		t.Fatalf("dry-run merge %+v, %v; want a conflict in f.txt, tried without downloading", c, err)
	}
}

// The commits either side of a window change files never downloaded. With the window's own
// in, its lines read without git diffing past either edge, which would fail.
func TestInsightsLinesReadOnlyTheWindowsCommits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := gittest.New(t)
	src.CommitFile("f.txt", "one\n", "first")
	src.CommitFile("f.txt", "two\n", "before the window")
	src.CommitFile("g.txt", "g\n", "in the window")
	src.CommitFile("f.txt", "four\n", "after the window")
	src.Git("config", "uploadpack.allowFilter", "true")
	dest := filepath.Join(t.TempDir(), "partial")
	src.Git("clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(src.Dir), dest)
	repo := OpenRepo(newTestRunner(t), dest)
	at, _ := strconv.ParseInt(src.Git("log", "-1", "--format=%ct", "HEAD~1"), 10, 64)
	q := ShippedQuery{Target: "origin/main", Since: at, Until: at}

	if needs, err := repo.WindowNeeds(ctx, q); err != nil || len(needs) != 0 {
		t.Fatalf("needs %v, %v; the window's one change is checked out", needs, err)
	}
	items, err := repo.Shipped(NoDownloads(ctx), q)
	if err != nil || len(items) != 1 || items[0].Title != "in the window" || items[0].Added != 1 {
		t.Fatalf("shipped %+v, %v; want the window's commit with its line", items, err)
	}
	hot, err := repo.HotFiles(NoDownloads(ctx), q)
	if err != nil || len(hot) != 1 || hot[0].Path != "g.txt" || hot[0].Lines != 1 {
		t.Fatalf("hot files %+v, %v", hot, err)
	}
}

func TestFileHistoryInAPartialCloneStopsAtARenameItCantFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := gittest.New(t)
	src.CommitFile("old.txt", "a\nb\nc\nd\ne\nf\ng\nh\n", "one")
	src.CommitFile("old.txt", "a\nb\nc\nd\ne\nf\ng\nH\n", "two")
	src.Git("mv", "old.txt", "new.txt")
	src.WriteFile("new.txt", "A\nb\nc\nd\ne\nf\ng\nH\n")
	src.Git("add", "new.txt")
	src.Commit("rename and change")
	src.CommitFile("new.txt", "A\nb\nc\nd\ne\nf\ng\nH\nx\n", "four")
	src.Git("config", "uploadpack.allowFilter", "true")
	dest := filepath.Join(t.TempDir(), "partial")
	src.Git("clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(src.Dir), dest)
	repo := OpenRepo(newTestRunner(t), dest)

	hist, stop, err := repo.FileHistory(ctx, "new.txt")
	if err != nil {
		t.Fatal(err)
	}
	renamed := src.Git("rev-parse", "HEAD~1")
	if len(hist) != 1 || hist[0].Subject != "four" || stop == nil || stop.Commit != renamed {
		t.Fatalf("history %+v, stop %+v; want four, then a stop at the rename", hist, stop)
	}
	if !slices.Contains(stop.Missing, src.Git("rev-parse", "HEAD~2:old.txt")) {
		t.Fatalf("missing %v; want old.txt as the rename left it", stop.Missing)
	}
	if err := repo.Download(ctx, stop.Missing, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	hist, stop, err = repo.FileHistory(ctx, "new.txt")
	if err != nil || stop != nil || len(hist) != 4 || hist[1].OldPath != "old.txt" {
		t.Fatalf("after the download: %+v, %+v, %v; want all four, through the rename", hist, stop, err)
	}
}

func TestChangedFilesInAPartialCloneNamesThemAndSaysWhatCountsNeed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := OpenRepo(newTestRunner(t), partialClone(t))
	head, err := repo.ReadString(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	c := CommitDetail{ID: head, Parents: []string{head + "~1"}}
	old, _ := repo.ReadString(ctx, "rev-parse", "HEAD~1:old.txt")

	files, missing, err := repo.ChangedFiles(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "old.txt" || files[0].Added != 0 || !slices.Equal(missing, []string{old}) {
		t.Fatalf("files %+v, missing %v; want old.txt named, no counts, its first version missing", files, missing)
	}
	if !stillMissing(t, repo) {
		t.Fatal("listing the commit's files downloaded old.txt")
	}
	if err := repo.Download(ctx, missing, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	files, missing, err = repo.ChangedFiles(ctx, c)
	if err != nil || len(missing) != 0 || files[0].Added != 1 || files[0].Deleted != 1 {
		t.Fatalf("after the download: %+v, %v, %v", files, missing, err)
	}
}
