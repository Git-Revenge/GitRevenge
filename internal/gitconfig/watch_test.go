package gitconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Long enough that steps of one write in a test fall within it, even on a busy machine.
const testSettle = 250 * time.Millisecond

// watching runs a Watcher that checks often, and returns what it reports.
func watching(t *testing.T) (*Watcher, <-chan string) {
	t.Helper()
	got := make(chan string, 16)
	w := newWatcher(func(p string) { got <- p }, 10*time.Millisecond, testSettle)
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
	case <-time.After(3 * testSettle):
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

// #379: a write that isn't atomic truncates first, and a check can land between the steps.
func TestWatcherWaitsForAWriteToFinish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitconfig")
	write(t, path, "[a]\n")
	w, got := watching(t)
	w.Add(path)

	write(t, path, "")
	time.Sleep(testSettle / 6) // Longer than the 10ms between checks.
	write(t, path, "[finished]\n")
	expectReport(t, got, path)
	expectQuiet(t, got)
}

func TestWatcherReportsAFileThatNeverSettles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitconfig")
	write(t, path, "[a]\n")
	got := make(chan string, 16)
	w := newWatcher(func(p string) { got <- p }, 10*time.Millisecond, 10*time.Millisecond)
	defer w.Close()
	w.Add(path)

	// The writer has to be gone before the test ends: a write still under way would put the
	// file back while the temp dir is being removed, and the cleanup would fail.
	stop, stopped := make(chan struct{}), make(chan struct{})
	defer func() {
		close(stop)
		<-stopped
	}()
	go func() {
		defer close(stopped)
		content := "[a]\n"
		for {
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
				content += "x"
				_ = os.WriteFile(path, []byte(content), 0o644)
			}
		}
	}()
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("a file that keeps changing was never reported")
	}
}

func TestWatcherCloseDoesNotWaitOutASettlingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitconfig")
	write(t, path, "[a]\n")
	w := newWatcher(func(string) {}, 10*time.Millisecond, time.Hour)
	w.Add(path)
	write(t, path, "[bigger]\n")
	time.Sleep(50 * time.Millisecond) // Long enough to be waiting for it to settle.

	closed := make(chan struct{})
	go func() {
		w.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close waited for the file to settle")
	}
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
	w := newWatcher(func(p string) { got <- p }, 10*time.Millisecond, testSettle)
	w.Add(path)
	w.Close()

	write(t, path, "[bigger]\n")
	expectQuiet(t, got)
}
