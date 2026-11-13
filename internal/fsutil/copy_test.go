package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCopyDirIfMissingCopiesTheTree(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "old"), filepath.Join(root, "new")
	write(t, filepath.Join(src, "WebsiteData", "LocalStorage", "wails.localstorage"), "layout")
	write(t, filepath.Join(src, "top"), "x")

	copied, err := CopyDirIfMissing(src, dst)
	if err != nil || !copied {
		t.Fatalf("copied=%v err=%v", copied, err)
	}
	if got := read(t, filepath.Join(dst, "WebsiteData", "LocalStorage", "wails.localstorage")); got != "layout" {
		t.Fatalf("nested file = %q", got)
	}
	if got := read(t, filepath.Join(dst, "top")); got != "x" {
		t.Fatalf("top file = %q", got)
	}
	// The source stays, so an older version run afterwards still finds its data.
	if got := read(t, filepath.Join(src, "top")); got != "x" {
		t.Fatalf("source changed: %q", got)
	}
}

func TestCopyDirIfMissingLeavesAnExistingDestination(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "old"), filepath.Join(root, "new")
	write(t, filepath.Join(src, "f"), "old")
	write(t, filepath.Join(dst, "f"), "new")

	copied, err := CopyDirIfMissing(src, dst)
	if err != nil || copied {
		t.Fatalf("copied=%v err=%v", copied, err)
	}
	if got := read(t, filepath.Join(dst, "f")); got != "new" {
		t.Fatalf("destination overwritten: %q", got)
	}
}

func TestCopyDirIfMissingWithNoSource(t *testing.T) {
	root := t.TempDir()
	copied, err := CopyDirIfMissing(filepath.Join(root, "none"), filepath.Join(root, "new"))
	if err != nil || copied {
		t.Fatalf("copied=%v err=%v", copied, err)
	}
	if _, err := os.Stat(filepath.Join(root, "new")); !os.IsNotExist(err) {
		t.Fatalf("destination created: %v", err)
	}
}

func TestCopyDirIfMissingRemovesAHalfCopy(t *testing.T) {
	root := t.TempDir()
	src, dst := filepath.Join(root, "old"), filepath.Join(root, "new")
	write(t, filepath.Join(src, "f"), "x")
	// A symlink can't be copied as a file, so the copy fails part way.
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(src, "zz-link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	if _, err := CopyDirIfMissing(src, dst); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("half-copied destination left behind: %v", err)
	}
}
