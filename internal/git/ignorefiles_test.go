package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreSources(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	fx.WriteFile("build/.gitignore", "*.bin\n")
	fx.WriteFile("a/b/.gitignore", "# nothing yet\n\n")
	fx.WriteFile("notes.gitignore", "not an ignore file\n")
	list, err := repo.IgnoreSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range list {
		got = append(got, filepath.ToSlash(s.Source))
	}
	want := []string{".gitignore", "a/b/.gitignore", "build/.gitignore", ".git/info/exclude"}
	if len(got) != 5 || !equalStrings(got[:4], want) || !filepath.IsAbs(got[4]) {
		t.Fatalf("sources = %v, want %v and the global file", got, want)
	}
	if list[0].Exists || list[0].Scope != IgnoreRoot {
		t.Errorf("missing top .gitignore = %+v", list[0])
	}
	if list[1].Rules != 0 || list[1].Dir != "a/b" || list[1].Scope != IgnoreDir {
		t.Errorf("a/b/.gitignore = %+v", list[1])
	}
	if list[2].Rules != 1 || !list[2].Exists {
		t.Errorf("build/.gitignore = %+v", list[2])
	}
	if list[3].Scope != IgnoreExclude || list[4].Scope != IgnoreGlobal {
		t.Errorf("scopes = %s, %s", list[3].Scope, list[4].Scope)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestReadWriteIgnoreFile(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	fx.WriteFile(".gitignore", "a\r\nb\r\n")
	text, err := repo.ReadIgnoreFile(ctx, ".gitignore")
	if err != nil || text != "a\nb\n" {
		t.Fatalf("read = %q, %v", text, err)
	}
	if err := repo.WriteIgnoreFile(ctx, ".gitignore", text, "a\nc\n*.log"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(fx.Path(".gitignore"))
	if string(data) != "a\r\nc\r\n*.log\r\n" {
		t.Fatalf("written = %q", data)
	}
	// Someone else changed it since: refused.
	if err := repo.WriteIgnoreFile(ctx, ".gitignore", text, "x\n"); !errors.Is(err, ErrIgnoreFileChanged) {
		t.Fatalf("stale write err = %v", err)
	}
	// A new folder .gitignore is created.
	if err := repo.WriteIgnoreFile(ctx, "docs/.gitignore", "", "_site/\n"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(fx.Path("docs/.gitignore")); string(data) != "_site/\n" {
		t.Fatalf("docs/.gitignore = %q", data)
	}
	// Only ignore files.
	if err := repo.WriteIgnoreFile(ctx, "README.md", "hi\n", "x\n"); err == nil {
		t.Fatal("wrote README.md")
	}
}

func TestIgnoreStats(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	fx.WriteFile(".gitignore", "*.log\n!keep.log\n/build/cache/\n*.tmp\n")
	fx.WriteFile("keep.log", "x\n")
	st, err := repo.IgnoreStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	by := map[int]IgnoreRuleStats{}
	for _, s := range st.Rules {
		if s.Source == ".gitignore" {
			by[s.Line] = s
		}
	}
	if s := by[1]; s.Ignores != 2 {
		t.Errorf("*.log = %+v", s)
	}
	if s := by[2]; s.Restores != 1 || len(s.Paths) != 1 || s.Paths[0] != "keep.log" {
		t.Errorf("!keep.log = %+v", s)
	}
	if s := by[3]; s.Tracked != 1 || s.Ignores != 2 {
		t.Errorf("/build/cache/ = %+v", s)
	}
	if _, ok := by[4]; ok {
		t.Errorf("*.tmp decides something: %+v", by[4])
	}
}

func TestCheckIgnorePath(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	fx.WriteFile(".gitignore", "*.log\n!keep.log\n/build/cache/\n")
	for _, c := range []struct {
		path             string
		matched, ignored bool
		line             int
		tracked          bool
	}{
		{"debug.log", true, true, 1, false},
		{"keep.log", true, false, 2, false},
		{"not/there/yet.log", true, true, 1, false},
		{"build/cache/README.md", true, true, 3, true},
		{"README.md", false, false, 0, true},
	} {
		got, err := repo.CheckIgnorePath(ctx, c.path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Matched != c.matched || got.Ignored != c.ignored || got.Rule.Line != c.line || got.Tracked != c.tracked {
			t.Errorf("%s = %+v", c.path, got)
		}
	}
	if _, err := repo.CheckIgnorePath(ctx, "../outside"); err == nil {
		t.Error("checked a path outside the work tree")
	}
}
