package git

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestReadCommitParsesHeaderAndMessage(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	parent := fx.Linear(1)[0]
	fx.CommitFile("a.txt", "a", "Add a\n\nLonger explanation\nover two lines.")
	o := NewObjectReader(newTestRunner(t), fx.Dir)
	defer func() { _ = o.Close() }()

	c, err := ReadCommit(o, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != fx.Head() || !slices.Equal(c.Parents, []string{parent}) {
		t.Fatalf("ids: %+v", c)
	}
	if c.Subject != "Add a" || c.Body != "Longer explanation\nover two lines." {
		t.Fatalf("message: %q / %q", c.Subject, c.Body)
	}
	if c.Author.Name != "Test User" || c.Author.Email != "test@example.com" || c.Author.Time == 0 || c.Author.TZ == "" {
		t.Fatalf("author: %+v", c.Author)
	}
	if c.Signed {
		t.Fatal("unsigned commit reported as signed")
	}
}

func TestReadCommitRejectsNonCommit(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a", "add")
	o := NewObjectReader(newTestRunner(t), fx.Dir)
	defer func() { _ = o.Close() }()
	if _, err := ReadCommit(o, "HEAD:a.txt"); err == nil {
		t.Fatal("blob accepted as commit")
	}
}

func TestParseSignatureOddNames(t *testing.T) {
	t.Parallel()
	s := parseSignature("Jane <Doe> Smith <jane@example.com> 1700000000 -0500")
	if s.Name != "Jane <Doe> Smith" || s.Email != "jane@example.com" || s.Time != 1700000000 || s.TZ != "-0500" {
		t.Fatalf("%+v", s)
	}
}

func changed(t *testing.T, fx *gittest.Repo, rev string) []FileChange {
	t.Helper()
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	o := NewObjectReader(newTestRunner(t), fx.Dir)
	defer func() { _ = o.Close() }()
	c, err := ReadCommit(o, rev)
	if err != nil {
		t.Fatal(err)
	}
	files, err := repo.ChangedFiles(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestChangedFiles(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.WriteFile("keep.txt", "keep")
	fx.WriteFile("old name.txt", strings.Repeat("rename me\n", 20))
	fx.WriteFile("gone.txt", "bye")
	fx.Commit("root")
	if files := changed(t, fx, "HEAD"); len(files) != 3 || files[0].Status != "A" {
		t.Fatalf("root commit files = %+v", files)
	}

	fx.WriteFile("keep.txt", "changed")
	fx.Git("mv", "old name.txt", "new name.txt")
	fx.RemoveFile("gone.txt")
	fx.WriteFile("dir/added.txt", "new")
	fx.Commit("mixed")

	got := map[string]FileChange{}
	for _, f := range changed(t, fx, "HEAD") {
		got[f.Path] = f
	}
	if got["keep.txt"].Status != "M" || got["gone.txt"].Status != "D" || got["dir/added.txt"].Status != "A" {
		t.Fatalf("statuses = %+v", got)
	}
	if r := got["new name.txt"]; r.Status != "R" || r.OldPath != "old name.txt" {
		t.Fatalf("rename = %+v", r)
	}
}
