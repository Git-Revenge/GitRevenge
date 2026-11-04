package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if _, err := a.DeleteBranchAndWorktree(path, "feature", false, true); err != nil {
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
	_, err := a.DeleteBranchAndWorktree(path, "feature", false, false)
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
	_, err := a.DeleteBranchAndWorktree(path, "feature", false, true)
	if err == nil {
		t.Fatal("removed a worktree with uncommitted changes")
	}
	if !branchExists(fx, "feature") {
		t.Error("branch was deleted though the worktree stayed")
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Errorf("the change is gone: %v", err)
	}
	if _, err := a.DeleteBranchAndWorktree(path, "feature", true, true); err != nil {
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
	if _, err := a.DeleteBranchAndWorktree(path, "feature", false, true); err != nil {
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
	if _, err := a.DeleteBranchAndWorktree(path, "feature", false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("worktree folder still there: %v", err)
	}
	if _, err := a.openRepo(other.Path); err == nil {
		t.Error("the worktree's tab is still open")
	}
	// The UI drops the tab next, which is fine with it already closed.
	if err := a.CloseRepo(other.Path); err != nil {
		t.Errorf("closing the tab afterwards: %v", err)
	}
}

func TestARefusedRemovalLeavesTheWorktreesTabOpen(t *testing.T) {
	a, _, path, dir := worktreeFixture(t)
	other, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DeleteBranchAndWorktree(path, "feature", false, true); err == nil {
		t.Fatal("removed a worktree with uncommitted changes")
	}
	if _, err := a.openRepo(other.Path); err != nil {
		t.Errorf("the tab closed: %v", err)
	}
}

// A process with its current folder in the worktree stops Windows deleting the folder, as a
// terminal or editor does. Git drops the worktree anyway, so the branch can still go.
func TestDeleteBranchAndWorktreeWithSomethingElseHoldingTheFolder(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows stops a folder in use being deleted")
	}
	a, fx, path, dir := worktreeFixture(t)
	holder := exec.Command("ping", "-n", "60", "127.0.0.1")
	holder.Dir = dir
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = holder.Process.Kill()
		_ = holder.Wait()
	})
	leftover, err := a.DeleteBranchAndWorktree(path, "feature", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if leftover == "" {
		t.Error("didn't mention the folder left behind")
	}
	if branchExists(fx, "feature") {
		t.Error("branch still there")
	}
}

func TestDeleteBranchAndWorktreeRefusesTheMainWorktree(t *testing.T) {
	a, fx, _, dir := worktreeFixture(t)
	tab, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.DeleteBranchAndWorktree(tab.Path, "main", false, true)
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
	_, err = a.DeleteBranchAndWorktree(tab.Path, "feature", false, true)
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
	if _, err := a.RemoveWorktree(tab.Path, dir, false); err != nil {
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

// waitForBranch waits for the graph to list a branch made outside the app, which only the file
// watcher can report.
func waitForBranch(t *testing.T, a *App, path, name string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if localRef(t, a, path, name) != nil {
			return
		}
	}
	t.Fatalf("%s was never noticed", name)
}

func TestClosingAWorktreesTabKeepsTheMainTabWatching(t *testing.T) {
	a, fx, path, dir := worktreeFixture(t)
	other, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	loadedPage(t, a, other.Path)
	if err := a.CloseRepo(other.Path); err != nil {
		t.Fatal(err)
	}
	fx.Git("branch", "made-outside")
	waitForBranch(t, a, path, "made-outside")
}

func TestClosingTheMainTabKeepsAWorktreesTabWatching(t *testing.T) {
	a, fx, path, dir := worktreeFixture(t)
	other, err := a.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Loaded first, so only the watcher can report what comes next.
	loadedPage(t, a, other.Path)
	if err := a.CloseRepo(path); err != nil {
		t.Fatal(err)
	}
	// A worktree's tab watches its own git folder, so switching branches is what it can see.
	fx.Git("-C", dir, "checkout", "-q", "-b", "made-outside")
	waitForBranch(t, a, other.Path, "made-outside")
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
