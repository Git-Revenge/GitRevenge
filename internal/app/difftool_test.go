package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestOpenDiffTool(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "one")
	fx.WriteFile("a.txt", "two\n")
	path := openForMap(t, a, fx)
	unstaged := git.DiffToolTarget{Mode: "unstaged", File: "a.txt"}

	if tool, err := a.DiffTool(path); err != nil || tool.Name != "" {
		t.Fatalf("none set: %+v, %v", tool, err)
	}
	if err := a.OpenDiffTool(path, unstaged); err == nil || !strings.Contains(err.Error(), "No diff tool is set") {
		t.Fatalf("none set: %v", err)
	}

	out := filepath.ToSlash(t.TempDir())
	fx.Git("config", "diff.tool", "fake")
	fx.Git("config", "difftool.fake.cmd", `cat "$REMOTE" > "`+out+`/remote"`)
	if tool, err := a.DiffTool(path); err != nil || tool != (git.DiffTool{Name: "fake", Key: "diff.tool"}) {
		t.Fatalf("set: %+v, %v", tool, err)
	}
	if err := a.OpenDiffTool(path, unstaged); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "remote")); string(b) != "two\n" {
		t.Errorf("the tool got %q", b)
	}

	// A tool git doesn't know says so, in its words.
	fx.Git("config", "diff.tool", "nosuchtool")
	err := a.OpenDiffTool(path, unstaged)
	if err == nil || !strings.HasPrefix(err.Error(), "nosuchtool ") {
		t.Errorf("unknown tool: %v", err)
	}
}
