package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestSavePatchesAsksBeforeReplacing(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "base")
	fx.CommitFile("a.txt", "2\n", "Give the feed more time")
	tip := fx.CommitFile("b.txt", "b\n", "Add the Breakwater station")
	path := openForMap(t, a, fx)
	folder := t.TempDir()
	revs := git.PatchRevs{Tip: tip, Last: 2}
	opts := git.PatchOptions{Numbered: true}

	saved, err := a.SavePatches(path, revs, opts, folder, "", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0001-Give-the-feed-more-time.patch", "0002-Add-the-Breakwater-station.patch"}
	if !slices.Equal(saved.Files, want) || len(saved.Exists) != 0 {
		t.Fatalf("saved = %+v", saved)
	}
	before, _ := os.ReadFile(filepath.Join(folder, want[0]))
	if err := os.WriteFile(filepath.Join(folder, want[0]), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Saved again: one is there, so nothing is written, and it says which.
	again, err := a.SavePatches(path, revs, opts, folder, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(again.Exists, want) || len(again.Files) != 0 {
		t.Fatalf("again = %+v, want both named as there", again)
	}
	if got, _ := os.ReadFile(filepath.Join(folder, want[0])); string(got) != "mine" {
		t.Fatal("a file that was there was replaced without asking")
	}
	entries, _ := os.ReadDir(folder)
	if len(entries) != 2 {
		t.Fatalf("the folder holds %d entries, want the 2 patches and nothing left over", len(entries))
	}
	// Asked, they're replaced.
	if _, err := a.SavePatches(path, revs, opts, folder, "", true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(folder, want[0])); string(got) != string(before) {
		t.Fatal("replacing didn't write the patch")
	}
}

func TestSavePatchesAsOneMailbox(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	base := fx.CommitFile("a.txt", "1\n", "base")
	fx.CommitFile("a.txt", "2\n", "two")
	tip := fx.CommitFile("a.txt", "3\n", "three")
	path := openForMap(t, a, fx)
	folder := t.TempDir()
	revs := git.PatchRevs{Since: base, Tip: tip}

	saved, err := a.SavePatches(path, revs, git.PatchOptions{Numbered: true}, folder, "main.mbox", false)
	if err != nil || !slices.Equal(saved.Files, []string{"main.mbox"}) {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
	text, _ := os.ReadFile(filepath.Join(folder, "main.mbox"))
	if strings.Count(string(text), "\nSubject: [PATCH ") != 2 {
		t.Fatalf("the mailbox:\n%s", text)
	}
	if again, err := a.SavePatches(path, revs, git.PatchOptions{}, folder, "main.mbox", false); err != nil || !slices.Equal(again.Exists, []string{"main.mbox"}) {
		t.Fatalf("again = %+v, %v", again, err)
	}
	if _, err := a.SavePatches(path, revs, git.PatchOptions{}, folder, "../main.mbox", false); err == nil {
		t.Fatal("a mailbox named with a folder should be refused")
	}
	if _, err := a.SavePatches(path, revs, git.PatchOptions{}, filepath.Join(folder, "nowhere"), "", false); err == nil {
		t.Fatal("a folder that isn't there should be refused")
	}
	copied, err := a.CopyPatches(path, git.PatchRevs{Tip: tip, Last: 1}, git.PatchOptions{Numbered: true})
	if err != nil || !strings.Contains(copied, "Subject: [PATCH] three") {
		t.Fatalf("copied = %q, %v", copied, err)
	}
}

func TestPatchSeriesBaseAndShallowEdge(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	src := gittest.New(t)
	src.CommitFile("a.txt", "1\n", "one")
	two := src.CommitFile("a.txt", "2\n", "two")
	three := src.CommitFile("a.txt", "3\n", "three")
	path := openForMap(t, a, src)

	series, err := a.PatchSeries(path, git.PatchRevs{Tip: three, Last: 1})
	if err != nil {
		t.Fatal(err)
	}
	if series.Count != 1 || series.Base != two || series.Edge || series.Merges != 0 {
		t.Fatalf("series = %+v", series)
	}

	// A shallow clone's oldest commit has no parent here: its patch would add every file.
	dest := filepath.Join(t.TempDir(), "shallow")
	src.Git("clone", "-q", "--depth", "2", "file://"+filepath.ToSlash(src.Dir), dest)
	tab, err := a.OpenRepo(dest)
	if err != nil {
		t.Fatal(err)
	}
	edge, err := a.PatchSeries(tab.Path, git.PatchRevs{Tip: three, Last: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !edge.Edge || edge.Base != "" {
		t.Fatalf("a series reaching the edge = %+v", edge)
	}
}

func TestBranchPatchBase(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	local, _ := gittest.WithRemote(t, 2)
	local.CommitFile("a.txt", "1\n", "unpushed")
	path := openForMap(t, a, local)

	if got, err := a.BranchPatchBase(path, "main", ""); err != nil || got != "origin/main" {
		t.Fatalf("main's base = %q, %v; want its remote's default branch", got, err)
	}
	if got, _ := a.BranchPatchBase(path, "main", "parent"); got != "parent" {
		t.Fatalf("a stacked branch's base = %q, want its parent", got)
	}
}

func TestPatchSeriesInAPartialCloneSaysWhatsMissing(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	path, head := partialTab(t, a)
	series, err := a.PatchSeries(path, git.PatchRevs{Tip: head, Last: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Missing) == 0 {
		t.Fatalf("series = %+v; want the versions the patch reads named as missing", series)
	}
	if err := a.DownloadFiles(path, "p1", series.Missing); err != nil {
		t.Fatal(err)
	}
	copied, err := a.CopyPatches(path, git.PatchRevs{Tip: head, Last: 1}, git.PatchOptions{Numbered: true})
	if err != nil || !strings.Contains(copied, "-one\n+two") {
		t.Fatalf("after the download, the patch = %q, %v", copied, err)
	}
}
