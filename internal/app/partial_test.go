package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// partialTab opens a partial clone of a two-commit repository as a tab, and returns its path
// and the commit that changed notes.txt: the version before it was never downloaded.
func partialTab(t *testing.T, a *App) (path, head string) {
	t.Helper()
	src := gittest.New(t)
	src.CommitFile("notes.txt", "one\n", "one")
	src.CommitFile("notes.txt", "two\n", "two")
	src.Git("config", "uploadpack.allowFilter", "true")
	dest := filepath.Join(t.TempDir(), "partial")
	src.Git("clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(src.Dir), dest)
	tab, err := a.OpenRepo(dest)
	if err != nil {
		t.Fatal(err)
	}
	return tab.Path, src.Head()
}

func TestFileContentsInAPartialCloneSaysWhatsMissingThenDownloads(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	path, head := partialTab(t, a)

	fc, err := a.FileContents(path, DiffCommit, head, "notes.txt", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if fc.Kind != "missing" || len(fc.Missing) != 1 {
		t.Fatalf("kind %q, missing %v; want the first version missing", fc.Kind, fc.Missing)
	}
	if err := a.DownloadFiles(path, "t1", fc.Missing); err != nil {
		t.Fatal(err)
	}
	fc, err = a.FileContents(path, DiffCommit, head, "notes.txt", "", false)
	if err != nil || fc.Kind != "text" || fc.Old.Text != "one\n" || fc.New.Text != "two\n" {
		t.Fatalf("after the download: %+v, %v", fc, err)
	}
}

func TestBlameInAPartialCloneAsksFirst(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	path, _ := partialTab(t, a)

	needs, err := a.BlameNeeds(path, "notes.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	if !needs.Partial || needs.Versions != 2 || len(needs.Missing) != 1 {
		t.Fatalf("needs = %+v", needs)
	}
	// Without asking, blame downloads nothing and says why.
	_, err = a.Blame(path, "notes.txt", "")
	if ue := userError(""); !errors.As(err, &ue) || !strings.Contains(err.Error(), "never downloaded") {
		t.Fatalf("blame = %v", err)
	}
	if err := a.DownloadFiles(path, "t2", needs.Missing); err != nil {
		t.Fatal(err)
	}
	if needs, _ := a.BlameNeeds(path, "notes.txt", ""); len(needs.Missing) != 0 {
		t.Fatalf("after the download: %+v", needs)
	}
	lines, err := a.Blame(path, "notes.txt", "")
	if err != nil || len(lines) != 1 {
		t.Fatalf("blame after the download = %v, %v", lines, err)
	}
}

func TestBlameNeedsNothingInAWholeClone(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	needs, err := a.BlameNeeds(tab.Path, "a.txt", "")
	if err != nil || needs.Partial || len(needs.Missing) != 0 {
		t.Fatalf("needs = %+v, %v", needs, err)
	}
}

func TestDownloadFilesTakesOnlyIDs(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	path, _ := partialTab(t, a)
	for _, bad := range [][]string{{"--upload-pack=touch x"}, {"HEAD"}} {
		if err := a.DownloadFiles(path, "t3", bad); err == nil {
			t.Errorf("%q was downloaded", bad)
		}
	}
	if err := a.DownloadFiles(path, "", nil); err == nil {
		t.Error("a download with no name ran")
	}
}
