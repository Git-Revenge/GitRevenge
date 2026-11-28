package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// #1082: restoring a version into the work tree, asking first, and Undo bringing back what
// was there, uncommitted changes and all. History and the index are left alone.
func TestRestoreAVersionAndUndoIt(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	first := fx.CommitFile("a.txt", "one\ntwo\n", "first")
	fx.CommitFile("a.txt", "one\nTWO\nthree\n", "second")
	head := fx.Head()
	tab, _ := a.OpenRepo(fx.Dir)
	fx.WriteFile("a.txt", "one\nTWO\nthree\nmine\n")
	read := func() string {
		b, _ := os.ReadFile(filepath.Join(fx.Dir, "a.txt"))
		return string(b)
	}

	plan, err := a.RestorePlan(tab.Path, "a.txt", first, "")
	if err != nil || plan.Added != 1 || plan.Removed != 3 || plan.Uncommitted != 1 {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	done, err := a.RestoreVersion(tab.Path, "a.txt", first, "")
	if err != nil || done.ID == "" || read() != "one\ntwo\n" {
		t.Fatalf("restored = %+v, %q, %v", done, read(), err)
	}
	if fx.Head() != head || fx.Git("diff", "--cached", "--name-only") != "" {
		t.Error("history or the index moved")
	}
	if err := a.UndoRestore(tab.Path, done.ID); err != nil || read() != "one\nTWO\nthree\nmine\n" {
		t.Fatalf("undo = %q, %v", read(), err)
	}
	if err := a.UndoRestore(tab.Path, done.ID); err == nil {
		t.Error("undid twice")
	}

	// Touched since: Undo says so and leaves the file alone.
	done, _ = a.RestoreVersion(tab.Path, "a.txt", first, "")
	fx.WriteFile("a.txt", "edited after\n")
	if err := a.UndoRestore(tab.Path, done.ID); err == nil || read() != "edited after\n" {
		t.Errorf("undo after an edit: %q, %v", read(), err)
	}
}

func TestRestoreABlocksLines(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\nold\nc\n", "first")
	second := fx.CommitFile("a.txt", "a\nnew\nc\n", "second")
	tab, _ := a.OpenRepo(fx.Dir)
	// A line went in above since: the block moved down, and is put back where it is now.
	fx.WriteFile("a.txt", "top\na\nnew\nc\n")
	done, err := a.RestoreBlock(tab.Path, "a.txt", "", second, 2, 2, []string{"old"})
	b, _ := os.ReadFile(filepath.Join(fx.Dir, "a.txt"))
	if err != nil || done.ID == "" || string(b) != "top\na\nold\nc\n" {
		t.Fatalf("restored lines = %q, %v", b, err)
	}
	// Changed since: it says so, and writes nothing.
	fx.WriteFile("a.txt", "a\nedited\nc\n")
	if _, err := a.RestoreBlock(tab.Path, "a.txt", "", second, 2, 2, []string{"old"}); err == nil {
		t.Error("put lines back over ones changed since")
	}
	if b, _ := os.ReadFile(filepath.Join(fx.Dir, "a.txt")); string(b) != "a\nedited\nc\n" {
		t.Errorf("the file changed: %q", b)
	}
}
