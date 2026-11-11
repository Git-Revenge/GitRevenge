package app

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// waitIdle waits for the background work the app has started so far, such as the settings
// read that opening a repository sets off. It doesn't wait on the app's WaitGroup: the file
// watcher can start more work at any moment, and adding to a group while it's waited on is a
// data race (issue #639). Only Shutdown, which stops new work first, may do that.
func waitIdle(t *testing.T, a *App) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); !a.idle(); time.Sleep(2 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("background work didn't finish")
		}
	}
}

// openForConfig opens a fresh repository in a test app with a global config of its own, and
// waits for the settings read that opening starts.
func openForConfig(t *testing.T) (*App, *gittest.Repo, string) {
	t.Helper()
	isolateGit(t)
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	waitIdle(t, a)
	return a, fx, tab.Path
}

// localConfig returns every value of a key in the repository's own config, one per line.
func localConfig(t *testing.T, fx *gittest.Repo, key string) string {
	t.Helper()
	out, err := fx.TryGit("config", "--local", "--get-all", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// setGlobal sets a key in the test's global config.
func setGlobal(t *testing.T, a *App, path string, args ...string) {
	t.Helper()
	g, _, err := a.repoGit(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Write(context.Background(), git.Cmd{Args: append([]string{"config", "--global"}, args...)}); err != nil {
		t.Fatal(err)
	}
}

func TestRepoSettingsAreWrittenToGitConfig(t *testing.T) {
	a, fx, path := openForConfig(t)
	p, err := a.RepoPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	p.Hide = []string{"wip/*", "spike/*"}
	p.CleanupOnPull = true
	if err := a.SetRepoPreferences(path, p); err != nil {
		t.Fatal(err)
	}
	if got := localConfig(t, fx, "gitrevenge.hide"); got != "wip/*\nspike/*" {
		t.Errorf("gitrevenge.hide = %q", got)
	}
	if got := localConfig(t, fx, "gitrevenge.cleanupOnPull"); got != "true" {
		t.Errorf("gitrevenge.cleanupOnPull = %q", got)
	}
	// Nothing else was touched, so nothing else was written.
	if got := localConfig(t, fx, "gitrevenge.syncNotes"); got != "" {
		t.Errorf("gitrevenge.syncNotes = %q, want unset", got)
	}

	if err := a.SetKindView(path, "bot", "hide"); err != nil {
		t.Fatal(err)
	}
	if got := localConfig(t, fx, "gitrevenge.hideKind"); got != "bot" {
		t.Errorf("gitrevenge.hideKind = %q", got)
	}

	fx.Branch("feature/a")
	fx.Branch("feature/b")
	if err := a.SetStackParent(path, "feature/b", "feature/a", false); err != nil {
		t.Fatal(err)
	}
	if got := localConfig(t, fx, "branch.feature/b.gitrevengeStackParent"); got != "feature/a" {
		t.Errorf("stack parent = %q", got)
	}
	// Saving the preferences again leaves the stack parent alone.
	if err := a.SetRepoPreferences(path, p); err != nil {
		t.Fatal(err)
	}
	if got := localConfig(t, fx, "branch.feature/b.gitrevengeStackParent"); got != "feature/a" {
		t.Errorf("stack parent after saving preferences = %q", got)
	}

	if err := a.SetInsightsThresholds(insightsScope(path), Thresholds{Stale: 7, LongLived: 21, Behind: 40, Size: 600}); err != nil {
		t.Fatal(err)
	}
	if got := localConfig(t, fx, "gitrevenge-insights.stale"); got != "7" {
		t.Errorf("gitrevenge-insights.stale = %q", got)
	}
}

func TestStoredSettingsMoveToGitConfigOnce(t *testing.T) {
	a, fx, path := openForConfig(t)
	m, _ := a.sessions()
	// As a release from before the move left them: in the app's file only.
	err := m.Settings().Update(func(s *settings.Settings) {
		s.SetRepo(settings.RepoSettings{Path: path, Hide: []string{"wip/*"}, SyncNotes: true, Kinds: map[string][]string{"docs": {}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := a.syncRepoConfig(ctx, path); err != nil {
		t.Fatal(err)
	}
	if got := localConfig(t, fx, "gitrevenge.hide"); got != "wip/*" {
		t.Fatalf("gitrevenge.hide = %q, want the stored pattern moved in", got)
	}
	if got := localConfig(t, fx, "gitrevenge.syncNotes"); got != "true" {
		t.Fatalf("gitrevenge.syncNotes = %q", got)
	}
	held := m.Settings().Get().Repo(path)
	if !held.InGitConfig || !slices.Equal(held.Hide, []string{"wip/*"}) || !held.SyncNotes {
		t.Fatalf("the app's copy = %+v", held)
	}
	if p, ok := held.Kinds["docs"]; !ok || len(p) != 0 {
		t.Fatalf("docs kind = %v (set: %v), want set and empty", p, ok)
	}

	// From here git config is the word: taking a key out there takes it out of the app, and
	// the app doesn't put it back.
	fx.Git("config", "--local", "--unset-all", "gitrevenge.hide")
	// The repository's watcher may notice first and do the same read; either way it's done
	// once this returns.
	if _, err := a.syncRepoConfig(ctx, path); err != nil {
		t.Fatal(err)
	}
	if got := m.Settings().Get().Repo(path).Hide; len(got) != 0 {
		t.Fatalf("hide = %v, want none", got)
	}
	if got := localConfig(t, fx, "gitrevenge.hide"); got != "" {
		t.Fatalf("gitrevenge.hide = %q, want it to stay removed", got)
	}
}

func TestGitConfigWinsOverStoredSettings(t *testing.T) {
	a, fx, path := openForConfig(t)
	m, _ := a.sessions()
	fx.Git("config", "--local", "--add", "gitrevenge.hide", "from-config/*")
	err := m.Settings().Update(func(s *settings.Settings) {
		s.SetRepo(settings.RepoSettings{Path: path, Hide: []string{"from-app/*"}, CleanupOnPull: true})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.syncRepoConfig(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	held := m.Settings().Get().Repo(path)
	if !slices.Equal(held.Hide, []string{"from-config/*"}) || held.CleanupOnPull {
		t.Fatalf("the app's copy = %+v, want what the repository's config says", held)
	}
	if got := localConfig(t, fx, "gitrevenge.cleanupOnPull"); got != "" {
		t.Fatalf("gitrevenge.cleanupOnPull = %q, want the config left as it was", got)
	}
}

func TestARepositoryInheritsGlobalSettings(t *testing.T) {
	a, fx, path := openForConfig(t)
	setGlobal(t, a, path, "--add", "gitrevenge.hide", "wip/*")
	setGlobal(t, a, path, "--add", "gitrevenge-kind.release.pattern", "rel/*")
	if _, err := a.syncRepoConfig(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	p, err := a.RepoPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Hide, []string{"wip/*"}) {
		t.Fatalf("hide = %v, want the global pattern", p.Hide)
	}
	if k := kindOf(p, "release"); !k.Custom || !slices.Equal(k.Patterns, []string{"rel/*"}) {
		t.Fatalf("release = %+v", k)
	}

	// Changing one thing writes that one thing; what's inherited stays inherited.
	p.SyncNotes = true
	if err := a.SetRepoPreferences(path, p); err != nil {
		t.Fatal(err)
	}
	if got := localConfig(t, fx, "gitrevenge.syncNotes"); got != "true" {
		t.Errorf("gitrevenge.syncNotes = %q", got)
	}
	if got := localConfig(t, fx, "gitrevenge.hide"); got != "" {
		t.Errorf("gitrevenge.hide = %q, want it left to the global config", got)
	}
	if got := localConfig(t, fx, "gitrevenge-kind.release.pattern"); got != "" {
		t.Errorf("release patterns = %q, want them left to the global config", got)
	}
}

func TestAFolderThatIsNotARepositoryKeepsItsSettingsInTheApp(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	dir := t.TempDir()
	p, _ := a.RepoPreferences(dir)
	p.Hide = []string{"wip/*"}
	if err := a.SetRepoPreferences(dir, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.RepoPreferences(dir); !slices.Equal(got.Hide, []string{"wip/*"}) {
		t.Fatalf("hide = %v", got.Hide)
	}
}
