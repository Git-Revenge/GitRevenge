package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestCommitDetailAndDiff(t *testing.T) {
	t.Parallel()
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
	// With no mailmap, who it's shown as is who wrote it.
	if info.Author.Name != "Test User" || info.Author.Email != "test@example.com" || info.Committer != info.Author {
		t.Fatalf("shown as %+v, committed by %+v", info.Author, info.Committer)
	}
	// A saved mailmap changes who it's shown as, and never what the commit says.
	fx.WriteFile(".mailmap", "Jane Doe <jane@example.com> <test@example.com>\n")
	info, err = a.CommitDetail(tab.Path, fx.Head())
	if err != nil {
		t.Fatal(err)
	}
	if info.Author.Name != "Jane Doe" || info.Author.Email != "jane@example.com" || info.Committer.Name != "Jane Doe" {
		t.Fatalf("with a mailmap, shown as %+v, committed by %+v", info.Author, info.Committer)
	}
	if info.Commit.Author.Name != "Test User" || info.Commit.Author.Email != "test@example.com" {
		t.Fatalf("the commit itself now says %+v", info.Commit.Author)
	}
	fx.RemoveFile(".mailmap")
	fc, err := a.FileContents(tab.Path, DiffCommit, fx.Head(), "a.txt", "", false)
	if err != nil || fc.Old.Text != "one\n" || fc.New.Text != "one\ntwo\n" {
		t.Fatalf("contents = %+v, %v", fc, err)
	}
	if _, err := a.CommitDetail(tab.Path, strings.Repeat("0", 40)); err == nil || err.Error() != "That commit no longer exists." {
		t.Fatalf("missing commit err = %v", err)
	}
}
