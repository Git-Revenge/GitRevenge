package app

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func archiveRepo(t *testing.T, a *App) (string, *gittest.Repo) {
	t.Helper()
	fx := gittest.New(t)
	fx.WriteFile(".gitattributes", "notes export-ignore\n")
	fx.WriteFile("src/app.js", "console.log(1)\n")
	fx.WriteFile("notes/todo.md", "- ship\n")
	fx.Commit("first")
	fx.Git("tag", "v1.0")
	return openForMap(t, a, fx), fx
}

func TestSaveArchiveAsksBeforeReplacingAndLeavesNothingBehind(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	path, _ := archiveRepo(t, a)
	dir := t.TempDir()
	o := git.ArchiveOptions{Format: "zip", Prefix: "proj-v1.0/"}

	got, err := a.SaveArchive(path, "v1.0", o, dir, "proj-v1.0.zip", false)
	if err != nil || got.Exists || got.File != "proj-v1.0.zip" {
		t.Fatalf("%+v, %v", got, err)
	}
	z, err := zip.OpenReader(filepath.Join(dir, "proj-v1.0.zip"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, f := range z.File {
		names = append(names, f.Name)
	}
	_ = z.Close()
	if strings.Contains(strings.Join(names, " "), "notes/") || !strings.Contains(strings.Join(names, " "), "proj-v1.0/src/app.js") {
		t.Errorf("names = %v", names)
	}

	// Saving again asks first, and writes nothing until asked.
	before, _ := os.Stat(filepath.Join(dir, "proj-v1.0.zip"))
	if got, err = a.SaveArchive(path, "v1.0", git.ArchiveOptions{Format: "zip", Sub: "src"}, dir, "proj-v1.0.zip", false); err != nil || !got.Exists {
		t.Fatalf("again: %+v, %v", got, err)
	}
	if after, _ := os.Stat(filepath.Join(dir, "proj-v1.0.zip")); after.Size() != before.Size() {
		t.Error("the file was replaced without asking")
	}
	if got, err = a.SaveArchive(path, "v1.0", git.ArchiveOptions{Format: "zip", Sub: "src"}, dir, "proj-v1.0.zip", true); err != nil || got.Exists {
		t.Fatalf("replace: %+v, %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("left behind: %v", entries)
	}
}

func TestSaveArchiveSaysWhatsWrong(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	path, _ := archiveRepo(t, a)
	dir := t.TempDir()
	zipped := git.ArchiveOptions{Format: "zip"}
	for _, c := range []struct {
		folder, name string
		o            git.ArchiveOptions
		want         string
	}{
		{filepath.Join(dir, "nowhere"), "a.zip", zipped, "That folder doesn't exist"},
		// Either slash, on every system: Linux would take the backslash as part of a name.
		{dir, `sub\a.zip`, zipped, "a file's name, without a folder"},
		{dir, "sub/a.zip", zipped, "a file's name, without a folder"},
		{dir, "a.zip", git.ArchiveOptions{Format: "zip", Sub: "src/app.js"}, "“src/app.js” isn't a folder in that commit."},
	} {
		if _, err := a.SaveArchive(path, "v1.0", c.o, c.folder, c.name, false); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %s: %v", c.folder, c.name, err)
		}
	}
	if _, err := a.ArchivePlan(path, "v1.0", "nope"); err == nil || err.Error() != "“nope” isn't a folder in that commit." {
		t.Errorf("plan of a missing folder: %v", err)
	}
	plan, err := a.ArchivePlan(path, "v1.0", "")
	if err != nil || plan.Files != 2 || plan.Ignored != 1 {
		t.Errorf("plan: %+v, %v", plan, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left behind: %v", entries)
	}
}

// Stopped while it writes, it leaves nothing in the folder: not the half-written archive.
func TestStopArchiveLeavesNothingBehind(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "a")
	// git sends a tar.gz through tar.tar.gz.command: this one waits first, so there's time to
	// stop it. A small repository zips faster than a test can press Stop.
	fx.Git("config", "tar.tar.gz.command", "sleep 3; cat")
	path := openForMap(t, a, fx)
	dir := t.TempDir()

	type result struct {
		saved SavedArchive
		err   error
	}
	done := make(chan result, 1)
	go func() {
		s, err := a.SaveArchive(path, "HEAD", git.ArchiveOptions{Format: "tar.gz"}, dir, "a.tar.gz", false)
		done <- result{s, err}
	}()
	// Stop once git has started: the file it writes is in the folder under another name.
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if entries, _ := os.ReadDir(dir); len(entries) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("it never started")
		}
	}
	time.Sleep(300 * time.Millisecond)
	a.StopArchive(path)
	got := <-done
	if got.err != nil || !got.saved.Stopped {
		t.Fatalf("stopped: %+v, %v", got.saved, got.err)
	}
	// The compressor holds the file until it ends; on Windows it goes once it has.
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		entries, _ := os.ReadDir(dir)
		if len(entries) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("left behind: %v", entries)
		}
	}
}
