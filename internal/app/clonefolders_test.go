package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// withFolders is a repository to clone with only some folders, over file:// as from a server.
func withFolders(t *testing.T) string {
	t.Helper()
	src := gittest.New(t)
	for _, f := range []string{"top.txt", "src/a.go", "src/sub/b.go", "docs/d.md"} {
		src.WriteFile(f, f+"\n")
	}
	src.Git("add", ".")
	src.Commit("files")
	return "file://" + filepath.ToSlash(src.Dir)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestCloneWithChosenFolders(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	parent := t.TempDir()

	dir, err := a.Clone(withFolders(t), parent, "some", git.CloneOptions{Folders: true})
	if err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(dir, "top.txt")) {
		t.Fatal("the first step wrote files")
	}
	counts, err := a.CloneFolders(dir)
	if err != nil || len(counts) != 4 {
		t.Fatalf("folders = %+v, %v", counts, err)
	}
	if err := a.CloneCheckout(dir, []string{"src"}); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]bool{"top.txt": true, "src/a.go": true, "src/sub/b.go": true, "docs/d.md": false} {
		if got := exists(filepath.Join(dir, filepath.FromSlash(f))); got != want {
			t.Errorf("%s on disk = %v, want %v", f, got, want)
		}
	}
	// Once written, it's a clone like any other: nothing waits for folders.
	if _, err := a.CloneFolders(dir); !errors.Is(err, notHalfCloned) {
		t.Errorf("after the checkout: %v", err)
	}
}

func TestCloneAbandonTakesTheCloneAway(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	parent := t.TempDir()
	dir, err := a.Clone(withFolders(t), parent, "gone", git.CloneOptions{Folders: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CloneAbandon(dir); err != nil {
		t.Fatal(err)
	}
	if exists(dir) {
		t.Fatal("the clone was left behind")
	}
}

func TestCloneFoldersTouchesOnlyItsOwnClones(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	other := t.TempDir()
	keep := filepath.Join(other, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CloneFolders(other); !errors.Is(err, notHalfCloned) {
		t.Errorf("CloneFolders: %v", err)
	}
	if err := a.CloneCheckout(other, nil); !errors.Is(err, notHalfCloned) {
		t.Errorf("CloneCheckout: %v", err)
	}
	if err := a.CloneAbandon(other); !errors.Is(err, notHalfCloned) {
		t.Errorf("CloneAbandon: %v", err)
	}
	if !exists(keep) {
		t.Fatal("a folder the app didn't clone was touched")
	}
}
