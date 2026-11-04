package git

import (
	"context"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestLFSStatusTrackUntrack(t *testing.T) {
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
	fx := gittest.New(t)
	fx.SetupLFS("*.bin")
	st, err := OpenRepo(newTestRunner(t), fx.Dir).LFS(context.Background())
	if err != nil || len(st.Files) != 0 || len(st.Patterns) != 1 {
		t.Fatalf("st = %+v, %v", st, err)
	}
}

// Locks need an LFS server, so the server's JSON is tested directly.
func TestParseLocks(t *testing.T) {
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
