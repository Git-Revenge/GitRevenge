package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestDroppedFilesBecomeReadable(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	var mu sync.Mutex
	var sent []DroppedFiles
	a.emitHook = func(name string, data any) {
		if name == "patches:dropped" {
			mu.Lock()
			sent = append(sent, data.(DroppedFiles))
			mu.Unlock()
		}
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "0001-fix.patch")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if a.chosen.has(file) {
		t.Fatal("not chosen before it's dropped")
	}
	// A folder dropped with it is left out.
	a.filesDropped([]string{file, dir, filepath.Join(dir, "gone.patch")})
	if !a.chosen.has(file) || a.chosen.has(dir) {
		t.Error("the dropped file, and only it, is chosen")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 || !slices.Equal(sent[0].Paths, []string{file}) {
		t.Errorf("sent = %+v", sent)
	}
}

func TestReadPatchFilesMakesOneSeriesInNameOrder(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, _ := seriesRepo(t)
	path := openForMap(t, a, fx)
	dir := t.TempDir()
	fx.Git("format-patch", "-q", "-3", "side", "-o", dir)
	entries, _ := os.ReadDir(dir)
	var files []string
	for i := len(entries) - 1; i >= 0; i-- {
		files = append(files, filepath.Join(dir, entries[i].Name()))
	}
	if len(files) != 3 {
		t.Fatalf("files = %v", files)
	}
	// Not dropped: not read.
	if _, err := a.ReadPatchFiles(path, files); err == nil {
		t.Fatal("files nobody chose shouldn't be read")
	}
	a.filesDropped(files)
	f, err := a.ReadPatchFiles(path, files)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(f.Token)
	if f.Kind != "mail" || len(f.Messages) != 3 || !strings.HasPrefix(f.Name, "0001-") || !strings.HasSuffix(f.Name, " and 2 more") {
		t.Fatalf("file = %s %s, %d messages", f.Name, f.Kind, len(f.Messages))
	}
	var subjects []string
	for _, m := range f.Messages {
		subjects = append(subjects, m.Subject)
	}
	if !slices.Equal(subjects, []string{"Change line 2", "Change line 10", "Change line 6"}) {
		t.Errorf("subjects = %v", subjects)
	}
}

func TestReadPatchFilesSaysWhichArentPatches(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, mbox := seriesRepo(t)
	path := openForMap(t, a, fx)
	dir := t.TempDir()
	notes := filepath.Join(dir, "notes.txt")
	diff := filepath.Join(dir, "change.diff")
	_ = os.WriteFile(notes, []byte("just notes\n"), 0o644)
	_ = os.WriteFile(diff, []byte("diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-aaa\n+AAA\n"), 0o644)
	a.filesDropped([]string{mbox, notes, diff})
	f, err := a.ReadPatchFiles(path, []string{mbox, notes})
	if err != nil || f.Kind != "" || f.Name != "notes.txt" {
		t.Fatalf("file = %+v, %v", f, err)
	}
	if _, err := a.ReadPatchFiles(path, []string{mbox, diff}); err == nil || !strings.Contains(err.Error(), "patches and diffs together") {
		t.Fatalf("err = %v", err)
	}
	one, err := a.ReadPatchFiles(path, []string{diff})
	if err != nil || one.Kind != "diff" || one.Name != "change.diff" {
		t.Fatalf("one diff = %+v, %v", one, err)
	}
	a.ClosePatches(one.Token)
}

func TestReadClipboardPatches(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, mbox := seriesRepo(t)
	path := openForMap(t, a, fx)
	series, _ := os.ReadFile(mbox)
	for _, tc := range []struct {
		name, text, kind string
		messages         int
	}{
		{"a series, with Windows line ends", strings.ReplaceAll(string(series), "\n", "\r\n"), "mail", 3},
		{"a diff", "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-aaa\n+AAA\n", "diff", 0},
		{"neither", "a shopping list", "", 0},
		{"nothing", "", "", 0},
	} {
		a.clipboardHook = func() (string, error) { return tc.text, nil }
		f, err := a.ReadClipboardPatches(path)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if f.Kind != tc.kind || len(f.Messages) != tc.messages || f.Name != "the clipboard" {
			t.Errorf("%s: kind %q, %d messages, name %q", tc.name, f.Kind, len(f.Messages), f.Name)
		}
		if tc.kind == "diff" && (f.Diff == nil || len(f.Diff.Files) != 1) {
			t.Errorf("%s: diff = %+v", tc.name, f.Diff)
		}
		if f.Token != "" {
			a.ClosePatches(f.Token)
		}
	}
}
