package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/fsutil"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// hostile is a repository as it would arrive in an archive: its own config tells git to run a
// program every time its files are checked, which is the first thing opening it does.
func hostile(t *testing.T) (fx *gittest.Repo, proof string) {
	t.Helper()
	fx = gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	fx.Git("config", "core.fsmonitor", "echo ran > ran.txt; false")
	// Setting it ran git, and so the program. Start clean.
	proof = filepath.Join(fx.Dir, "ran.txt")
	_ = os.Remove(proof)
	return fx, proof
}

// GHSA-r6gg-5grh-qv76, end to end: opening such a repository used to run its program at
// once. Now nothing runs until the user has seen what it would run and agreed.
func TestARepositoryThatRunsProgramsAsksFirst(t *testing.T) {
	fx, proof := hostile(t)
	a := newTestApp(t)

	_, err := a.OpenRepo(fx.Dir)
	if err == nil || !strings.Contains(err.Error(), "run programs") {
		t.Fatalf("OpenRepo err = %v, want it refused", err)
	}
	if _, statErr := os.Stat(proof); statErr == nil {
		t.Fatal("the repository's program ran before anyone agreed to it")
	}
	if tabs, _ := a.RestoreSession(); len(tabs.Tabs) != 0 {
		t.Errorf("a tab was opened: %+v", tabs.Tabs)
	}

	q := a.TakeTrustQuestion()
	if q == nil || !fsutil.SameFolder(q.Path, fx.Dir) || q.Name != filepath.Base(fx.Dir) {
		t.Fatalf("question = %+v", q)
	}
	if len(q.Findings) != 1 || q.Findings[0].Name != "core.fsmonitor" || !strings.Contains(q.Findings[0].Value, "echo ran") || q.Findings[0].Why == "" {
		t.Errorf("findings = %+v", q.Findings)
	}
	if again := a.TakeTrustQuestion(); again != nil {
		t.Errorf("the question was kept after it was taken: %+v", again)
	}

	// Everything else that would run git there is refused too, and still runs nothing.
	if _, err := a.GitConfigScopes(fx.Dir); err == nil {
		t.Error("GitConfigScopes went ahead in an untrusted repository")
	}
	if _, err := a.EffectiveGitConfig(fx.Dir); err == nil {
		t.Error("EffectiveGitConfig went ahead in an untrusted repository")
	}
	if _, err := a.LinkRules(fx.Dir); err == nil {
		t.Error("LinkRules went ahead in an untrusted repository")
	}
	if _, statErr := os.Stat(proof); statErr == nil {
		t.Fatal("the repository's program ran before anyone agreed to it")
	}

	// The user agrees: it opens, and stays trusted.
	if err := a.TrustRepo(fx.Dir); err != nil {
		t.Fatal(err)
	}
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil || !fsutil.SameFolder(tab.Path, fx.Dir) {
		t.Fatalf("after trusting: %+v, %v", tab, err)
	}
	if a.TakeTrustQuestion() != nil {
		t.Error("a question was left after a repository opened")
	}
}

// A repository whose config runs nothing opens as it always did. Once opened it's the
// user's: what they set up in it later (a hook manager, say) doesn't stop it reopening.
func TestAnOrdinaryRepositoryOpensWithoutAsking(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	a := newTestApp(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.TakeTrustQuestion() != nil {
		t.Error("asked about a repository that runs nothing")
	}
	if err := a.CloseRepo(tab.Path); err != nil {
		t.Fatal(err)
	}
	fx.Git("config", "core.hooksPath", ".husky/_")
	if _, err := a.OpenRepo(fx.Dir); err != nil {
		t.Errorf("reopening after the user added a hooks folder: %v", err)
	}
}

// Looking at a repository without opening it as a tab (a group's status, its config) is
// refused if it would run something, and doesn't count as having opened it.
func TestOnlyOpeningATabTrustsARepository(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	a := newTestApp(t)
	if _, err := a.GitConfigScopes(fx.Dir); err != nil {
		t.Fatal(err)
	}
	m, _ := a.sessions()
	if trusted := m.Settings().Get().TrustedRepos; len(trusted) != 0 {
		t.Errorf("trusted without being opened: %v", trusted)
	}
}

// Repositories the settings knew before this version are trusted as they stand, so an
// update doesn't stop at every tab.
func TestRepositoriesFromBeforeAreTrusted(t *testing.T) {
	fx, _ := hostile(t)
	a := newTestApp(t)
	old := `{"version":1,"recent":[{"path":` + quoteJSON(fx.Dir) + `,"lastOpened":"2026-11-01T00:00:00Z"}],"openTabs":[` + quoteJSON(fx.Dir) + `],"groups":[]}`
	if err := os.WriteFile(a.settingsPath, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := a.RestoreSession()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Tabs) != 1 || len(st.Failed) != 0 {
		t.Fatalf("restored %+v, failed %+v", st.Tabs, st.Failed)
	}
	m, _ := a.sessions()
	if s := m.Settings().Get(); !s.TrustSeeded || !s.Trusts(fx.Dir) {
		t.Errorf("settings after the update: seeded %v, trusted %v", s.TrustSeeded, s.TrustedRepos)
	}

	// A repository that turns up afterwards is asked about like any other.
	other, _ := hostile(t)
	if _, err := a.OpenRepo(other.Dir); err == nil {
		t.Error("a new repository was trusted because older ones were")
	}
}

// A repository the app cloned holds only what git and the user's own settings put there.
func TestAClonedRepositoryIsTrusted(t *testing.T) {
	isolateGit(t)
	src := gittest.New(t)
	src.CommitFile("a.txt", "a\n", "one")
	a := newTestApp(t)
	parent := t.TempDir()
	dir, err := a.Clone(src.Dir, parent, "copy")
	if err != nil {
		t.Fatal(err)
	}
	m, _ := a.sessions()
	if s := m.Settings().Get(); !s.Trusts(dir) {
		t.Errorf("the clone isn't trusted: %v", s.TrustedRepos)
	}
	// So a hook the user adds to it later doesn't stop it opening.
	if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenRepo(dir); err != nil {
		t.Errorf("OpenRepo: %v", err)
	}
}

func quoteJSON(s string) string {
	return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
}
