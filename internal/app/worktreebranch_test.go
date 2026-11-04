package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// worktreeFixture opens a repository on main with the branch "feature" checked out in a
// linked worktree, one commit ahead of main.
func worktreeFixture(t *testing.T) (*App, *gittest.Repo, string, string) {
	t.Helper()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "base")
	dir := filepath.Join(t.TempDir(), "feature-wt")
	fx.Git("worktree", "add", "-b", "feature", dir)
	fx.Git("-C", dir, "commit", "--allow-empty", "-m", "feature work")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	loadedPage(t, a, tab.Path)
	return a, fx, tab.Path, dir
}

func localRef(t *testing.T, a *App, path, name string) *git.Ref {
	t.Helper()
	refs, err := a.Refs(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := range refs {
		if refs[i].Kind == git.LocalBranch && refs[i].Short == name {
			return &refs[i]
		}
	}
	return nil
}

func branchExists(fx *gittest.Repo, name string) bool {
	return strings.TrimSpace(fx.Git("branch", "--list", name)) != ""
}

func TestRefsSayWhichWorktreeHasABranch(t *testing.T) {
	a, _, path, dir := worktreeFixture(t)
	r := localRef(t, a, path, "feature")
	if r == nil || r.Worktree == "" {
		t.Fatalf("feature = %+v, want its worktree", r)
	}
	if !samePathForTest(t, r.Worktree, dir) {
		t.Errorf("worktree = %q, want %q", r.Worktree, dir)
	}
	// The branch checked out here isn't "in another worktree".
	if m := localRef(t, a, path, "main"); m == nil || m.Worktree != "" || !m.Head {
		t.Errorf("main = %+v", m)
	}
}

func TestPlainDeleteOfABranchInAWorktreeNamesWhy(t *testing.T) {
	a, _, path, _ := worktreeFixture(t)
	err := a.BranchAction(path, "delete", []string{"feature", "true"})
	if err == nil || !strings.Contains(err.Error(), "another worktree") {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteBranchAndWorktree(t *testing.T) {
	a, fx, path, dir := worktreeFixture(t)
	if err := a.DeleteBranchAndWorktree(path, "feature", false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("worktree folder still there: %v", err)
	}
	if branchExists(fx, "feature") {
		t.Error("branch still there")
	}
}

func TestDeleteBranchAndWorktreeTouchesNothingIfCommitsWouldBeLost(t *testing.T) {
	a, fx, path, dir := worktreeFixture(t)
	err := a.DeleteBranchAndWorktree(path, "feature", false, false)
	if err == nil || !strings.Contains(err.Error(), "aren't merged") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("worktree was removed anyway: %v", err)
	}
	if !branchExists(fx, "feature") {
		t.Error("branch was deleted anyway")
	}
}

func TestDeleteBranchAndWorktreeNeedsATickForUncommittedChanges(t *testing.T) {
	a, fx, path, dir := worktreeFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := a.DeleteBranchAndWorktree(path, "feature", false, true)
	if err == nil {
		t.Fatal("removed a worktree with uncommitted changes")
	}
	if !branchExists(fx, "feature") {
		t.Error("branch was deleted though the worktree stayed")
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Errorf("the change is gone: %v", err)
	}
	if err := a.DeleteBranchAndWorktree(path, "feature", true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("worktree folder still there: %v", err)
	}
}

func TestDeleteBranchAndWorktreeWhenTheFolderIsAlreadyGone(t *testing.T) {
	a, fx, path, dir := worktreeFixture(t)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteBranchAndWorktree(path, "feature", false, true); err != nil {
		t.Fatal(err)
	}
	if branchExists(fx, "feature") {
		t.Error("branch still there")
	}
}

func TestDeleteBranchAndWorktreeWithTheWorktreeOpenInATab(t *testing.T) {
	a, _, path, dir := worktreeFixture(t)
	other, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	loadedPage(t, a, other.Path)
	if err := a.DeleteBranchAndWorktree(path, "feature", false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("worktree folder still there: %v", err)
	}
	if err := a.CloseRepo(other.Path); err != nil {
		t.Errorf("closing the tab afterwards: %v", err)
	}
}

func TestDeleteBranchAndWorktreeRefusesTheMainWorktree(t *testing.T) {
	a, fx, _, dir := worktreeFixture(t)
	tab, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = a.DeleteBranchAndWorktree(tab.Path, "main", false, true)
	if err == nil || !strings.Contains(err.Error(), "main worktree") {
		t.Fatalf("err = %v", err)
	}
	if !branchExists(fx, "main") {
		t.Error("main was deleted")
	}
}

func TestDeleteBranchAndWorktreeRefusesTheOneOpenHere(t *testing.T) {
	a, _, _, dir := worktreeFixture(t)
	tab, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = a.DeleteBranchAndWorktree(tab.Path, "feature", false, true)
	if err == nil || !strings.Contains(err.Error(), "open here") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("worktree was removed: %v", err)
	}
}

func TestAWorktreeAddedOrRemovedInTheAppUpdatesTheBranch(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "base")
	fx.Git("branch", "feature")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	loadedPage(t, a, tab.Path)
	dir := filepath.Join(t.TempDir(), "wt")
	if err := a.AddWorktree(tab.Path, dir, "feature"); err != nil {
		t.Fatal(err)
	}
	waitForWorktree(t, a, tab.Path, "feature", true)
	if err := a.RemoveWorktree(tab.Path, dir, false); err != nil {
		t.Fatal(err)
	}
	waitForWorktree(t, a, tab.Path, "feature", false)
}

func TestAWorktreeChangedOutsideTheAppUpdatesTheBranch(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "base")
	fx.Git("branch", "feature")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	loadedPage(t, a, tab.Path)
	dir := filepath.Join(t.TempDir(), "wt")
	fx.Git("worktree", "add", dir, "feature")
	waitForWorktree(t, a, tab.Path, "feature", true)
}

func waitForWorktree(t *testing.T, a *App, path, name string, want bool) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if r := localRef(t, a, path, name); r != nil && (r.Worktree != "") == want {
			return
		}
	}
	t.Fatalf("%s never reported worktree = %v", name, want)
}

func samePathForTest(t *testing.T, a, b string) bool {
	t.Helper()
	ra, err := filepath.EvalSymlinks(filepath.FromSlash(a))
	if err != nil {
		t.Fatal(err)
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		t.Fatal(err)
	}
	return strings.EqualFold(ra, rb)
}
