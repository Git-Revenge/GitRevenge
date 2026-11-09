package repoconfig

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// fixture is a repository, with the global config pointed at a file of the test's own.
func fixture(t *testing.T) (*gittest.Repo, *git.Repo) {
	t.Helper()
	bin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	fx := gittest.New(t)
	return fx, git.OpenRepo(git.NewRunner(bin), fx.Dir)
}

// global sets a key in the test's global config, through the runner the package reads with.
// (The fixture's own Git helper has an isolated global config of its own.)
func global(t *testing.T, g *git.Repo, args ...string) {
	t.Helper()
	if _, err := g.Write(context.Background(), git.Cmd{Args: append([]string{"config", "--global"}, args...)}); err != nil {
		t.Fatal(err)
	}
}

// globalValues reads every value of a key from the test's global config.
func globalValues(t *testing.T, g *git.Repo, key string) string {
	t.Helper()
	out, err := g.Read(context.Background(), "config", "--global", "--get-all", key)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func load(t *testing.T, g *git.Repo) Settings {
	t.Helper()
	s, err := Load(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func save(t *testing.T, g *git.Repo, was, now Settings) {
	t.Helper()
	if err := Save(context.Background(), g, was, now); err != nil {
		t.Fatal(err)
	}
}

func TestNothingSet(t *testing.T) {
	_, g := fixture(t)
	if s := load(t, g); !s.Empty() {
		t.Fatalf("settings = %+v, want none", s)
	}
	if has, err := HasLocal(context.Background(), g); err != nil || has {
		t.Fatalf("HasLocal = %v, %v; want false", has, err)
	}
}

func TestSaveAndLoad(t *testing.T) {
	_, g := fixture(t)
	want := Settings{
		Kinds:         map[string][]string{"release": {"release/*", "hotfix/*"}, "docs": {}},
		Hide:          []string{"wip/*", "spike/*"},
		HideKinds:     []string{"bot"},
		CleanupOnPull: true,
		SyncNotes:     true,
		StackParents:  map[string]string{"feature/b": "feature/a", "feature/c.d": ""},
		Insights:      &[4]int{14, 30, 50, 800},
	}
	save(t, g, Settings{}, want)
	if got := load(t, g); !got.Equal(want) {
		t.Fatalf("settings = %+v\nwant       %+v", got, want)
	}
	if has, err := HasLocal(context.Background(), g); err != nil || !has {
		t.Fatalf("HasLocal = %v, %v; want true", has, err)
	}

	// Taking things away again leaves nothing behind.
	save(t, g, want, Settings{})
	if got := load(t, g); !got.Empty() {
		t.Fatalf("settings after clearing = %+v, want none", got)
	}
}

func TestRepositoryOverGlobal(t *testing.T) {
	fx, g := fixture(t)
	global(t, g, "gitrevenge.cleanupOnPull", "true")
	global(t, g, "--add", "gitrevenge.hide", "wip/*")
	global(t, g, "--add", "gitrevenge.hide", "tmp/*")
	global(t, g, "--add", "gitrevenge-kind.release.pattern", "rel/*")
	global(t, g, "--add", "gitrevenge-kind.bot.pattern", "renovate/*")
	global(t, g, "gitrevenge-insights.stale", "21")

	// With nothing of its own, a repository takes the global settings.
	got := load(t, g)
	if !got.CleanupOnPull || strings.Join(got.Hide, ",") != "wip/*,tmp/*" || strings.Join(got.Kinds["release"], ",") != "rel/*" {
		t.Fatalf("inherited = %+v", got)
	}
	if got.Insights == nil || got.Insights[0] != 21 {
		t.Fatalf("inherited thresholds = %v", got.Insights)
	}

	// Its own list replaces the global one; it doesn't add to it. Each kind stands alone.
	fx.Git("config", "--local", "--add", "gitrevenge.hide", "spike/*")
	fx.Git("config", "--local", "gitrevenge.cleanupOnPull", "false")
	fx.Git("config", "--local", "--add", "gitrevenge-kind.release.pattern", "release/*")
	got = load(t, g)
	if got.CleanupOnPull {
		t.Error("the repository's false should win over the global true")
	}
	if strings.Join(got.Hide, ",") != "spike/*" {
		t.Errorf("hide = %v, want only the repository's", got.Hide)
	}
	if strings.Join(got.Kinds["release"], ",") != "release/*" || strings.Join(got.Kinds["bot"], ",") != "renovate/*" {
		t.Errorf("kinds = %v", got.Kinds)
	}
}

func TestAnEmptyListTurnsOffTheGlobalOne(t *testing.T) {
	fx, g := fixture(t)
	global(t, g, "--add", "gitrevenge.hide", "wip/*")
	was := load(t, g)
	now := was
	now.Hide = nil
	save(t, g, was, now)
	if got := load(t, g); len(got.Hide) != 0 {
		t.Fatalf("hide = %v, want none here", got.Hide)
	}
	// It's written down as one empty value, so the global list stays for other repositories.
	if out := fx.Git("config", "--local", "--get-all", "gitrevenge.hide"); strings.TrimSpace(out) != "" {
		t.Fatalf("local hide = %q, want one empty value", out)
	}
	if out := globalValues(t, g, "gitrevenge.hide"); out != "wip/*" {
		t.Fatalf("global hide = %q, want it untouched", out)
	}
}

func TestSaveWritesOnlyWhatChanged(t *testing.T) {
	fx, g := fixture(t)
	global(t, g, "gitrevenge.syncNotes", "true")
	global(t, g, "--add", "gitrevenge.hide", "wip/*")
	was := load(t, g)
	now := was
	now.CleanupOnPull = true
	save(t, g, was, now)
	out, err := fx.TryGit("config", "--local", "--get-regexp", "^gitrevenge")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out); got != "gitrevenge.cleanuponpull true" {
		t.Fatalf("the repository's config holds %q, want only the changed key", got)
	}
}

func TestAKindWithNoPatternsMatchesNothing(t *testing.T) {
	_, g := fixture(t)
	global(t, g, "--add", "gitrevenge-kind.docs.pattern", "gh-pages")
	was := load(t, g)
	now := was
	now.Kinds = map[string][]string{"docs": {}}
	save(t, g, was, now)
	got := load(t, g)
	if p, ok := got.Kinds["docs"]; !ok || len(p) != 0 {
		t.Fatalf("docs = %v (set: %v), want set and empty", p, ok)
	}
	// Dropping the repository's say brings the global patterns back.
	save(t, g, got, was)
	if p := load(t, g).Kinds["docs"]; strings.Join(p, ",") != "gh-pages" {
		t.Fatalf("docs = %v, want the global pattern again", p)
	}
}

func TestStackParentsLiveWithTheBranch(t *testing.T) {
	fx, g := fixture(t)
	fx.Linear(1)
	fx.Branch("feature/a")
	fx.Branch("feature/b")
	save(t, g, Settings{}, Settings{StackParents: map[string]string{"feature/b": "feature/a"}})
	if out := fx.Git("config", "--get", "branch.feature/b.gitrevengeStackParent"); strings.TrimSpace(out) != "feature/a" {
		t.Fatalf("branch.feature/b.gitrevengeStackParent = %q", out)
	}
	// Deleting the branch takes its section, and the setting, with it.
	fx.Checkout("feature/a")
	fx.Git("branch", "-D", "feature/b")
	if got := load(t, g); len(got.StackParents) != 0 {
		t.Fatalf("stack parents after deleting the branch = %v, want none", got.StackParents)
	}
}

func TestSmartLinkKeysAreNotOurs(t *testing.T) {
	fx, g := fixture(t)
	fx.Git("config", "--local", "gitrevenge.webLinks", "false")
	fx.Git("config", "--local", "gitrevenge-link.Jira.pattern", "ABC-1")
	if has, err := HasLocal(context.Background(), g); err != nil || has {
		t.Fatalf("HasLocal = %v, %v; want false with only smart-link keys set", has, err)
	}
	if s := load(t, g); !s.Empty() {
		t.Fatalf("settings = %+v, want none", s)
	}
}
