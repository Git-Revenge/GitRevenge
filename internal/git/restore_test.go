package git

import (
	"context"
	"errors"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// #1082: a version's text in work-tree form, under today's name, and written back.
func TestVersionTextAndWorkFile(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	first := fx.CommitFile("lib/sms.js", "one\ntwo\n", "first")
	fx.Git("mv", "lib/sms.js", "lib/texts.js")
	fx.Commit("rename")
	fx.WriteFile(".gitattributes", "*.js text eol=crlf\n")
	fx.Commit("crlf")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	// From before the rename, read under its old name, and put through today's attributes.
	got, err := repo.VersionText(ctx, first, "lib/sms.js", "lib/texts.js")
	if err != nil || string(got) != "one\r\ntwo\r\n" {
		t.Fatalf("version text = %q, %v", got, err)
	}
	if err := repo.WriteWorkFile(ctx, "lib/texts.js", got, true); err != nil {
		t.Fatal(err)
	}
	if data, exists, err := repo.WorkFile("lib/texts.js"); err != nil || !exists || string(data) != "one\r\ntwo\r\n" {
		t.Errorf("written = %q, %v, %v", data, exists, err)
	}
	if err := repo.WriteWorkFile(ctx, "lib/texts.js", nil, false); err != nil {
		t.Fatal(err)
	}
	if _, exists, _ := repo.WorkFile("lib/texts.js"); exists {
		t.Error("the file wasn't removed")
	}
}

func TestLinesRestored(t *testing.T) {
	t.Parallel()
	was := []byte("a\nb\nold one\nold two\nc\n")
	// Lines went in above, so the two lines moved down; they're still as they were.
	work := []byte("new\na\nb\nold one\nold two\nc\n")
	got, err := LinesRestored(work, was, 3, 4, []string{"back"})
	if err != nil || string(got) != "new\na\nb\nback\nc\n" {
		t.Errorf("moved down: %q, %v", got, err)
	}
	// CRLF in the work tree: the new lines end the same way.
	got, err = LinesRestored([]byte("a\r\nb\r\nold one\r\nold two\r\nc\r\n"), was, 3, 4, []string{"x", "y"})
	if err != nil || string(got) != "a\r\nb\r\nx\r\ny\r\nc\r\n" {
		t.Errorf("crlf: %q, %v", got, err)
	}
	// The file's last line, with no newline after it.
	got, err = LinesRestored([]byte("a\nend"), []byte("a\nend"), 2, 2, []string{"END"})
	if err != nil || string(got) != "a\nEND" {
		t.Errorf("last line: %q, %v", got, err)
	}
	// They were changed since: nothing to put back over.
	if _, err := LinesRestored([]byte("a\nb\nchanged\nold two\nc\n"), was, 3, 4, []string{"back"}); !errors.Is(err, ErrLinesMoved) {
		t.Errorf("changed since: %v", err)
	}
	if _, err := LinesRestored(work, was, 4, 9, []string{"x"}); err == nil || errors.Is(err, ErrLinesMoved) {
		t.Errorf("a range past the end: %v", err)
	}
}
