package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestCommitDetailAndDiff(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "first")
	fx.CommitFile("a.txt", "one\ntwo\n", "second\n\nbody")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}

	info, err := a.CommitDetail(tab.Path, fx.Head())
	if err != nil {
		t.Fatal(err)
	}
	if info.Commit.Subject != "second" || info.Commit.Body != "body" || len(info.Files) != 1 || info.Files[0].Path != "a.txt" {
		t.Fatalf("info = %+v", info)
	}
	fc, err := a.FileContents(tab.Path, DiffCommit, fx.Head(), "a.txt", "", false)
	if err != nil || fc.Old.Text != "one\n" || fc.New.Text != "one\ntwo\n" {
		t.Fatalf("contents = %+v, %v", fc, err)
	}
	if _, err := a.CommitDetail(tab.Path, strings.Repeat("0", 40)); err == nil || err.Error() != "That commit no longer exists." {
		t.Fatalf("missing commit err = %v", err)
	}
}
