package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// #377: the global config usually sits in the home folder, and watching that folder silenced
// every repository inside it.
func TestRepoInsideGlobalConfigFolderReportsChanges(t *testing.T) {
	for _, order := range []struct {
		name        string
		configFirst bool
	}{{"config read first", true}, {"repository opened first", false}} {
		t.Run(order.name, func(t *testing.T) {
			isolateGit(t)
			fx := gittest.New(t)
			fx.Linear(1)
			global := filepath.Join(filepath.Dir(fx.Dir), ".gitconfig")
			if err := os.WriteFile(global, []byte("[user]\n\tname = T\n\temail = t@example.invalid\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_CONFIG_GLOBAL", global)

			a := newTestApp(t)
			a.configBackupDir = t.TempDir()
			changed := make(chan RepoChange, 16)
			a.emitHook = func(name string, data any) {
				if c, ok := data.(RepoChange); ok && name == "repo:changed" {
					changed <- c
				}
			}
			startConfig := func() {
				if _, err := a.gitConfig(); err != nil {
					t.Fatal(err)
				}
			}
			if order.configFirst {
				startConfig()
			}
			if _, err := a.OpenRepo(fx.Dir); err != nil {
				t.Fatal(err)
			}
			if !order.configFirst {
				startConfig()
			}
			time.Sleep(300 * time.Millisecond)

			fx.Git("branch", "seen-by-the-watcher")
			select {
			case <-changed:
			case <-time.After(5 * time.Second):
				t.Fatal("no repo:changed for a repository inside the global config's folder")
			}
		})
	}
}
