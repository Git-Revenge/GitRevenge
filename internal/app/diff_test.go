package app

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestFileContentsModes(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "v1\n", "first")
	fx.CommitFile("a.txt", "v2\n", "second")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fx.WriteFile("a.txt", "v3\n")
	fx.Git("add", "a.txt")
	fx.WriteFile("a.txt", "v4\n")

	cases := []struct {
		mode, commit, old, new string
	}{
		{DiffCommit, fx.Head(), "v1\n", "v2\n"},
		{DiffStaged, "", "v2\n", "v3\n"},
		{DiffUnstaged, "", "v3\n", "v4\n"},
	}
	for _, c := range cases {
		fc, err := a.FileContents(tab.Path, c.mode, c.commit, "a.txt", "", false)
		if err != nil || fc.Old.Text != c.old || fc.New.Text != c.new {
			t.Errorf("%s: old %q new %q err %v", c.mode, fc.Old.Text, fc.New.Text, err)
		}
	}
	root := fx.Git("rev-list", "--max-parents=0", "HEAD")
	if fc, _ := a.FileContents(tab.Path, DiffCommit, root, "a.txt", "", false); fc.Old.Exists || fc.New.Text != "v1\n" {
		t.Fatalf("root commit = %+v", fc)
	}
}

func TestPartialStagingGuards(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\ntwo\n", "base")
	tab, _ := a.OpenRepo(fx.Dir)
	fx.WriteFile("a.txt", "ONE\nTWO\n")

	// Stage the first line only, based on what the UI saw.
	if err := a.SetStagedContent(tab.Path, "a.txt", "one\ntwo\n", "ONE\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("show", ":a.txt"); got != "ONE\ntwo" {
		t.Fatalf("index = %q", got)
	}
	// A stale view (index changed since) is refused.
	if err := a.SetStagedContent(tab.Path, "a.txt", "one\ntwo\n", "x\n"); !errors.Is(err, errChanged) {
		t.Fatalf("stale stage err = %v", err)
	}

	// Discard the second line's change in the work tree.
	if err := a.SetWorkTreeContent(tab.Path, "a.txt", "ONE\nTWO\n", "ONE\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	// The file changed on disk since the view: refuse rather than clobber.
	fx.WriteFile("a.txt", "edited elsewhere\n")
	if err := a.SetWorkTreeContent(tab.Path, "a.txt", "ONE\ntwo\n", "x\n"); !errors.Is(err, errChanged) {
		t.Fatalf("stale discard err = %v", err)
	}
	if b, _ := os.ReadFile(fx.Path("a.txt")); string(b) != "edited elsewhere\n" {
		t.Fatal("newer edits must not be overwritten")
	}
}

func TestFileImage(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	// A 1x1 PNG.
	png := "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82"
	fx.WriteFile("docs/img/dot.png", png)
	fx.WriteFile("docs/readme.md", "![dot](img/dot.png)")
	head := fx.Commit("docs with an image")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}

	url, err := a.FileImage(tab.Path, DiffCommit, head, "docs/img/dot.png")
	if err != nil || !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("FileImage(png) = %.40q, %v", url, err)
	}
	for _, p := range []string{"docs/readme.md", "docs/missing.png", "../outside.png", "/etc/passwd"} {
		if url, err := a.FileImage(tab.Path, DiffUnstaged, "", p); err != nil || url != "" {
			t.Errorf("FileImage(%q) = %.40q, %v; want empty", p, url, err)
		}
	}
}

func TestFileData(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	pdf := "%PDF-1.4\n\x00\x01\x02 binary bytes\n%%EOF\n"
	fx.WriteFile("docs/spec.pdf", pdf)
	head := fx.Commit("add a pdf")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.FileData(tab.Path, DiffCommit, head, "docs/spec.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := base64.StdEncoding.DecodeString(got); string(raw) != pdf {
		t.Fatalf("FileData = %q", raw)
	}
	if got, err := a.FileData(tab.Path, DiffUnstaged, "", "docs/missing.pdf"); err != nil || got != "" {
		t.Fatalf("missing file = %q, %v", got, err)
	}
}
