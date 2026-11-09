package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func changedPaths(t *testing.T, a *App, path string) []string {
	t.Helper()
	wt, err := a.WorkingTree(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range wt.Status.Files {
		out = append(out, f.Path)
	}
	slices.Sort(out)
	return out
}

func TestIgnoreAndUndo(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.WriteFile("cache/kept.txt", "tracked\n")
	fx.Commit("base")
	fx.WriteFile("cache/a.bin", "x\n")
	fx.WriteFile("notes.txt", "x\n")
	fx.Git("config", "core.excludesFile", filepath.Join(t.TempDir(), "g"))
	tab, _ := a.OpenRepo(fx.Dir)

	pv, err := a.PreviewIgnore(tab.Path, git.IgnoreTarget{Scope: git.IgnoreRoot}, "/cache/")
	if err != nil || !slices.Equal(pv.Hidden, []string{"cache/a.bin"}) || !slices.Equal(pv.Tracked, []string{"cache/kept.txt"}) {
		t.Fatalf("preview = %+v, %v", pv, err)
	}
	added, err := a.AddIgnore(tab.Path, git.IgnoreTarget{Scope: git.IgnoreRoot}, "/cache/", pv.Tracked)
	if err != nil {
		t.Fatal(err)
	}
	if added.Hidden != 1 || added.Rule.Line != 1 {
		t.Fatalf("added = %+v", added)
	}
	// .gitignore is new and the tracked file's removal is staged; cache/a.bin is gone.
	if got, want := changedPaths(t, a, tab.Path), []string{".gitignore", "cache/kept.txt", "notes.txt"}; !slices.Equal(got, want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	if _, err := os.Stat(fx.Path("cache/kept.txt")); err != nil {
		t.Fatal("stopping tracking deleted the file:", err)
	}

	if err := a.UndoIgnore(tab.Path, added); err != nil {
		t.Fatal(err)
	}
	if got, want := changedPaths(t, a, tab.Path), []string{"cache/a.bin", "notes.txt"}; !slices.Equal(got, want) {
		t.Fatalf("changes after undo = %v, want %v", got, want)
	}
	if err := a.UndoIgnore(tab.Path, added); err == nil {
		t.Fatal("undid a rule that's already gone")
	}
}

func TestStopIgnoring(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile(".gitignore", "*.log\n", "base")
	fx.WriteFile("a.log", "x\n")
	fx.WriteFile("b.log", "x\n")
	fx.Git("config", "core.excludesFile", filepath.Join(t.TempDir(), "g"))
	tab, _ := a.OpenRepo(fx.Dir)

	ig, err := a.IgnoredFiles(tab.Path)
	if err != nil || len(ig.Files) != 2 {
		t.Fatalf("ignored = %+v, %v", ig, err)
	}
	ex, err := a.StopIgnoring(tab.Path, "a.log", ig.Files[0].Rule, true)
	if err != nil || ex.Pattern != "!/a.log" {
		t.Fatalf("exception = %+v, %v", ex, err)
	}
	if got, want := changedPaths(t, a, tab.Path), []string{".gitignore", "a.log"}; !slices.Equal(got, want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	if err := a.RemoveIgnoreRule(tab.Path, ex); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StopIgnoring(tab.Path, "a.log", ig.Files[0].Rule, false); err != nil {
		t.Fatal(err)
	}
	if got, want := changedPaths(t, a, tab.Path), []string{".gitignore", "a.log", "b.log"}; !slices.Equal(got, want) {
		t.Fatalf("changes after removing the rule = %v, want %v", got, want)
	}
}
