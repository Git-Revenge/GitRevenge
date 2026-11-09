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

func TestEscapeIgnorePath(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"debug.log":         "/debug.log",
		"a/b c.txt":         "/a/b c.txt",
		"#notes":            "/#notes",
		"!bang":             "/!bang",
		"what?[1]*.txt":     `/what\?\[1]\*.txt`,
		`back\slash`:        `/back\\slash`,
		"trailing  ":        `/trailing\ \ `,
		"build/cache/x.bin": "/build/cache/x.bin",
	} {
		if got := EscapeIgnorePath(in); got != want {
			t.Errorf("EscapeIgnorePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRootPattern(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ dir, in, want string }{
		{"", "*.log", "*.log"},
		{"build", "*.bin", "/build/**/*.bin"},
		{"build", "cache/", "/build/**/cache/"},
		{"build", "/cache/", "/build/cache/"},
		{"build", "cache/x.bin", "/build/cache/x.bin"},
		{"build/", "!keep", "!/build/**/keep"},
	} {
		if got := rootPattern(c.dir, c.in); got != c.want {
			t.Errorf("rootPattern(%q, %q) = %q, want %q", c.dir, c.in, got, c.want)
		}
	}
}

// ignoreFixture is a repository with a committed file, untracked files and a build folder.
func ignoreFixture(t *testing.T) (*gittest.Repo, *Repo) {
	t.Helper()
	fx := gittest.New(t)
	fx.WriteFile("README.md", "hi\n")
	fx.WriteFile("build/cache/README.md", "tracked\n")
	fx.Commit("base")
	fx.WriteFile("debug.log", "x\n")
	fx.WriteFile("server.log", "x\n")
	fx.WriteFile("notes.txt", "x\n")
	fx.WriteFile("build/cache/index.bin", "x\n")
	fx.WriteFile("build/cache/chunks.bin", "x\n")
	// Keeps the user's own excludes file out of the tests.
	fx.Git("config", "core.excludesFile", filepath.Join(t.TempDir(), "global-ignore"))
	return fx, OpenRepo(newTestRunner(t), fx.Dir)
}

func untrackedPaths(t *testing.T, repo *Repo) []string {
	t.Helper()
	var out []string
	for p, f := range status(t, repo) {
		if f.Untracked {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func TestAddIgnoreRuleHidesFiles(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	got, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreRoot}, "*.log")
	if err != nil {
		t.Fatal(err)
	}
	if got != (IgnoreLine{Source: ".gitignore", Line: 1, Pattern: "*.log", Created: true}) {
		t.Fatalf("added %+v", got)
	}
	// Adding the same rule again doesn't repeat it.
	again, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreRoot}, "*.log")
	if err != nil || again.Line != 1 {
		t.Fatalf("re-add = %+v, %v", again, err)
	}
	if u := untrackedPaths(t, repo); slices.Contains(u, "debug.log") || slices.Contains(u, "server.log") {
		t.Fatalf("logs still untracked: %v", u)
	}
	data, _ := os.ReadFile(fx.Path(".gitignore"))
	if string(data) != "*.log\n" {
		t.Fatalf(".gitignore = %q", data)
	}

	if err := repo.RemoveIgnoreRule(ctx, got); err != nil {
		t.Fatal(err)
	}
	if u := untrackedPaths(t, repo); !slices.Contains(u, "debug.log") {
		t.Fatalf("debug.log not back after removing the rule: %v", u)
	}
	if _, err := os.Stat(fx.Path(".gitignore")); !os.IsNotExist(err) {
		t.Fatalf(".gitignore it created is still there: %v", err)
	}
}

func TestAddIgnoreRuleKeepsCRLFAndMissingNewline(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	fx.WriteFile(".gitignore", "a\r\nb")
	if _, err := repo.AddIgnoreRule(context.Background(), IgnoreTarget{Scope: IgnoreRoot}, "*.log"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(fx.Path(".gitignore"))
	if string(data) != "a\r\nb\r\n*.log\r\n" {
		t.Fatalf(".gitignore = %q", data)
	}
}

func TestAddIgnoreRuleTargets(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	dir, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreDir, Dir: "build"}, "*.bin")
	if err != nil || dir.Source != "build/.gitignore" {
		t.Fatalf("dir rule = %+v, %v", dir, err)
	}
	ex, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreExclude}, "/notes.txt")
	if err != nil || filepath.ToSlash(ex.Source) != ".git/info/exclude" {
		t.Fatalf("exclude rule = %+v, %v", ex, err)
	}
	global, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreGlobal}, "server.log")
	if err != nil || !filepath.IsAbs(global.Source) {
		t.Fatalf("global rule = %+v, %v", global, err)
	}
	want := []string{"build/.gitignore", "debug.log"}
	if u := untrackedPaths(t, repo); !slices.Equal(u, want) {
		t.Fatalf("untracked = %v, want %v", u, want)
	}
	if _, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreDir, Dir: "../outside"}, "x"); err == nil {
		t.Fatal("a folder outside the work tree was accepted")
	}
	if _, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreRoot}, "a\nb"); err == nil {
		t.Fatal("a two-line rule was accepted")
	}
	_ = fx
}

func TestRemoveIgnoreRuleChecksTheLine(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	fx.WriteFile(".gitignore", "# top\n*.log\n*.tmp\n")
	// The line moved: the pattern is found elsewhere.
	if err := repo.RemoveIgnoreRule(ctx, IgnoreLine{Source: ".gitignore", Line: 1, Pattern: "*.log"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(fx.Path(".gitignore"))
	if string(data) != "# top\n*.tmp\n" {
		t.Fatalf(".gitignore = %q", data)
	}
	if err := repo.RemoveIgnoreRule(ctx, IgnoreLine{Source: ".gitignore", Line: 2, Pattern: "*.log"}); err == nil {
		t.Fatal("removed a rule that isn't there")
	}
	for _, src := range []string{"README.md", "../.gitignore", filepath.Join(t.TempDir(), "x")} {
		if err := repo.RemoveIgnoreRule(ctx, IgnoreLine{Source: src, Line: 1, Pattern: "hi"}); err == nil {
			t.Errorf("edited %s, which isn't an ignore file", src)
		}
	}
}

func TestPreviewIgnore(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	pv, err := repo.PreviewIgnore(ctx, IgnoreTarget{Scope: IgnoreRoot}, "/build/cache/")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(pv.Hidden)
	if want := []string{"build/cache/chunks.bin", "build/cache/index.bin"}; !slices.Equal(pv.Hidden, want) {
		t.Errorf("hidden = %v, want %v", pv.Hidden, want)
	}
	if want := []string{"build/cache/README.md"}; !slices.Equal(pv.Tracked, want) {
		t.Errorf("tracked = %v, want %v", pv.Tracked, want)
	}
	// A folder's own .gitignore: patterns are relative to it.
	pv, err = repo.PreviewIgnore(ctx, IgnoreTarget{Scope: IgnoreDir, Dir: "build"}, "index.bin")
	if err != nil || !slices.Equal(pv.Hidden, []string{"build/cache/index.bin"}) {
		t.Errorf("dir preview = %+v, %v", pv, err)
	}
	// An existing "!" rule keeps its file visible.
	fx.WriteFile(".gitignore", "!keep.log\n")
	fx.WriteFile("keep.log", "x\n")
	pv, err = repo.PreviewIgnore(ctx, IgnoreTarget{Scope: IgnoreRoot}, "*.log")
	slices.Sort(pv.Hidden)
	if want := []string{"debug.log", "server.log"}; err != nil || !slices.Equal(pv.Hidden, want) {
		t.Errorf("hidden with an exception = %v, %v, want %v", pv.Hidden, err, want)
	}
	// Nothing written.
	if data, _ := os.ReadFile(fx.Path(".gitignore")); string(data) != "!keep.log\n" {
		t.Errorf("preview wrote .gitignore: %q", data)
	}
}

func TestIgnoredFilesAndExceptions(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	fx.WriteFile(".gitignore", "*.log\n/build/cache/\n")
	fx.WriteFile("node_modules/a/b.js", "x\n")
	fx.WriteFile("build/.gitignore", "")
	fx.Git("config", "core.excludesFile", filepath.Join(t.TempDir(), "g"))
	if err := os.WriteFile(filepath.Join(fx.Dir, ".git", "info", "exclude"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, truncated, err := repo.IgnoredFiles(ctx)
	if err != nil || truncated {
		t.Fatal(err, truncated)
	}
	by := map[string]IgnoredFile{}
	for _, f := range files {
		by[f.Path] = f
	}
	if f := by["debug.log"]; f.Rule.Source != ".gitignore" || f.Rule.Line != 1 || f.Rule.Pattern != "*.log" || !f.CanExcept {
		t.Errorf("debug.log = %+v", f)
	}
	if f, ok := by["node_modules/"]; !ok || filepath.ToSlash(f.Rule.Source) != ".git/info/exclude" || !f.CanExcept {
		t.Errorf("node_modules/ = %+v (listed: %v)", f, ok)
	}
	// build/cache holds a tracked file, so its untracked files are listed one by one, and
	// no exception can bring them back from under the ignored folder.
	if f, ok := by["build/cache/index.bin"]; !ok || f.CanExcept {
		t.Errorf("build/cache/index.bin = %+v (listed: %v)", f, ok)
	}

	// An exception goes after the rule in the same file.
	ex, err := repo.ExceptIgnored(ctx, "debug.log", by["debug.log"].Rule)
	if err != nil || ex != (IgnoreLine{Source: ".gitignore", Line: 3, Pattern: "!/debug.log"}) {
		t.Fatalf("exception = %+v, %v", ex, err)
	}
	u := untrackedPaths(t, repo)
	if !slices.Contains(u, "debug.log") || slices.Contains(u, "server.log") {
		t.Fatalf("untracked after the exception = %v", u)
	}
	// A folder exception from info/exclude stays in info/exclude.
	if _, err := repo.ExceptIgnored(ctx, "node_modules/", by["node_modules/"].Rule); err != nil {
		t.Fatal(err)
	}
	if u := untrackedPaths(t, repo); !slices.Contains(u, "node_modules/a/b.js") {
		t.Fatalf("node_modules not back: %v", u)
	}
	data, _ := os.ReadFile(filepath.Join(fx.Dir, ".git", "info", "exclude"))
	if !strings.HasSuffix(string(data), "!/node_modules/\n") {
		t.Fatalf("info/exclude = %q", data)
	}
}

func TestExceptGlobalRuleStaysInThisClone(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	rule, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreGlobal}, "*.log")
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := repo.IgnoredFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found IgnoredFile
	for _, f := range files {
		if f.Path == "debug.log" {
			found = f
		}
	}
	if !sameIgnoreFile(found.Rule.Source, rule.Source) {
		t.Fatalf("debug.log rule %+v, want source %s", found.Rule, rule.Source)
	}
	ex, err := repo.ExceptIgnored(ctx, "debug.log", found.Rule)
	if err != nil || filepath.ToSlash(ex.Source) != ".git/info/exclude" {
		t.Fatalf("exception = %+v, %v", ex, err)
	}
	global, _ := os.ReadFile(rule.Source)
	if string(global) != "*.log\n" {
		t.Fatalf("global excludes changed: %q", global)
	}
	if u := untrackedPaths(t, repo); !slices.Contains(u, "debug.log") {
		t.Fatalf("debug.log not back: %v", u)
	}
	_ = fx
}

func TestStopTracking(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	if err := repo.StopTracking(context.Background(), []string{"build/cache/README.md"}); err != nil {
		t.Fatal(err)
	}
	st, err := repo.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(st.Files, func(f FileStatus) bool { return f.Path == "build/cache/README.md" && f.Index == "D" }) {
		t.Fatalf("no staged deletion: %+v", st.Files)
	}
	if _, err := os.Stat(fx.Path("build/cache/README.md")); err != nil {
		t.Fatal("file deleted from disk:", err)
	}
}

func TestRestoreIgnoreRule(t *testing.T) {
	t.Parallel()
	fx, repo := ignoreFixture(t)
	ctx := context.Background()
	fx.WriteFile(".gitignore", "a\n*.log\nb\n")
	rule := IgnoreLine{Source: ".gitignore", Line: 2, Pattern: "*.log"}
	if err := repo.RemoveIgnoreRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	if err := repo.RestoreIgnoreRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(fx.Path(".gitignore")); string(data) != "a\n*.log\nb\n" {
		t.Fatalf(".gitignore = %q", data)
	}
}
