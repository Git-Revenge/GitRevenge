package app

import (
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// focusFixture is a repository on main with two unrelated side branches.
//
//	base ── main work
//	   ├── one work
//	   └── two work
func focusFixture(t *testing.T) (*App, *gittest.Repo, string) {
	t.Helper()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "base")
	fx.Branch("one")
	fx.CommitFile("one.txt", "1", "one work")
	fx.Checkout("main")
	fx.Branch("two")
	fx.CommitFile("two.txt", "1", "two work")
	fx.Checkout("main")
	fx.CommitFile("m.txt", "1", "main work")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if page := loadedPage(t, a, tab.Path); page.Total != 4 {
		t.Fatalf("total = %d before focusing", page.Total)
	}
	return a, fx, tab.Path
}

// waitForSubjects waits for the graph to show exactly these commits.
func waitForSubjects(t *testing.T, a *App, path string, want ...string) GraphPage {
	t.Helper()
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w] = true
	}
	var page GraphPage
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		page = loadedPage(t, a, path)
		same := len(page.Rows) == len(want)
		for _, r := range page.Rows {
			same = same && wanted[r.Subject]
		}
		if same {
			return page
		}
	}
	got := []string{}
	for _, r := range page.Rows {
		got = append(got, r.Subject)
	}
	t.Fatalf("graph shows %v, want %v", got, want)
	return page
}

func focusedNames(t *testing.T, a *App, path string) map[string]bool {
	t.Helper()
	refs, err := a.Refs(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, r := range refs {
		if r.Focused {
			out[r.Short] = true
		}
	}
	return out
}

func TestFocusShowsOneBranchUntilItIsCleared(t *testing.T) {
	t.Parallel()
	a, _, path := focusFixture(t)
	if err := a.SetGraphFocus(path, "one", false); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, path, "one work", "base")
	if got, _ := a.GraphFocus(path); got != (session.Focus{Branch: "one"}) {
		t.Errorf("focus = %+v", got)
	}
	if got := focusedNames(t, a, path); len(got) != 1 || !got["one"] {
		t.Errorf("focused refs = %v", got)
	}

	if err := a.SetGraphFocus(path, "", false); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, path, "main work", "two work", "one work", "base")
	if got, _ := a.GraphFocus(path); got != (session.Focus{}) {
		t.Errorf("focus = %+v after clearing", got)
	}
	if got := focusedNames(t, a, path); len(got) != 0 {
		t.Errorf("focused refs = %v after clearing", got)
	}
}

func TestOnlyOneFocusAtATime(t *testing.T) {
	t.Parallel()
	a, _, path := focusFixture(t)
	for _, b := range []string{"one", "two"} {
		if err := a.SetGraphFocus(path, b, false); err != nil {
			t.Fatal(err)
		}
	}
	waitForSubjects(t, a, path, "two work", "base")
	if got := focusedNames(t, a, path); len(got) != 1 || !got["two"] {
		t.Errorf("focused refs = %v", got)
	}
}

func TestFocusKeepsLabelsOnTheCommitsShown(t *testing.T) {
	t.Parallel()
	a, fx, path := focusFixture(t)
	base := fx.Git("rev-list", "--max-parents=0", "HEAD")
	fx.Git("branch", "at-base", base[:40])
	if err := a.SetGraphFocus(path, "one", false); err != nil {
		t.Fatal(err)
	}
	page := waitForSubjects(t, a, path, "one work", "base")
	found := false
	for _, list := range page.Refs {
		for _, r := range list {
			found = found || r.Short == "at-base"
			if r.Short == "two" || r.Short == "main" {
				t.Errorf("label for %s, on a commit that isn't shown", r.Short)
			}
		}
	}
	if !found {
		t.Error("the branch on a shown commit lost its label")
	}
}

func TestFocusIncludesTheRemoteCopy(t *testing.T) {
	t.Parallel()
	a, fx, path := focusFixture(t)
	fx.Git("update-ref", "refs/remotes/origin/one", fx.Git("rev-parse", "two")[:40])
	fx.Git("remote", "add", "origin", fx.Dir)
	if err := a.SetGraphFocus(path, "one", false); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, path, "one work", "two work", "base")
	if got := focusedNames(t, a, path); !got["one"] || !got["origin/one"] {
		t.Errorf("focused refs = %v", got)
	}
}

func TestFocusOnlyMatchesTheWholeName(t *testing.T) {
	t.Parallel()
	a, fx, path := focusFixture(t)
	fx.Git("branch", "one-more", "two")
	if err := a.SetGraphFocus(path, "one", false); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, path, "one work", "base")
}

func TestFocusCanShowAHiddenBranch(t *testing.T) {
	t.Parallel()
	a, _, path := focusFixture(t)
	if err := a.HideBranch(path, "one", true); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGraphFocus(path, "one", false); err != nil {
		t.Fatal(err)
	}
	page := waitForSubjects(t, a, path, "one work", "base")
	found := false
	for _, list := range page.Refs {
		for _, r := range list {
			found = found || r.Short == "one"
		}
	}
	if !found {
		t.Error("the focused branch has no label")
	}
}

func TestFocusLeavesOutStashes(t *testing.T) {
	t.Parallel()
	a, fx, path := focusFixture(t)
	fx.WriteFile("a.txt", "changed")
	fx.Stash("kept")
	if err := a.SetGraphFocus(path, "one", false); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, path, "one work", "base")
}

func TestFocusOnAStackCoversEveryBranchInIt(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "base")
	fx.Branch("low")
	fx.CommitFile("l.txt", "1", "low work")
	fx.Branch("high")
	fx.CommitFile("h.txt", "1", "high work")
	fx.Checkout("low")
	fx.Branch("side")
	fx.CommitFile("s.txt", "1", "side work")
	fx.Checkout("main")
	fx.Branch("other")
	fx.CommitFile("o.txt", "1", "other work")
	fx.Checkout("main")
	fx.CommitFile("m.txt", "1", "main work")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	loadedPage(t, a, tab.Path)
	if err := a.SetStackParent(tab.Path, "high", "low", false); err != nil {
		t.Fatal(err)
	}
	if err := a.SetStackParent(tab.Path, "side", "low", false); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGraphFocus(tab.Path, "high", true); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, tab.Path, "side work", "high work", "low work", "base")
	got := focusedNames(t, a, tab.Path)
	if len(got) != 3 || !got["low"] || !got["high"] || !got["side"] {
		t.Errorf("focused refs = %v", got)
	}
}

func TestFocusOnABranchWithNoStackIsJustTheBranch(t *testing.T) {
	t.Parallel()
	a, _, path := focusFixture(t)
	if err := a.SetGraphFocus(path, "one", true); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, path, "one work", "base")
}

func TestFocusEndsWhenTheBranchGoes(t *testing.T) {
	t.Parallel()
	a, _, path := focusFixture(t)
	if err := a.SetGraphFocus(path, "one", false); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, path, "one work", "base")
	if err := a.BranchAction(path, "delete", []string{"one", "true"}); err != nil {
		t.Fatal(err)
	}
	// Deleting it is noticed by the watcher, which reloads.
	waitForSubjects(t, a, path, "main work", "two work", "base")
	if got, _ := a.GraphFocus(path); got != (session.Focus{}) {
		t.Errorf("focus = %+v after the branch went", got)
	}
}

func TestFocusOnAMissingBranchIsRefused(t *testing.T) {
	t.Parallel()
	a, _, path := focusFixture(t)
	if err := a.SetGraphFocus(path, "no-such-branch", false); err == nil {
		t.Fatal("focused on a branch that doesn't exist")
	}
	if got, _ := a.GraphFocus(path); got != (session.Focus{}) {
		t.Errorf("focus = %+v", got)
	}
}

func TestFocusDoesNotMarkOtherRefs(t *testing.T) {
	t.Parallel()
	a, _, path := focusFixture(t)
	if err := a.SetGraphFocus(path, "one", false); err != nil {
		t.Fatal(err)
	}
	waitForSubjects(t, a, path, "one work", "base")
	refs, _ := a.Refs(path)
	for _, r := range refs {
		if r.Focused != (r.Short == "one" && r.Kind == git.LocalBranch) {
			t.Errorf("%s focused = %v", r.Short, r.Focused)
		}
	}
}
