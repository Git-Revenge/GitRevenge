package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// link makes a symbolic link, or skips the test where the system won't make one (Windows
// without Developer Mode).
func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Skipf("can't make a symbolic link here: %v", err)
	}
}

// outsideFile is a file next to the repository, not in it.
func outsideFile(t *testing.T, content string) (dir, file string) {
	t.Helper()
	dir = t.TempDir()
	file = filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, file
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A file in the work tree that is a link to a file outside it shows as where it points,
// which is what git stores for it, and is never written through.
func TestLinkedFileIsNotFollowedOutOfTheRepository(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "base")
	_, secret := outsideFile(t, "outside\n")
	link(t, secret, fx.Path("notes.txt"))
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	fc, err := repo.Contents(ctx, "notes.txt", FileRef{}, WorkTreeVersion("notes.txt"), false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fc.New.Text, "outside") || fc.New.Text != filepath.ToSlash(secret) {
		t.Errorf("shown as %q, want the link's target %q", fc.New.Text, filepath.ToSlash(secret))
	}
	if data, _, err := repo.FileBytes(ctx, WorkTreeVersion("notes.txt")); err != nil || strings.Contains(string(data), "outside") {
		t.Errorf("FileBytes = %q, %v; want the link's target", data, err)
	}

	if err := repo.SetWorkTreeContent("notes.txt", "overwritten\n"); !errors.Is(err, ErrSymlink) {
		t.Errorf("SetWorkTreeContent err = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, secret); got != "outside\n" {
		t.Errorf("the file outside the repository now holds %q", got)
	}

	// Staging it stages a link, not a copy of the file it points at.
	if err := repo.SetIndexContent(ctx, "notes.txt", fc.New.Text); err != nil {
		t.Fatal(err)
	}
	if staged := fx.Git("ls-files", "-s", "--", "notes.txt"); !strings.HasPrefix(staged, "120000 ") {
		t.Errorf("staged as %q, want a link (mode 120000)", staged)
	}
}

// A folder in the work tree that is a link to a folder outside it: nothing under it is read,
// written or deleted.
func TestLinkedFolderLeadingOutIsRefused(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "base")
	outside, secret := outsideFile(t, "outside\n")
	link(t, outside, fx.Path("vendor"))
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if _, err := repo.Contents(ctx, "vendor/secret.txt", FileRef{}, WorkTreeVersion("vendor/secret.txt"), false); err == nil {
		t.Error("read a file through a folder link that leads out of the repository")
	}
	if err := repo.SetWorkTreeContent("vendor/secret.txt", "overwritten\n"); err == nil {
		t.Error("wrote a file through a folder link that leads out of the repository")
	}
	if err := repo.SetWorkTreeContent("vendor/new.txt", "new\n"); err == nil {
		t.Error("made a file through a folder link that leads out of the repository")
	}
	if err := repo.removeUntracked("vendor/secret.txt"); err == nil {
		t.Error("deleted a file through a folder link that leads out of the repository")
	}
	if got := mustRead(t, secret); got != "outside\n" {
		t.Errorf("the file outside the repository now holds %q", got)
	}
	if _, err := os.Stat(filepath.Join(outside, "new.txt")); err == nil {
		t.Error("a file was made outside the repository")
	}

	// Deleting the link itself is fine: it's in the repository, and only the link goes.
	if err := repo.removeUntracked("vendor"); err != nil {
		t.Errorf("removing the link itself: %v", err)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Errorf("removing the link took what it pointed at: %v", err)
	}
}

// A folder link that stays inside the repository works as any folder does.
func TestLinkedFolderInsideTheRepositoryWorks(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("real/a.txt", "a\n", "base")
	link(t, fx.Path("real"), fx.Path("alias"))
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	if err := repo.SetWorkTreeContent("alias/a.txt", "b\n"); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, fx.Path("real/a.txt")); got != "b\n" {
		t.Errorf("content = %q", got)
	}
}

// git doesn't read a .gitignore that is a link, and a rule added to one would be written
// to the file it points at.
func TestLinkedGitignoreIsNotWrittenThrough(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "base")
	_, secret := outsideFile(t, "outside\n")
	link(t, secret, fx.Path(".gitignore"))
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if _, err := repo.AddIgnoreRule(ctx, IgnoreTarget{Scope: IgnoreRoot}, "*.log"); !errors.Is(err, ErrSymlink) {
		t.Errorf("AddIgnoreRule err = %v, want ErrSymlink", err)
	}
	if _, _, err := repo.sourceFile(ctx, ".gitignore"); !errors.Is(err, ErrSymlink) {
		t.Errorf("sourceFile err = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, secret); got != "outside\n" {
		t.Errorf("the file outside the repository now holds %q", got)
	}
}
