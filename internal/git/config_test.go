package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// isolatedConfigRunner returns a runner whose global config is a temp file with content,
// and which ignores system config.
func isolatedConfigRunner(t *testing.T, content string) (*Runner, string) {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	return newTestRunner(t), global
}

func TestParseConfigList(t *testing.T) {
	t.Parallel()
	out := "global\x00file:C:/Users/me/.gitconfig\x00user.name\nMe\x00local\x00file:.git/config\x00core.bare\x00command\x00\x00a.b\nx\ny\x00"
	got := parseConfigList([]byte(out), true)
	want := []ConfigEntry{
		{Scope: "global", File: filepath.Clean(filepath.FromSlash("C:/Users/me/.gitconfig")), Key: "user.name", Value: "Me"},
		{Scope: "local", File: filepath.Clean(filepath.FromSlash(".git/config")), Key: "core.bare", Implicit: true},
		{Scope: "command", Key: "a.b", Value: "x\ny"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if got := parseConfigList([]byte("a.b\n1\x00c.d\x00"), false); !slices.Equal(got, []ConfigEntry{{Key: "a.b", Value: "1"}, {Key: "c.d", Implicit: true}}) {
		t.Errorf("file list = %+v", got)
	}
}

func TestSameConfigKey(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"user.name", "User.Name", true},
		{"remote.origin.url", "REMOTE.origin.URL", true},
		{"remote.Origin.url", "remote.origin.url", false},
		{"includeIf.gitdir:~/Work/.path", "includeif.gitdir:~/Work/.PATH", true},
		{"alias.co", "alias.ci", false},
	} {
		if got := SameConfigKey(c.a, c.b); got != c.want {
			t.Errorf("SameConfigKey(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestEffectiveConfig(t *testing.T) {
	inc := filepath.Join(t.TempDir(), "work.gitconfig")
	if err := os.WriteFile(inc, []byte("[user]\n\temail = work@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, global := isolatedConfigRunner(t, "[user]\n\tname = Global\n[include]\n\tpath = "+filepath.ToSlash(inc)+"\n")
	fx := gittest.New(t)
	fx.Git("config", "user.name", "Local")

	entries, err := r.EffectiveConfig(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	find := func(key, value string) ConfigEntry {
		i := slices.IndexFunc(entries, func(e ConfigEntry) bool { return e.Key == key && e.Value == value })
		if i < 0 {
			t.Fatalf("%s = %s missing from %+v", key, value, entries)
		}
		return entries[i]
	}
	if e := find("user.name", "Global"); e.Scope != "global" || !sameFile(e.File, global) {
		t.Errorf("global entry = %+v", e)
	}
	if e := find("user.email", "work@example.com"); e.Scope != "global" || !sameFile(e.File, inc) {
		t.Errorf("included entry = %+v", e)
	}
	if e := find("user.name", "Local"); e.Scope != "local" || !sameFile(e.File, filepath.Join(fx.Dir, ".git", "config")) {
		t.Errorf("local entry = %+v", e)
	}

	// The runner's own -c settings aren't the user's config.
	if slices.ContainsFunc(entries, func(e ConfigEntry) bool { return e.Scope == "command" }) {
		t.Errorf("command-line config listed: %+v", entries)
	}

	// Outside a repository there is no local config.
	entries, err = r.EffectiveConfig(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(entries, func(e ConfigEntry) bool { return e.Scope == "local" }) {
		t.Errorf("local config read outside a repository: %+v", entries)
	}
}

func sameFile(a, b string) bool {
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

func TestFileConfigAndEdit(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "config")

	if got, err := r.FileConfig(ctx, file); err != nil || got != nil {
		t.Fatalf("missing file = %v, %v", got, err)
	}
	for _, args := range [][]string{
		{"--", "alias.co", "checkout"},
		{"--", "core.pager", "-R"},
		{"--add", "--", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"},
		{"--add", "--", "remote.origin.fetch", "+refs/tags/*:refs/tags/*"},
	} {
		if err := r.EditConfig(ctx, file, args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	got, err := r.FileConfig(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[1].Value != "-R" || got[0].File != file {
		t.Errorf("entries = %+v", got)
	}

	if err := r.EditConfig(ctx, file, "--unset-all", "--", "no.such"); !errors.Is(err, ErrNoSuchKey) {
		t.Errorf("unset missing = %v, want ErrNoSuchKey", err)
	}
	var ce *ConfigError
	if err := r.EditConfig(ctx, file, "--", "nodot", "x"); !errors.As(err, &ce) {
		t.Errorf("invalid key = %v, want *ConfigError", err)
	}

	bad := filepath.Join(t.TempDir(), "bad")
	if err := os.WriteFile(bad, []byte("[ok]\n\ta = 1\n[broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.FileConfig(ctx, bad); !errors.As(err, &ce) || ce.Line != 3 {
		t.Errorf("bad file = %v (%+v), want line 3", err, ce)
	}
}

func TestGlobalConfigPath(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", "")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	homeFile := filepath.Join(home, ".gitconfig")
	xdgFile := filepath.Join(xdg, "git", "config")
	check := func(wantPath, wantAlt string) {
		t.Helper()
		got, alt, err := newTestRunner(t).GlobalConfigPath(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got != wantPath || alt != wantAlt {
			t.Errorf("GlobalConfigPath = %q, %q; want %q, %q", got, alt, wantPath, wantAlt)
		}
	}
	// Neither exists: git creates ~/.gitconfig.
	check(homeFile, "")

	if err := os.MkdirAll(filepath.Dir(xdgFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	check(xdgFile, "")

	if err := os.WriteFile(homeFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	check(homeFile, xdgFile)

	override := filepath.Join(t.TempDir(), "custom")
	t.Setenv("GIT_CONFIG_GLOBAL", override)
	check(override, "")
}

func TestLocalConfigPathWorktree(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	wt := filepath.Join(t.TempDir(), "wt")
	fx.Git("worktree", "add", "-b", "side", wt)
	got, err := OpenRepo(newTestRunner(t), wt).LocalConfigPath(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !sameFile(got, filepath.Join(fx.Dir, ".git", "config")) {
		t.Errorf("worktree config = %s, want the main repository's", got)
	}
}

func TestConfigKeysAndCommands(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	keys, err := r.ConfigKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(keys, "pull.rebase") || slices.ContainsFunc(keys, func(k string) bool { return k == "" }) {
		t.Errorf("config keys missing pull.rebase or has blanks: %d keys", len(keys))
	}
	cmds, err := r.Commands(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(cmds, "checkout") {
		t.Errorf("commands = %v, want checkout", cmds)
	}
}
