package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// newConfigTestApp returns an App whose global config is an isolated temp file.
func newConfigTestApp(t *testing.T) (*App, string) {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = Global\n[alias]\n\tst = status\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	a := newTestApp(t)
	a.configBackupDir = filepath.Join(t.TempDir(), "config-backups")
	return a, global
}

func TestGitConfigEditSaveRestore(t *testing.T) {
	a, global := newConfigTestApp(t)
	fx := gittest.New(t)

	scopes, err := a.GitConfigScopes(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if scopes.Global != global || !sameFileT(t, scopes.Local, filepath.Join(fx.Dir, ".git", "config")) {
		t.Fatalf("scopes = %+v", scopes)
	}

	doc, err := a.OpenGitConfig(scopes.Local)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Dirty || !doc.Exists || !hasEntry(doc, "user.name", "Test User") {
		t.Fatalf("opened doc = %+v", doc)
	}

	steps := []func() (ConfigDoc, error){
		func() (ConfigDoc, error) { return a.SetGitConfig(scopes.Local, "pull.rebase", "true") },
		func() (ConfigDoc, error) { return a.AddGitConfigValue(scopes.Local, "remote.origin.fetch", "a") },
		func() (ConfigDoc, error) { return a.AddGitConfigValue(scopes.Local, "remote.origin.fetch", "b") },
		func() (ConfigDoc, error) {
			return a.ReplaceGitConfigValue(scopes.Local, "remote.origin.fetch", "a", "c")
		},
		func() (ConfigDoc, error) { return a.UnsetGitConfig(scopes.Local, "remote.origin.fetch", "b") },
		func() (ConfigDoc, error) { return a.SetGitConfig(scopes.Local, "alias.co", "checkout") },
		func() (ConfigDoc, error) { return a.RenameGitConfigKey(scopes.Local, "alias.co", "alias.sw") },
	}
	for i, step := range steps {
		if doc, err = step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if !doc.Dirty || !hasEntry(doc, "pull.rebase", "true") || !hasEntry(doc, "remote.origin.fetch", "c") ||
		hasEntry(doc, "remote.origin.fetch", "b") || !hasEntry(doc, "alias.sw", "checkout") || hasEntry(doc, "alias.co", "checkout") {
		t.Fatalf("after edits = %+v", doc.Entries)
	}
	if strings.Contains(fx.Git("config", "--list", "--local"), "pull.rebase") {
		t.Fatal("edits reached the file before save")
	}
	if _, err := a.UnsetGitConfig(scopes.Local, "no.such", ""); err == nil {
		t.Error("unsetting a missing key succeeded")
	}

	res, err := a.SaveGitConfig(scopes.Local, false)
	if err != nil || !res.Saved || res.Doc.Dirty {
		t.Fatalf("save = %+v, %v", res, err)
	}
	if got := fx.Git("config", "pull.rebase"); got != "true" {
		t.Errorf("pull.rebase on disk = %q", got)
	}

	backups, err := a.GitConfigBackups(scopes.Local)
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, %v", backups, err)
	}
	if text, _ := a.GitConfigBackupText(scopes.Local, backups[0].ID); strings.Contains(text, "pull") {
		t.Errorf("backup has the new content: %q", text)
	}
	doc, err = a.RestoreGitConfig(scopes.Local, backups[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if hasEntry(doc, "pull.rebase", "true") || doc.Dirty {
		t.Errorf("after restore = %+v", doc)
	}
	if _, err := fx.TryGit("config", "pull.rebase"); err == nil {
		t.Error("restore didn't reach the file")
	}
	if backups, _ := a.GitConfigBackups(scopes.Local); len(backups) != 2 {
		t.Errorf("restore didn't back up the current file: %v", backups)
	}
}

func TestGitConfigRawTextAndConflicts(t *testing.T) {
	a, global := newConfigTestApp(t)
	if _, err := a.OpenGitConfig(global); err != nil {
		t.Fatal(err)
	}

	doc, err := a.SetGitConfigText(global, "[user]\n\tname = Raw\n[broken\n")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Problem == nil || doc.Problem.Line != 3 {
		t.Fatalf("problem = %+v", doc.Problem)
	}
	res, err := a.SaveGitConfig(global, false)
	if err != nil || res.Saved {
		t.Fatalf("saved invalid config: %+v, %v", res, err)
	}

	if _, err := a.SetGitConfigText(global, "[user]\n\tname = Raw\n"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte("[user]\n\tname = Elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err = a.SaveGitConfig(global, false)
	if err != nil || res.Saved || !res.Doc.ChangedOnDisk || !strings.Contains(res.DiskText, "Elsewhere") {
		t.Fatalf("save over external change = %+v, %v", res, err)
	}
	if res, err = a.SaveGitConfig(global, true); err != nil || !res.Saved {
		t.Fatalf("forced save = %+v, %v", res, err)
	}

	doc, err = a.ReloadGitConfig(global)
	if err != nil || !hasEntry(doc, "user.name", "Raw") || doc.Dirty {
		t.Errorf("reload = %+v, %v", doc, err)
	}
	if err := a.CloseGitConfig(global); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetGitConfig(global, "a.b", "c"); err == nil {
		t.Error("edited a closed file")
	}
}

func TestEffectiveGitConfig(t *testing.T) {
	a, global := newConfigTestApp(t)
	fx := gittest.New(t)
	entries, err := a.EffectiveGitConfig(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(entries, func(e git.ConfigEntry) bool { return e.Key == "alias.st" })
	if i < 0 || entries[i].Scope != "global" || !sameFileT(t, entries[i].File, global) {
		t.Errorf("alias.st = %+v", entries)
	}
	if !slices.ContainsFunc(entries, func(e git.ConfigEntry) bool { return e.Scope == "local" && e.Key == "user.name" }) {
		t.Error("local entries missing")
	}
}

func TestGitConfigFileText(t *testing.T) {
	a, global := newConfigTestApp(t)
	if err := os.WriteFile(global, []byte("[user]\r\n\tname = A\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GitConfigFileText(global); err != nil || got != "[user]\n\tname = A\n" {
		t.Errorf("text = %q, %v; want LF line endings", got, err)
	}
	if got, err := a.GitConfigFileText(filepath.Join(t.TempDir(), "missing")); err != nil || got != "" {
		t.Errorf("missing file = %q, %v", got, err)
	}
}

func hasEntry(d ConfigDoc, key, value string) bool {
	return slices.ContainsFunc(d.Entries, func(e git.ConfigEntry) bool { return e.Key == key && e.Value == value })
}

func sameFileT(t *testing.T, a, b string) bool {
	t.Helper()
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}
