package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestClone(t *testing.T) {
	t.Parallel()
	_, remote := gittest.WithRemote(t, 2)
	r := newTestRunner(t)
	dir := filepath.Join(t.TempDir(), "copy")

	if err := Clone(context.Background(), r, remote.Dir, dir, CloneOptions{}, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := OpenRepo(r, dir).ReadString(context.Background(), "rev-parse", "HEAD")
	if err != nil || got != remote.Git("rev-parse", "main") {
		t.Fatalf("HEAD = %q, %v", got, err)
	}
}

func TestCloneFailureLeavesNoFolder(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	dir := filepath.Join(t.TempDir(), "copy")

	if err := Clone(context.Background(), r, filepath.Join(t.TempDir(), "missing"), dir, CloneOptions{}, NetOptions{}); err == nil {
		t.Fatal("clone of a missing repository succeeded")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("target folder left behind: %v", err)
	}
}

func TestCloneOptionArgs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		o    CloneOptions
		want []string
	}{
		{CloneOptions{}, nil},
		// A shallow clone fetches only the default branch unless told every branch.
		{CloneOptions{Depth: 50}, []string{"--depth", "50", "--no-single-branch"}},
		{CloneOptions{Since: 1700000000}, []string{"--shallow-since=@1700000000", "--no-single-branch"}},
		{CloneOptions{Depth: 5, Since: 1700000000}, []string{"--depth", "5", "--no-single-branch"}},
		{CloneOptions{Partial: true}, []string{"--filter=blob:none"}},
		{CloneOptions{Branch: "dev"}, []string{"--single-branch", "--branch", "dev"}},
		{CloneOptions{Depth: 1, Branch: "dev"}, []string{"--depth", "1", "--single-branch", "--branch", "dev"}},
		{CloneOptions{Submodules: true}, []string{"--recurse-submodules"}},
		{CloneOptions{Folders: true}, []string{"--no-checkout", "--sparse"}},
		{
			CloneOptions{Depth: 1, Partial: true, Submodules: true},
			[]string{"--depth", "1", "--filter=blob:none", "--no-single-branch", "--recurse-submodules", "--shallow-submodules", "--also-filter-submodules"},
		},
	} {
		got, err := c.o.args()
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%+v: args = %q, %v; want %q", c.o, got, err, c.want)
		}
	}
	for _, o := range []CloneOptions{{Depth: -1}, {Since: -5}, {Branch: "--upload-pack=touch x"}} {
		if _, err := o.args(); err == nil {
			t.Errorf("%+v: no error", o)
		}
	}
}

// cloneSource is a repository to clone with options: four commits on main, a branch named
// feature with one more, and filters allowed, as a server that offers partial clones does.
func cloneSource(t *testing.T) *gittest.Repo {
	t.Helper()
	src := gittest.New(t)
	src.Linear(4)
	src.Branch("feature")
	src.CommitFile("feature.txt", "f\n", "feature")
	src.Checkout("main")
	src.Git("config", "uploadpack.allowFilter", "true")
	return src
}

// cloneWith clones src over file://, as from a server, so --depth and --filter apply.
func cloneWith(t *testing.T, src *gittest.Repo, opts CloneOptions, o NetOptions) *Repo {
	t.Helper()
	r := newTestRunner(t)
	dir := filepath.Join(t.TempDir(), "copy")
	if err := Clone(context.Background(), r, "file://"+filepath.ToSlash(src.Dir), dir, opts, o); err != nil {
		t.Fatal(err)
	}
	return OpenRepo(r, dir)
}

// says runs git in repo and returns what it printed, or "" when it failed.
func says(t *testing.T, repo *Repo, args ...string) string {
	t.Helper()
	out, err := repo.ReadString(context.Background(), args...)
	if err != nil {
		return ""
	}
	return out
}

func TestCloneShallowKeepsEveryBranch(t *testing.T) {
	t.Parallel()
	repo := cloneWith(t, cloneSource(t), CloneOptions{Depth: 1}, NetOptions{})
	if got := says(t, repo, "rev-parse", "--is-shallow-repository"); got != "true" {
		t.Errorf("shallow = %q", got)
	}
	if got := says(t, repo, "rev-list", "--count", "HEAD"); got != "1" {
		t.Errorf("commits on main = %q, want 1", got)
	}
	if says(t, repo, "rev-parse", "--verify", "-q", "origin/feature") == "" {
		t.Error("origin/feature wasn't fetched: the dialog said every branch")
	}
}

func TestCloneShallowSince(t *testing.T) {
	t.Parallel()
	src := cloneSource(t)
	// main's commits, newest first: since the second, a clone holds two of them.
	times := strings.Fields(src.Git("log", "--format=%ct", "main"))
	since, err := strconv.ParseInt(times[1], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	repo := cloneWith(t, src, CloneOptions{Since: since}, NetOptions{})
	if got := says(t, repo, "rev-list", "--count", "HEAD"); got != "2" {
		t.Errorf("commits on main = %q, want 2", got)
	}
	if got := says(t, repo, "rev-parse", "--is-shallow-repository"); got != "true" {
		t.Errorf("shallow = %q", got)
	}
}

func TestClonePartialLeavesOldContentsBehind(t *testing.T) {
	t.Parallel()
	repo := cloneWith(t, cloneSource(t), CloneOptions{Partial: true}, NetOptions{})
	if got := says(t, repo, "config", "remote.origin.partialclonefilter"); got != "blob:none" {
		t.Errorf("filter = %q", got)
	}
	if got := says(t, repo, "rev-list", "--count", "HEAD"); got != "4" {
		t.Errorf("commits = %q, want every one", got)
	}
	// The first commit's file.txt was never checked out, so it never came.
	_, err := repo.readCmd(context.Background(), Cmd{Args: []string{"cat-file", "-e", "HEAD~3:file.txt"}, Env: []string{"GIT_NO_LAZY_FETCH=1"}})
	if err == nil {
		t.Error("an old version of file.txt was downloaded")
	}
}

func TestCloneOneBranch(t *testing.T) {
	t.Parallel()
	repo := cloneWith(t, cloneSource(t), CloneOptions{Branch: "feature"}, NetOptions{})
	if got := says(t, repo, "symbolic-ref", "--short", "HEAD"); got != "feature" {
		t.Errorf("checked out %q, want feature", got)
	}
	if says(t, repo, "rev-parse", "--verify", "-q", "origin/main") != "" {
		t.Error("origin/main was fetched too")
	}
}

func TestCloneSubmodules(t *testing.T) {
	t.Parallel()
	sub := gittest.New(t)
	sub.CommitFile("lib.txt", "lib\n", "lib")
	src := gittest.New(t)
	src.CommitFile("app.txt", "app\n", "app")
	src.AddSubmodule(sub, "lib")
	// git refuses file:// submodules unless told; real ones are https or ssh. GIT_CONFIG_*
	// reaches the submodule's own clone, where -c doesn't.
	allowFile := NetOptions{Env: []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=protocol.file.allow", "GIT_CONFIG_VALUE_0=always"}}
	repo := cloneWith(t, src, CloneOptions{Submodules: true}, allowFile)
	if _, err := os.Stat(filepath.Join(repo.Dir(), "lib", "lib.txt")); err != nil {
		t.Errorf("the submodule wasn't cloned: %v", err)
	}
}

// foldersSource is a repository with files at the top, in nested folders, and in folders
// beside them, to clone and choose from.
func foldersSource(t *testing.T) *gittest.Repo {
	t.Helper()
	src := gittest.New(t)
	for _, f := range []string{"top.txt", "src/a.go", "src/sub/b.go", "src/sub/c.go", "docs/d.md", "other/e.txt"} {
		src.WriteFile(f, f+"\n")
	}
	src.Git("add", ".")
	src.Commit("files")
	return src
}

func TestFolderCounts(t *testing.T) {
	t.Parallel()
	repo := OpenRepo(newTestRunner(t), foldersSource(t).Dir)
	got, err := repo.FolderCounts(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	want := []FolderCount{{"", 6, 1}, {"docs", 1, 1}, {"other", 1, 1}, {"src", 3, 1}, {"src/sub", 2, 2}}
	if !slices.Equal(got, want) {
		t.Fatalf("counts = %+v\nwant %+v", got, want)
	}
}

// onDisk lists the files a clone wrote, with forward slashes.
func onDisk(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	return files
}

func TestCloneFoldersWritesOnlyThoseChosen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := foldersSource(t)
	repo := cloneWith(t, src, CloneOptions{Folders: true}, NetOptions{})
	if got := onDisk(t, repo.Dir()); len(got) != 0 {
		t.Fatalf("the first step wrote %v", got)
	}
	// The folder list comes from the tree, with nothing checked out.
	counts, err := repo.FolderCounts(ctx, "")
	if err != nil || len(counts) != 5 {
		t.Fatalf("counts = %+v, %v", counts, err)
	}
	if err := repo.CheckOutFolders(ctx, []string{"src/sub"}); err != nil {
		t.Fatal(err)
	}
	// Cone mode: the folder, the files directly in its parents, and the top level's own.
	want := []string{"src/a.go", "src/sub/b.go", "src/sub/c.go", "top.txt"}
	if got := onDisk(t, repo.Dir()); !slices.Equal(got, want) {
		t.Fatalf("wrote %v, want %v", got, want)
	}
}

func TestCloneFoldersEveryFolderAfterAll(t *testing.T) {
	t.Parallel()
	repo := cloneWith(t, foldersSource(t), CloneOptions{Folders: true}, NetOptions{})
	if err := repo.CheckOutFolders(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if got := onDisk(t, repo.Dir()); len(got) != 6 {
		t.Fatalf("wrote %v, want all 6", got)
	}
}

func TestCheckOutFoldersRefusesPathsOutside(t *testing.T) {
	t.Parallel()
	repo := cloneWith(t, foldersSource(t), CloneOptions{Folders: true}, NetOptions{})
	for _, bad := range []string{"../x", "/etc", "", "src\nother"} {
		if err := repo.CheckOutFolders(context.Background(), []string{bad}); !errors.Is(err, ErrCloneOptions) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestLsRemoteHeads(t *testing.T) {
	t.Parallel()
	src := cloneSource(t)
	heads, err := LsRemoteHeads(context.Background(), newTestRunner(t), src.Dir, NetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if heads.Default != "main" || !slices.Equal(heads.Branches, []string{"feature", "main"}) {
		t.Fatalf("heads = %+v", heads)
	}
}

func TestParseRemoteHeads(t *testing.T) {
	t.Parallel()
	got := parseRemoteHeads([]byte("ref: refs/heads/trunk\tHEAD\n1111\tHEAD\n2222\trefs/heads/release/2.0\n1111\trefs/heads/trunk\n"))
	if got.Default != "trunk" || !slices.Equal(got.Branches, []string{"release/2.0", "trunk"}) {
		t.Errorf("got %+v", got)
	}
	// A server that doesn't say which branch is its default.
	if got := parseRemoteHeads([]byte("1111\trefs/heads/main\n")); got.Default != "" || len(got.Branches) != 1 {
		t.Errorf("no symref: %+v", got)
	}
	if got := parseRemoteHeads(nil); got.Branches == nil {
		t.Error("no branches is an empty list, not null")
	}
}

func TestInit(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	dir := filepath.Join(t.TempDir(), "new", "repo")

	if err := Init(context.Background(), r, dir, "trunk"); err != nil {
		t.Fatal(err)
	}
	got, err := OpenRepo(r, dir).ReadString(context.Background(), "symbolic-ref", "--short", "HEAD")
	if err != nil || got != "trunk" {
		t.Fatalf("branch = %q, %v", got, err)
	}
}

func TestCloneName(t *testing.T) {
	t.Parallel()
	for url, want := range map[string]string{
		"https://github.com/owner/repo.git": "repo",
		"https://github.com/owner/repo":     "repo",
		"https://github.com/owner/repo/":    "repo",
		"git@github.com:owner/repo.git":     "repo",
		"git@host:repo.git":                 "repo",
		"ssh://git@host:22/owner/repo.git/": "repo",
		"/srv/git/project/.git":             "project",
		`C:\src\tool`:                       "tool",
		"  https://example.com/a/b.git  ":   "b",
		"":                                  "",
		"https://example.com/..":            "",
	} {
		if got := CloneName(url); got != want {
			t.Errorf("CloneName(%q) = %q, want %q", url, got, want)
		}
	}
}
