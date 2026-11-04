package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestCloneOpensAsRepo(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	_, remote := gittest.WithRemote(t, 2)
	parent := t.TempDir()

	dir, err := a.Clone(remote.Dir, parent, a.CloneName(remote.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(parent, filepath.Base(remote.Dir)); dir != want {
		t.Fatalf("dir = %q, want %q", dir, want)
	}
	tab, err := a.OpenRepo(dir)
	if err != nil || tab.Head != "main" {
		t.Fatalf("tab = %+v, %v", tab, err)
	}
}

func TestCloneIntoEmptyFolder(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	_, remote := gittest.WithRemote(t, 1)
	parent := t.TempDir()
	if err := os.Mkdir(filepath.Join(parent, "here"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Clone(remote.Dir, parent, "here"); err != nil {
		t.Fatal(err)
	}
}

func TestCloneRefusesNonEmptyFolder(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	_, remote := gittest.WithRemote(t, 1)
	parent := t.TempDir()
	keep := filepath.Join(parent, "taken", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := a.Clone(remote.Dir, parent, "taken")
	if err == nil || !strings.Contains(err.Error(), "isn't an empty folder") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("existing file was touched")
	}
}

func TestCloneValidatesInput(t *testing.T) {
	a := newTestApp(t)
	parent := t.TempDir()
	for _, c := range []struct{ url, parent, name string }{
		{"", parent, "x"},
		{"https://example.com/x.git", "relative", "x"},
		{"https://example.com/x.git", filepath.Join(parent, "missing"), "x"},
		{"https://example.com/x.git", parent, ""},
		{"https://example.com/x.git", parent, ".."},
		{"https://example.com/x.git", parent, "a/b"},
	} {
		if _, err := a.Clone(c.url, c.parent, c.name); err == nil {
			t.Errorf("Clone(%q, %q, %q) succeeded", c.url, c.parent, c.name)
		} else if ue := userError(""); !errors.As(err, &ue) {
			t.Errorf("Clone(%q, %q, %q) = %v, want a user error", c.url, c.parent, c.name, err)
		}
	}
}

func TestCloneMissingRepoIsExplained(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	parent := t.TempDir()

	_, err := a.Clone(filepath.Join(t.TempDir(), "nope"), parent, "copy")
	if err == nil || !strings.Contains(err.Error(), "No repository was found") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "copy")); !os.IsNotExist(err) {
		t.Fatalf("folder left behind: %v", err)
	}
}

func TestCancelCloneCleansUp(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	reached := make(chan struct{}, 1)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case reached <- struct{}{}:
		default:
		}
		select { // hang like a slow server until git goes away
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)
	parent := t.TempDir()

	done := make(chan error, 1)
	go func() {
		_, err := a.Clone(srv.URL+"/repo.git", parent, "repo")
		done <- err
	}()
	select {
	case <-reached:
	case <-time.After(30 * time.Second):
		t.Fatal("clone never reached the server")
	}
	a.CancelClone()

	select {
	case err := <-done:
		if err == nil || err.Error() != "Cancelled." {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("clone did not stop")
	}
	if _, err := os.Stat(filepath.Join(parent, "repo")); !os.IsNotExist(err) {
		t.Fatalf("folder left behind: %v", err)
	}
}

func TestRemoveCloneKeepsExistingFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755); err != nil {
		t.Fatal(err)
	}

	removeClone(dir, true)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("entries = %v, %v", entries, err)
	}

	removeClone(dir, false)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("folder not removed: %v", err)
	}
}

func TestInitCreatesRepo(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	dir := filepath.Join(t.TempDir(), "fresh")

	got, err := a.Init(dir, "trunk")
	if err != nil {
		t.Fatal(err)
	}
	tab, err := a.OpenRepo(got)
	if err != nil || tab.Head != "trunk" {
		t.Fatalf("tab = %+v, %v", tab, err)
	}
}

func TestInitRefusesExistingRepo(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)

	if _, err := a.Init(fx.Dir, ""); err == nil || !strings.Contains(err.Error(), "already a git repository") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitBadBranchIsExplained(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)

	_, err := a.Init(filepath.Join(t.TempDir(), "x"), "bad..name")
	if err == nil || strings.HasPrefix(err.Error(), "fatal:") {
		t.Fatalf("err = %v", err)
	}
}
