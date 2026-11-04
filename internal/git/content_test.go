package git

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestContentsVersions(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "head\n", "base")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	fx.WriteFile("a.txt", "index\n")
	fx.Git("add", "a.txt")
	fx.WriteFile("a.txt", "worktree\n")

	fc, err := repo.Contents(ctx, "a.txt", FileRef{Rev: "HEAD", Path: "a.txt"}, IndexVersion("a.txt"), false)
	if err != nil || fc.Kind != "text" || fc.Old.Text != "head\n" || fc.New.Text != "index\n" {
		t.Fatalf("HEAD vs index = %+v, %v", fc, err)
	}
	fc, err = repo.Contents(ctx, "a.txt", IndexVersion("a.txt"), WorkTreeVersion("a.txt"), false)
	if err != nil || fc.Old.Text != "index\n" || fc.New.Text != "worktree\n" {
		t.Fatalf("index vs worktree = %+v, %v", fc, err)
	}
	fc, err = repo.Contents(ctx, "new.txt", IndexVersion("new.txt"), WorkTreeVersion("new.txt"), false)
	if err != nil || fc.Old.Exists || fc.New.Exists {
		t.Fatalf("missing file on both sides = %+v, %v", fc, err)
	}
}

func TestContentsBinaryLargeImage(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	fx.WriteFile("bin.dat", "a\x00b")
	if fc, _ := repo.Contents(ctx, "bin.dat", IndexVersion("bin.dat"), WorkTreeVersion("bin.dat"), false); fc.Kind != "binary" {
		t.Fatalf("binary = %+v", fc.Kind)
	}

	fx.WriteFile("big.txt", strings.Repeat("x", MaxTextBytes+1))
	fc, _ := repo.Contents(ctx, "big.txt", IndexVersion("big.txt"), WorkTreeVersion("big.txt"), false)
	if fc.Kind != "large" || fc.New.Text != "" || fc.New.Size != MaxTextBytes+1 {
		t.Fatalf("large = %s size %d", fc.Kind, fc.New.Size)
	}
	if fc, _ := repo.Contents(ctx, "big.txt", IndexVersion("big.txt"), WorkTreeVersion("big.txt"), true); fc.Kind != "text" {
		t.Fatalf("forced large = %s", fc.Kind)
	}

	fx.WriteFile("pic.PNG", "\x89PNG\r\n\x1a\n\x00fake")
	fc, _ = repo.Contents(ctx, "pic.PNG", IndexVersion("pic.PNG"), WorkTreeVersion("pic.PNG"), false)
	if fc.Kind != "image" || fc.Mime != "image/png" || fc.New.Image == "" || fc.Old.Exists {
		t.Fatalf("image = %+v", fc)
	}
}

func TestSetIndexContentStagesPartialChange(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\ntwo\nthree\n", "base")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	fx.WriteFile("a.txt", "ONE\ntwo\nTHREE\n")

	// Stage only the first line's change.
	if err := repo.SetIndexContent(ctx, "a.txt", "ONE\ntwo\nthree\n"); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("show", ":a.txt"); got != "ONE\ntwo\nthree" {
		t.Fatalf("index = %q", got)
	}
	if b, _ := os.ReadFile(fx.Path("a.txt")); string(b) != "ONE\ntwo\nTHREE\n" {
		t.Fatal("work tree must be untouched")
	}
	staged := fx.Git("diff", "--cached")
	unstaged := fx.Git("diff")
	if !strings.Contains(staged, "+ONE") || strings.Contains(staged, "THREE") || !strings.Contains(unstaged, "+THREE") {
		t.Fatalf("staged:\n%s\nunstaged:\n%s", staged, unstaged)
	}
}

func TestSetIndexContentNewFileAndMode(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	fx.WriteFile("new.txt", "a\nb\n")
	if err := repo.SetIndexContent(context.Background(), "new.txt", "a\n"); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("ls-files", "-s", "new.txt"); !strings.HasPrefix(got, "100644 ") {
		t.Fatalf("ls-files = %q", got)
	}
	if got := fx.Git("show", ":new.txt"); got != "a" {
		t.Fatalf("index = %q", got)
	}
}

// With autocrlf the work tree has CRLF but the index stores LF. Reading with filters and
// writing with --path must round-trip so only real edits show as changes.
func TestSetIndexContentRespectsLineEndingFilters(t *testing.T) {
	fx := gittest.New(t)
	fx.Git("config", "core.autocrlf", "true")
	fx.WriteFile("a.txt", "one\r\ntwo\r\n")
	fx.Commit("crlf")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	fc, err := repo.Contents(ctx, "a.txt", IndexVersion("a.txt"), WorkTreeVersion("a.txt"), false)
	if err != nil || fc.Old.Text != fc.New.Text {
		t.Fatalf("an unchanged CRLF file must compare equal: old %q new %q (%v)", fc.Old.Text, fc.New.Text, err)
	}
	fx.WriteFile("a.txt", "ONE\r\ntwo\r\n")
	if err := repo.SetIndexContent(ctx, "a.txt", "ONE\r\ntwo\r\n"); err != nil {
		t.Fatal(err)
	}
	if blob := fx.Git("cat-file", "-p", ":a.txt"); strings.Contains(blob, "\r") {
		t.Fatalf("index should hold LF after clean filter: %q", blob)
	}
	if fx.Git("diff") != "" {
		t.Fatal("work tree should match the index after staging everything")
	}
}

func TestSetWorkTreeContent(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "base")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if err := repo.SetWorkTreeContent("a.txt", "b\n"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(fx.Path("a.txt")); string(b) != "b\n" {
		t.Fatalf("content = %q", b)
	}
	if err := repo.SetWorkTreeContent("../escape.txt", "x"); err == nil {
		t.Fatal("writing outside the repo must fail")
	}
}
