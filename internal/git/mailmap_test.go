package git

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// gittest commits as "Test User <test@example.com>".
const mapTestUser = "Jane Doe <jane@example.com> <test@example.com>\n"

func TestLogReadsNamesThroughTheMailmap(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(2)

	got := readLog(t, fx, LogOptions{})
	if c := got[0]; c.AuthorName != "Test User" || c.WrittenName != "Test User" || c.AuthorEmail != c.WrittenEmail {
		t.Fatalf("with no mailmap the names should be the commit's own: %+v", c)
	}

	// Saved, not committed: git reads the work tree's file, and so does the graph.
	fx.WriteFile(".mailmap", mapTestUser)
	got = readLog(t, fx, LogOptions{})
	for _, c := range got {
		if c.AuthorName != "Jane Doe" || c.AuthorEmail != "jane@example.com" {
			t.Errorf("%s reads as %s <%s>, want Jane Doe <jane@example.com>", c.Subject, c.AuthorName, c.AuthorEmail)
		}
		if c.WrittenName != "Test User" || c.WrittenEmail != "test@example.com" {
			t.Errorf("%s was written by %s <%s>, want Test User <test@example.com>", c.Subject, c.WrittenName, c.WrittenEmail)
		}
	}
}

func TestListsOfCommitsReadNamesThroughTheMailmap(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "first")
	fx.Git("checkout", "-q", "-b", "topic")
	fx.CommitFile("a.txt", "one\ntwo\n", "second")
	fx.WriteFile(".mailmap", mapTestUser)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	history, _, err := r.FileHistory(ctx, "a.txt")
	if err != nil || len(history) != 2 {
		t.Fatalf("history = %+v, %v", history, err)
	}
	if history[0].Author != "Jane Doe" {
		t.Errorf("file history shows %q, want Jane Doe", history[0].Author)
	}
	between, _, err := r.CommitsBetween(ctx, "main", "topic", 10)
	if err != nil || len(between) != 1 {
		t.Fatalf("between = %+v, %v", between, err)
	}
	if between[0].Author != "Jane Doe" {
		t.Errorf("a list of commits shows %q, want Jane Doe", between[0].Author)
	}
}

func TestMailmapReadsTheWorkTreesFile(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	in := []Identity{{Name: "Test User", Email: "test@example.com"}, {Name: "No Email"}, {Name: "Sam", Email: "sam@example.com"}}

	got, err := r.Mailmap(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("with no mailmap, identity %d = %+v, want it unchanged", i, got[i])
		}
	}

	fx.WriteFile(".mailmap", mapTestUser)
	got, err = r.Mailmap(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	want := []Identity{{Name: "Jane Doe", Email: "jane@example.com"}, {Name: "No Email"}, {Name: "Sam", Email: "sam@example.com"}}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("identity %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestMailmapStateFollowsTheFilesGitReads(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	none := r.MailmapState(ctx)
	if none.FilesMoved() || !none.Same(r.MailmapState(ctx)) {
		t.Fatal("a repository nothing happened to reads as changed")
	}

	fx.WriteFile(".mailmap", mapTestUser)
	if !none.FilesMoved() {
		t.Fatal("a new .mailmap wasn't noticed")
	}
	saved := r.MailmapState(ctx)
	if saved.Same(none) || saved.FilesMoved() {
		t.Fatalf("after a .mailmap was saved: same %v, moved %v", saved.Same(none), saved.FilesMoved())
	}

	fx.RemoveFile(".mailmap")
	if !saved.FilesMoved() {
		t.Fatal("a deleted .mailmap wasn't noticed")
	}

	// mailmap.file is read from the top of the work tree, and its file counts too.
	fx.Git("config", "mailmap.file", "people.txt")
	withFile := r.MailmapState(ctx)
	if withFile.Same(r.MailmapState(context.Background())) == false || withFile.Same(none) {
		t.Fatalf("setting mailmap.file should change the state once: %+v", withFile)
	}
	if want := filepath.Join(fx.Dir, "people.txt"); len(withFile.Files) != 2 || withFile.Files[1] != want {
		t.Fatalf("files = %v, want the .mailmap and %s", withFile.Files, want)
	}
	fx.WriteFile("people.txt", mapTestUser)
	if !withFile.FilesMoved() {
		t.Fatal("a change to the file mailmap.file names wasn't noticed")
	}
}
