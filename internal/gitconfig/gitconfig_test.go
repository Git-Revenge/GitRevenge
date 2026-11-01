package gitconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func noValidate(string) error { return nil }

func newBackups(t *testing.T) *Backups {
	t.Helper()
	return &Backups{Dir: filepath.Join(t.TempDir(), "config-backups"), Keep: DefaultKeep}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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

func TestDocSaveKeepsCRLFAndBacksUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	write(t, path, "[user]\r\n\tname = A\r\n")
	b := newBackups(t)
	d, err := Open(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if text, _ := d.Text(); text != "[user]\n\tname = A\n" {
		t.Errorf("working copy = %q, want LF", text)
	}
	if dirty, _ := d.Dirty(); dirty {
		t.Error("dirty right after open")
	}
	if err := d.SetText("[user]\n\tname = B\n"); err != nil {
		t.Fatal(err)
	}
	if dirty, _ := d.Dirty(); !dirty {
		t.Error("not dirty after an edit")
	}
	if err := d.Save(noValidate, b, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "[user]\r\n\tname = B\r\n" {
		t.Errorf("saved %q, want CRLF kept", got)
	}
	if dirty, _ := d.Dirty(); dirty {
		t.Error("dirty after save")
	}
	list, err := b.List(path)
	if err != nil || len(list) != 1 {
		t.Fatalf("backups = %v, %v", list, err)
	}
	if got, _ := b.Read(path, list[0].ID); string(got) != "[user]\r\n\tname = A\r\n" {
		t.Errorf("backup = %q", got)
	}
}

func TestDocSaveNewFileHasNoBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "git", "config")
	b := newBackups(t)
	d, err := Open(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Exists() {
		t.Error("missing file reported as existing")
	}
	_ = d.SetText("[a]\n\tb = 1\n")
	if err := d.Save(noValidate, b, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "[a]\n\tb = 1\n" {
		t.Errorf("saved %q", got)
	}
	if list, _ := b.List(path); len(list) != 0 {
		t.Errorf("backups of a new file = %v", list)
	}
}

func TestDocSaveRefusesInvalidAndExternalChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	write(t, path, "[a]\n\tb = 1\n")
	b := newBackups(t)
	d, err := Open(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	_ = d.SetText("[a]\n\tb = 2\n")

	invalid := errors.New("invalid")
	if err := d.Save(func(string) error { return invalid }, b, false); !errors.Is(err, invalid) {
		t.Errorf("save with a failing validation = %v", err)
	}

	write(t, path, "[a]\n\tb = 3\n")
	if changed, _ := d.ChangedOnDisk(); !changed {
		t.Error("external change not noticed")
	}
	if err := d.Save(noValidate, b, false); !errors.Is(err, ErrChangedOnDisk) {
		t.Fatalf("save over an external change = %v", err)
	}
	if got := read(t, path); got != "[a]\n\tb = 3\n" {
		t.Errorf("external change overwritten: %q", got)
	}
	if err := d.Save(noValidate, b, true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "[a]\n\tb = 2\n" {
		t.Errorf("forced save = %q", got)
	}
}

func TestWriteRespectsGitLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	write(t, path, "x")
	write(t, path+".lock", "")
	if err := Write(path, []byte("y"), []byte("x"), nil); !errors.Is(err, ErrLocked) {
		t.Errorf("write while locked = %v", err)
	}
	_ = os.Remove(path + ".lock")
	if err := Write(path, []byte("y"), []byte("x"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Error("lock left behind")
	}
}

func TestWriteThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "gitconfig")
	_ = os.MkdirAll(filepath.Dir(target), 0o755)
	write(t, target, "old")
	link := filepath.Join(dir, ".gitconfig")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := Write(link, []byte("new"), []byte("old"), nil); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink replaced by a file")
	}
	if got := read(t, target); got != "new" {
		t.Errorf("target = %q", got)
	}
}

func TestBackupsPruneAndNaming(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	b := newBackups(t)
	b.Keep = 3
	clock := time.Date(2026, 10, 30, 14, 25, 1, 0, time.UTC)
	b.now = func() time.Time { return clock }
	for i := range 5 {
		if _, err := b.Add(path, []byte{byte('0' + i)}); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			// The next one lands in the same millisecond.
			continue
		}
		clock = clock.Add(time.Second)
	}
	list, err := b.List(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("kept %d backups, want 3", len(list))
	}
	if got, _ := b.Read(path, list[0].ID); string(got) != "4" {
		t.Errorf("newest = %q", got)
	}
	for _, bk := range list {
		if strings.ContainsAny(bk.ID, `:\/`) {
			t.Errorf("backup id %q isn't a portable file name", bk.ID)
		}
	}
	if _, err := b.Read(path, "../../etc/passwd"); err == nil {
		t.Error("read outside the backup folder")
	}

	var m meta
	raw, err := os.ReadFile(filepath.Join(b.Dir, FileID(path), "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &m); err != nil || m.Path != path {
		t.Errorf("meta = %+v, %v; want path %s", m, err, path)
	}
}

func TestFileIDFoldsCaseWhereTheFileSystemDoes(t *testing.T) {
	a := filepath.Join(t.TempDir(), "Config")
	b := filepath.Join(filepath.Dir(a), "config")
	same := FileID(a) == FileID(b)
	if want := runtime.GOOS == "windows" || runtime.GOOS == "darwin"; same != want {
		t.Errorf("FileID case-insensitive = %v, want %v", same, want)
	}
	if FileID(a) != FileID(filepath.Join(filepath.Dir(a), ".", "Config")) {
		t.Error("FileID not normalized")
	}
}
