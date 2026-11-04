package app

import (
	"os"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestWorkingTreeFlow(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "base")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fx.WriteFile("a.txt", "one\ntwo\n")
	fx.WriteFile("new.txt", "new\n")

	wt, err := a.WorkingTree(tab.Path)
	if err != nil || len(wt.Status.Files) != 2 || wt.Signing.Enabled {
		t.Fatalf("working tree = %+v, %v", wt, err)
	}
	if err := a.StageFiles(tab.Path, []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	fc, err := a.FileContents(tab.Path, DiffStaged, "", "a.txt", "", false)
	if err != nil || fc.New.Text != "one\ntwo\n" {
		t.Fatalf("staged contents = %+v, %v", fc, err)
	}
	if _, err := a.CommitChanges(tab.Path, "Add two", false); err != nil {
		t.Fatal(err)
	}
	if msg, _ := a.LastCommitMessage(tab.Path); msg != "Add two" {
		t.Fatalf("last message = %q", msg)
	}

	// Nothing staged now: committing again explains why it can't.
	if _, err := a.CommitChanges(tab.Path, "again", false); err == nil || !strings.Contains(err.Error(), "Nothing is staged") {
		t.Fatalf("err = %v", err)
	}

	// Discard decides from fresh status: new.txt is untracked, so it's deleted.
	if err := a.DiscardFiles(tab.Path, []string{"new.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fx.Path("new.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file should be deleted")
	}
}

func TestStageAndUnstageAll(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	tab, _ := a.OpenRepo(fx.Dir)
	fx.WriteFile("x.txt", "x")
	fx.WriteFile("y.txt", "y")
	if err := a.StageFiles(tab.Path, nil); err != nil {
		t.Fatal(err)
	}
	wt, _ := a.WorkingTree(tab.Path)
	for _, f := range wt.Status.Files {
		if !f.Staged() {
			t.Fatalf("%s not staged", f.Path)
		}
	}
	if err := a.UnstageFiles(tab.Path, nil); err != nil {
		t.Fatal(err)
	}
	wt, _ = a.WorkingTree(tab.Path)
	for _, f := range wt.Status.Files {
		if f.Staged() {
			t.Fatalf("%s still staged", f.Path)
		}
	}
}
