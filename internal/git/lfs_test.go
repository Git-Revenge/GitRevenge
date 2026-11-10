package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestLFSStatusTrackUntrack(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.SetupLFS("*.bin") // skips when git-lfs is missing
	fx.WriteFile("big.bin", "\x00binary")
	fx.Commit("add bin")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	st, err := repo.LFS(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Patterns) != 1 || st.Patterns[0].Pattern != "*.bin" {
		t.Fatalf("patterns = %+v", st.Patterns)
	}
	if len(st.Files) != 1 || st.Files[0].Name != "big.bin" || !st.Files[0].Downloaded {
		t.Fatalf("files = %+v", st.Files)
	}

	if err := repo.LFSTrack(ctx, "*.psd", true); err != nil {
		t.Fatal(err)
	}
	st, _ = repo.LFS(ctx)
	var psd *LFSPattern
	for i := range st.Patterns {
		if st.Patterns[i].Pattern == "*.psd" {
			psd = &st.Patterns[i]
		}
	}
	if psd == nil || !psd.Lockable {
		t.Fatalf("patterns after track = %+v", st.Patterns)
	}
	if err := repo.LFSUntrack(ctx, "*.psd"); err != nil {
		t.Fatal(err)
	}
	if st, _ := repo.LFS(ctx); len(st.Patterns) != 1 {
		t.Fatalf("patterns after untrack = %+v", st.Patterns)
	}
}

func TestLFSEmptyRepo(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.SetupLFS("*.bin")
	st, err := OpenRepo(newTestRunner(t), fx.Dir).LFS(context.Background())
	if err != nil || len(st.Files) != 0 || len(st.Patterns) != 1 {
		t.Fatalf("st = %+v, %v", st, err)
	}
}

// Locks need an LFS server, so the server's JSON is tested directly.
func TestParseLocks(t *testing.T) {
	t.Parallel()
	out := []byte(`{"ours":[{"id":"1","path":"art/a.psd","owner":{"name":"me"},"locked_at":"2026-10-28T10:00:00Z"}],
		"theirs":[{"id":"2","path":"art/b.psd","owner":{"name":"sam"},"locked_at":"2026-10-28T11:00:00Z"}]}`)
	locks, err := parseLocks(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(locks) != 2 || !locks[0].Ours || locks[0].Path != "art/a.psd" || locks[1].Ours || locks[1].Owner != "sam" {
		t.Fatalf("locks = %+v", locks)
	}
	if locks, err := parseLocks([]byte(`{"ours":[],"theirs":[]}`)); err != nil || len(locks) != 0 {
		t.Fatalf("empty = %+v, %v", locks, err)
	}
}

func TestParseSubmoduleLine(t *testing.T) {
	t.Parallel()
	sha := "0123456789abcdef0123456789abcdef01234567"
	cases := map[string]Submodule{
		" " + sha + " libs/a (heads/main)":           {Path: "libs/a", Commit: sha, State: "ok", Describe: "heads/main"},
		"-" + sha + " libs/b":                        {Path: "libs/b", Commit: sha, State: "uninitialized"},
		"+" + sha + " path with space (v1.0-2-gabc)": {Path: "path with space", Commit: sha, State: "modified", Describe: "v1.0-2-gabc"},
		"U" + sha + " c":                             {Path: "c", Commit: sha, State: "conflict"},
	}
	for line, want := range cases {
		got, ok := parseSubmoduleLine(line)
		if !ok || got != want {
			t.Errorf("%q = %+v, want %+v", line, got, want)
		}
	}
	if _, ok := parseSubmoduleLine("short"); ok {
		t.Error("garbage accepted")
	}
}

func TestSubmodulesStatusAndUpdate(t *testing.T) {
	t.Parallel()
	sub := gittest.New(t)
	subHead := sub.Linear(1)[0]
	fx := gittest.New(t)
	fx.Linear(1)
	fx.AddSubmodule(sub, "libs/sub")
	ctx := context.Background()

	subs, err := OpenRepo(newTestRunner(t), fx.Dir).Submodules(ctx)
	if err != nil || len(subs) != 1 || subs[0].Path != "libs/sub" || subs[0].State != "ok" || subs[0].Commit != subHead {
		t.Fatalf("subs = %+v, %v", subs, err)
	}

	// A fresh clone has the submodule uninitialized until update.
	clone := gittest.Clone(t, fx)
	repo := OpenRepo(newTestRunner(t), clone.Dir)
	subs, _ = repo.Submodules(ctx)
	if len(subs) != 1 || subs[0].State != "uninitialized" {
		t.Fatalf("clone subs = %+v", subs)
	}
	// The test submodule lives on disk; git blocks file:// submodules by default (real ones
	// use https or ssh). GIT_CONFIG_* reaches the child clone, unlike -c.
	allowFile := NetOptions{Env: []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=protocol.file.allow", "GIT_CONFIG_VALUE_0=always"}}
	if err := repo.UpdateSubmodules(ctx, allowFile); err != nil {
		t.Fatal(err)
	}
	if subs, _ := repo.Submodules(ctx); subs[0].State != "ok" {
		t.Fatalf("after update = %+v", subs)
	}
}

// Listing must not install LFS's hooks, which then run git-lfs on every commit (#604).
func TestLFSStatusLeavesHooksAlone(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	if _, err := fx.TryGit("lfs", "version"); err != nil {
		t.Skip("git-lfs not installed")
	}
	fx.Linear(1)
	st, err := OpenRepo(newTestRunner(t), fx.Dir).LFS(context.Background())
	if err != nil || len(st.Patterns) != 0 || len(st.Files) != 0 {
		t.Fatalf("st = %+v, %v", st, err)
	}
	for _, hook := range []string{"post-checkout", "post-commit", "post-merge", "pre-push"} {
		if _, err := os.Stat(fx.Path(".git/hooks/" + hook)); err == nil {
			t.Errorf("%s hook was installed", hook)
		}
	}
}

// The patterns read from attributes files match what `git lfs track --json` lists, apart
// from two of its quirks: it prefixes .git/info/attributes patterns with ".git/info/" and
// gives global ones no source.
func TestLFSPatternsMatchGitLFS(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	if _, err := fx.TryGit("lfs", "version"); err != nil {
		t.Skip("git-lfs not installed")
	}
	fx.WriteFile(".gitattributes", "*.bin filter=lfs diff=lfs merge=lfs -text\n"+
		"*.psd filter=lfs diff=lfs merge=lfs -text lockable\n"+
		"my[[:space:]]file.dat filter=lfs -text\n"+
		"\"q x.zip\" filter=lfs\n"+
		"*.txt -filter\n# a comment\n[attr]big filter=lfs\n*.mov big\n*.md text\n")
	fx.WriteFile("sub/.gitattributes", "*.wav filter=lfs\r\n")
	fx.Commit("attributes")
	fx.WriteFile("new/.gitattributes", "*.ogg filter=lfs\n") // not yet added
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	got, err := repo.lfsPatterns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Patterns []LFSPattern `json:"patterns"`
	}
	if err := repo.lfsJSON(ctx, &want, "track", "--json"); err != nil {
		t.Fatal(err)
	}
	var tracked []LFSPattern
	for _, p := range want.Patterns {
		if p.Tracked {
			p.Source = filepath.ToSlash(p.Source)
			tracked = append(tracked, p)
		}
	}
	key := func(p LFSPattern) string { return p.Pattern }
	sortBy := func(ps []LFSPattern) {
		slices.SortFunc(ps, func(a, b LFSPattern) int { return strings.Compare(key(a), key(b)) })
	}
	sortBy(got)
	sortBy(tracked)
	if !slices.Equal(got, tracked) {
		t.Fatalf("got  %+v\nwant %+v", got, tracked)
	}
}

func TestParseLFSAttributes(t *testing.T) {
	t.Parallel()
	got := parseLFSAttributes("[attr]big filter=lfs\n*.mov big\n\"a\\\"b c\" filter=lfs lockable\n*.x -filter\nlone\n", "d/.gitattributes", "d")
	want := []LFSPattern{
		{Pattern: "d/*.mov", Source: "d/.gitattributes", Tracked: true},
		{Pattern: `d/a"b c`, Source: "d/.gitattributes", Tracked: true, Lockable: true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
}
