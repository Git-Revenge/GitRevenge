package gitconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// watching runs a Watcher that checks often, and returns what it reports.
func watching(t *testing.T) (*Watcher, <-chan string) {
	t.Helper()
	got := make(chan string, 16)
	w := newWatcher(func(p string) { got <- p }, 10*time.Millisecond)
	t.Cleanup(w.Close)
	return w, got
}

func expectReport(t *testing.T, got <-chan string, want string) {
	t.Helper()
	select {
	case p := <-got:
		if p != want {
			t.Errorf("reported %s, want %s", p, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("no change reported for %s", want)
	}
}

func expectQuiet(t *testing.T, got <-chan string) {
	t.Helper()
	select {
	case p := <-got:
		t.Errorf("unexpected report %s", p)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWatcherReportsOnlyChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitconfig")
	write(t, path, "[a]\n")
	w, got := watching(t)
	w.Add(path)
	expectQuiet(t, got)

	write(t, filepath.Join(filepath.Dir(path), "unrelated"), "x")
	expectQuiet(t, got)

	write(t, path, "[bigger]\n")
	expectReport(t, got, path)
	expectQuiet(t, got)
}

func TestWatcherNoticesASameSizeEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitconfig")
	write(t, path, "[a]\n")
	w, got := watching(t)
	w.Add(path)

	write(t, path, "[b]\n")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	expectReport(t, got, path)
}

// Editors and git replace config files by renaming over them.
func TestWatcherNoticesAReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gitconfig")
	write(t, path, "[a]\n")
	w, got := watching(t)
	w.Add(path)

	next := filepath.Join(dir, "gitconfig.lock")
	write(t, next, "[replaced]\n")
	if err := os.Rename(next, path); err != nil {
		t.Fatal(err)
	}
	expectReport(t, got, path)
}

func TestWatcherNoticesCreationAndDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not", "yet", "gitconfig")
	w, got := watching(t)
	w.Add(path)
	expectQuiet(t, got)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, path, "[a]\n")
	expectReport(t, got, path)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	expectReport(t, got, path)
}

func TestWatcherFollowsASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	write(t, target, "[a]\n")
	link := filepath.Join(dir, "gitconfig")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("can't make a symlink here: %v", err)
	}
	w, got := watching(t)
	w.Add(link)

	write(t, target, "[longer]\n")
	expectReport(t, got, link)
}

func TestWatcherAddingAgainKeepsWhatItKnows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitconfig")
	write(t, path, "[a]\n")
	w, got := watching(t)
	w.Add(path)
	w.Add(path)
	expectQuiet(t, got)

	write(t, path, "[bigger]\n")
	expectReport(t, got, path)
	expectQuiet(t, got)
}

func TestWatcherStopsReportingWhenClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitconfig")
	write(t, path, "[a]\n")
	got := make(chan string, 16)
	w := newWatcher(func(p string) { got <- p }, 10*time.Millisecond)
	w.Add(path)
	w.Close()

	write(t, path, "[bigger]\n")
	expectQuiet(t, got)
}
