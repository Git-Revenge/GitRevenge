package app

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/semdiff"
)

func TestFileContentsModes(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestFileDiffComparesTheVersions(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "keep\nold line\nkeep\n", "first")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fx.WriteFile("a.txt", "keep\nnew line\nkeep\n")
	fd, err := a.FileDiff(tab.Path, DiffUnstaged, "", "a.txt", "", false, ReadCode)
	if err != nil {
		t.Fatal(err)
	}
	if fd.Contents.New.Text != "keep\nnew line\nkeep\n" || fd.Diff == nil {
		t.Fatalf("got %+v", fd)
	}
	if len(fd.Diff.Blocks) != 1 || fd.Diff.Blocks[0].OldStart != 1 || fd.Diff.Blocks[0].NewEnd != 2 || fd.Diff.Algorithm != "myers" {
		t.Errorf("diff %+v, want line 2 changed, by myers", fd.Diff)
	}
	if want := 0; len(fd.Diff.Blocks[0].New) == 0 || fd.Diff.Blocks[0].New[0].From != want {
		t.Errorf("marks %+v, want the first word of line 2", fd.Diff.Blocks[0].New)
	}
}

func TestFileDiffFollowsGitsAlgorithmUnlessOneIsChosen(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\nb\n", "first")
	fx.Git("config", "diff.algorithm", "Histogram")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fx.WriteFile("a.txt", "a\nc\n")
	if got, _ := a.GitDiffAlgorithm(tab.Path); got != "histogram" {
		t.Errorf("git's algorithm = %q, want histogram", got)
	}
	fd, err := a.FileDiff(tab.Path, DiffUnstaged, "", "a.txt", "", false, ReadCode)
	if err != nil || fd.Diff.Algorithm != "histogram" {
		t.Fatalf("algorithm %v, %v; want histogram, from git", fd.Diff, err)
	}

	p, _ := a.Preferences()
	p.DiffAlgorithm, p.DiffMoves = "patience", false
	if err := a.SetPreferences(p); err != nil {
		t.Fatal(err)
	}
	if fd, _ = a.FileDiff(tab.Path, DiffUnstaged, "", "a.txt", "", false, ReadCode); fd.Diff.Algorithm != "patience" {
		t.Errorf("algorithm %s, want patience, as chosen", fd.Diff.Algorithm)
	}
	if p, _ = a.Preferences(); p.DiffAlgorithm != "patience" || p.DiffMoves || !p.DiffRibbon {
		t.Errorf("preferences %+v, want patience, no moves, the ribbon", p)
	}
	p.DiffAlgorithm = "quantum"
	if err := a.SetPreferences(p); err == nil {
		t.Error("took an unknown line diff")
	}
}

func TestFileDiffLeavesBinaryFilesUncompared(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("b.bin", "a\x00b", "first")
	tab, _ := a.OpenRepo(fx.Dir)
	fx.WriteFile("b.bin", "a\x00c")
	fd, err := a.FileDiff(tab.Path, DiffUnstaged, "", "b.bin", "", false, ReadCode)
	if err != nil || fd.Contents.Kind != "binary" || fd.Diff != nil {
		t.Errorf("got %+v, %v; want a binary file with no diff", fd, err)
	}
}

func TestFileDiffReadsCodeInTheFilesLanguage(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	src := "package p\n\nconst maxHours = 0x30\n"
	fx.CommitFile("a.go", src, "first")
	fx.CommitFile("b.inc", src, "an include")
	fx.CommitFile(".gitattributes", "*.inc linguist-language=Go\n", "attributes")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	after := strings.Replace(src, "0x30", "48", 1)
	fx.WriteFile("a.go", after)
	fx.WriteFile("b.inc", after)
	for _, file := range []string{"a.go", "b.inc"} {
		// The patient read: on a busy machine even a small file can miss the quick one.
		fd, err := a.FileDiff(tab.Path, DiffUnstaged, "", file, "", false, ReadPatient)
		if err != nil {
			t.Fatal(err)
		}
		if fd.Diff.Engine != "code" || fd.Diff.Language != "Go" || len(fd.Diff.Blocks) != 1 || fd.Diff.Blocks[0].Kind != "format" {
			t.Errorf("%s: %+v, want it read as Go, formatting only", file, fd.Diff)
		}
		// Lines still say what the file is, so the view can offer reading it as code.
		fd, err = a.FileDiff(tab.Path, DiffUnstaged, "", file, "", false, ReadLines)
		if err != nil {
			t.Fatal(err)
		}
		if fd.Diff.Engine != "lines" || fd.Diff.Language != "Go" || fd.Diff.Blocks[0].Kind != "" {
			t.Errorf("%s by lines: %+v", file, fd.Diff)
		}
	}
	if _, err := a.FileDiff(tab.Path, DiffUnstaged, "", "a.go", "", false, "sideways"); err == nil {
		t.Error("took an unknown way to read")
	}

	p, _ := a.Preferences()
	if !p.DiffCode || p.DiffHideFormatting || len(p.Languages) != len(semdiff.Languages()) {
		t.Errorf("preferences %+v, want code on, formatting shown, every language", p)
	}
	p.DiffCode, p.DiffHideFormatting = false, true
	if err := a.SetPreferences(p); err != nil {
		t.Fatal(err)
	}
	if p, _ = a.Preferences(); p.DiffCode || !p.DiffHideFormatting {
		t.Errorf("preferences %+v, want lines only, formatting hidden", p)
	}
}

func TestFormattingOnlyMarksFilesWithNothingElse(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("fmt.go", "package p\n\nconst a = 0x30\n", "first")
	fx.CommitFile("real.go", "package p\n\nconst a = 1\n", "second")
	fx.CommitFile("notes.txt", "a\n", "third")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fx.WriteFile("fmt.go", "package p\n\nconst a = 48\n")
	fx.WriteFile("real.go", "package p\n\nconst a = 2\n")
	fx.WriteFile("notes.txt", "b\n")
	got, err := a.FormattingOnly(tab.Path, DiffUnstaged, "", []string{"fmt.go", "real.go", "notes.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "fmt.go" {
		t.Errorf("formatting only: %q, want fmt.go", got)
	}
	fx.Git("add", "-A")
	fx.Git("commit", "-qm", "edits")
	head := strings.TrimSpace(fx.Git("rev-parse", "HEAD"))
	if got, _ = a.FormattingOnly(tab.Path, DiffCommit, head, []string{"fmt.go", "real.go"}); strings.Join(got, ",") != "fmt.go" {
		t.Errorf("in the commit: %q, want fmt.go", got)
	}
}

func TestFileDiffReadsAScriptByItsFirstLine(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("deploy", "#!/bin/sh\necho 'a b'\n", "first")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fx.WriteFile("deploy", "#!/bin/sh\necho \"a b\"\n")
	fd, err := a.FileDiff(tab.Path, DiffUnstaged, "", "deploy", "", false, ReadPatient)
	if err != nil {
		t.Fatal(err)
	}
	if fd.Diff.Engine != "code" || fd.Diff.Language != "Shell" || len(fd.Diff.Blocks) != 1 || fd.Diff.Blocks[0].Kind != "format" {
		t.Errorf("%+v, want it read as Shell, formatting only", fd.Diff)
	}
}
