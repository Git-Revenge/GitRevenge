package git

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// archived is a repository whose .gitattributes leave a folder and a file out of archives and
// fill in a placeholder: three files go in, three stay out.
func archived(t *testing.T) (*Repo, *gittest.Repo) {
	t.Helper()
	fx := gittest.New(t)
	fx.WriteFile(".gitattributes", "docs export-ignore\nsecret.txt export-ignore\nVERSION export-subst\n")
	fx.WriteFile("src/main.go", "package main\n")
	fx.WriteFile("docs/guide.md", "# Guide\n")
	fx.WriteFile("docs/api/index.md", "# API\n")
	fx.WriteFile("secret.txt", "not for release\n")
	fx.WriteFile("VERSION", "$Format:%H$\n")
	fx.Commit("everything")
	fx.Git("tag", "v1.0")
	return OpenRepo(newTestRunner(t), fx.Dir), fx
}

func TestArchivePlanSaysWhatGoesIn(t *testing.T) {
	t.Parallel()
	repo, fx := archived(t)
	ctx := context.Background()
	plan, err := repo.ArchivePlan(ctx, "v1.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Commit != fx.Head() || plan.Files != 3 || plan.Ignored != 3 || plan.Subst != 1 || len(plan.Missing) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	want := int64(len("docs export-ignore\nsecret.txt export-ignore\nVERSION export-subst\n") + len("package main\n") + len("$Format:%H$\n"))
	if plan.Bytes != want {
		t.Errorf("bytes = %d, want %d", plan.Bytes, want)
	}

	// One folder: only what's under it, export-ignore still applying.
	if plan, err = repo.ArchivePlan(ctx, "v1.0", `src\`); err != nil || plan.Files != 1 || plan.Ignored != 0 {
		t.Errorf("src: %+v, %v", plan, err)
	}
	if plan, err = repo.ArchivePlan(ctx, "v1.0", "docs"); err != nil || plan.Files != 0 || plan.Ignored != 2 {
		t.Errorf("an ignored folder: %+v, %v", plan, err)
	}
	if _, err = repo.ArchivePlan(ctx, "v1.0", "VERSION"); !errors.Is(err, ErrNotAFolder) {
		t.Errorf("a file as the folder: %v", err)
	}
	if _, err = repo.ArchivePlan(ctx, "v1.0", "../elsewhere"); err == nil {
		t.Error("a folder outside the repository was taken")
	}
}

func TestArchiveWritesAZipAsGitDoes(t *testing.T) {
	t.Parallel()
	repo, fx := archived(t)
	file := filepath.Join(t.TempDir(), "out.zip")
	if err := repo.Archive(context.Background(), "v1.0", ArchiveOptions{Format: "zip", Prefix: "proj-v1.0"}, file); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = z.Close() }()
	var names []string
	version := ""
	for _, f := range z.File {
		names = append(names, f.Name)
		if f.Name == "proj-v1.0/VERSION" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			version = strings.TrimSpace(string(b))
		}
	}
	// The prefix became a folder, the ignored files stayed out, and the placeholder was filled.
	for _, want := range []string{"proj-v1.0/.gitattributes", "proj-v1.0/src/main.go", "proj-v1.0/VERSION"} {
		if !slices.Contains(names, want) {
			t.Errorf("%s missing from %v", want, names)
		}
	}
	for _, name := range names {
		if strings.Contains(name, "docs/") || strings.Contains(name, "secret") {
			t.Errorf("%s should have been left out", name)
		}
	}
	if version != fx.Head() {
		t.Errorf("VERSION = %q, want the commit id", version)
	}
}

func TestArchiveWritesOneFolderAsATarGz(t *testing.T) {
	t.Parallel()
	repo, _ := archived(t)
	file := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := repo.Archive(context.Background(), "v1.0", ArchiveOptions{Format: "tar.gz", Prefix: "p/", Sub: "/src/"}, file); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var files []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			files = append(files, h.Name)
		}
	}
	if !slices.Equal(files, []string{"p/src/main.go"}) {
		t.Errorf("files = %v", files)
	}
	if err := repo.Archive(context.Background(), "v1.0", ArchiveOptions{Format: "rar"}, file); err == nil {
		t.Error("a format git doesn't write was taken")
	}
}

func TestArchivePlanInAPartialCloneDownloadsNothing(t *testing.T) {
	t.Parallel()
	repo := OpenRepo(newTestRunner(t), partialClone(t))
	ctx := context.Background()
	plan, err := repo.ArchivePlan(ctx, "HEAD~1", "")
	if err != nil {
		t.Fatal(err)
	}
	// old.txt as it was then was never fetched: its size isn't known, and it's to download.
	if plan.Files != 1 || plan.Bytes != -1 || len(plan.Missing) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if !stillMissing(t, repo) {
		t.Error("planning downloaded the file")
	}
}

func TestCleanSubAndFolderPrefix(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"": "", ".": "", "src": "src", `src\app\`: "src/app", "/a/b/": "a/b"} {
		if got, err := CleanSub(in); err != nil || got != want {
			t.Errorf("CleanSub(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"..", "a/../b", "a//b"} {
		if _, err := CleanSub(bad); err == nil {
			t.Errorf("CleanSub(%q) was taken", bad)
		}
	}
	for in, want := range map[string]string{"": "", "proj": "proj/", "proj/": "proj/", `a\b`: "a/b/", "/x": "x/"} {
		if got := FolderPrefix(in); got != want {
			t.Errorf("FolderPrefix(%q) = %q", in, got)
		}
	}
}
