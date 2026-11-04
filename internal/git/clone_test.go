package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestClone(t *testing.T) {
	_, remote := gittest.WithRemote(t, 2)
	r := newTestRunner(t)
	dir := filepath.Join(t.TempDir(), "copy")

	if err := Clone(context.Background(), r, remote.Dir, dir, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := OpenRepo(r, dir).ReadString(context.Background(), "rev-parse", "HEAD")
	if err != nil || got != remote.Git("rev-parse", "main") {
		t.Fatalf("HEAD = %q, %v", got, err)
	}
}

func TestCloneFailureLeavesNoFolder(t *testing.T) {
	r := newTestRunner(t)
	dir := filepath.Join(t.TempDir(), "copy")

	if err := Clone(context.Background(), r, filepath.Join(t.TempDir(), "missing"), dir, NetOptions{}); err == nil {
		t.Fatal("clone of a missing repository succeeded")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("target folder left behind: %v", err)
	}
}

func TestInit(t *testing.T) {
	r := newTestRunner(t)
	dir := filepath.Join(t.TempDir(), "new", "repo")

	if err := Init(context.Background(), r, dir, "trunk"); err != nil {
		t.Fatal(err)
	}
	got, err := OpenRepo(r, dir).ReadString(context.Background(), "symbolic-ref", "--short", "HEAD")
	if err != nil || got != "trunk" {
		t.Fatalf("branch = %q, %v", got, err)
	}
}

func TestCloneName(t *testing.T) {
	for url, want := range map[string]string{
		"https://github.com/owner/repo.git": "repo",
		"https://github.com/owner/repo":     "repo",
		"https://github.com/owner/repo/":    "repo",
		"git@github.com:owner/repo.git":     "repo",
		"git@host:repo.git":                 "repo",
		"ssh://git@host:22/owner/repo.git/": "repo",
		"/srv/git/project/.git":             "project",
		`C:\src\tool`:                       "tool",
		"  https://example.com/a/b.git  ":   "b",
		"":                                  "",
		"https://example.com/..":            "",
	} {
		if got := CloneName(url); got != want {
			t.Errorf("CloneName(%q) = %q, want %q", url, got, want)
		}
	}
}
