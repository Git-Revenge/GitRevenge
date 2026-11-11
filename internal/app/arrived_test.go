package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestHeadCommit(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(3)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := a.HeadCommit(tab.Path)
	if err != nil || head.Branch != "main" || head.Commit != shas[len(shas)-1] {
		t.Fatalf("on a branch: %+v, %v; want main at %s", head, err, shas[len(shas)-1])
	}
	fx.Git("checkout", "-q", "--detach", shas[0])
	head, err = a.HeadCommit(tab.Path)
	if err != nil || head.Branch != "" || head.Commit != shas[0] {
		t.Fatalf("detached: %+v, %v; want no branch at %s", head, err, shas[0])
	}
}

func TestArrivedCommits(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(4) // oldest first
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// A branch pulled forward from the first commit to the last gained the three in between,
	// newest first.
	got, err := a.ArrivedCommits(tab.Path, shas[0], shas[3])
	if err != nil {
		t.Fatal(err)
	}
	want := []string{shas[3], shas[2], shas[1]}
	if got.Total != 3 || !reflect.DeepEqual(got.Commits, want) {
		t.Fatalf("forward = %+v, want %v", got, want)
	}
	// Moved backwards, or not at all, it gained nothing.
	for _, to := range []string{shas[0], shas[3]} {
		got, err := a.ArrivedCommits(tab.Path, shas[3], to)
		if err != nil || got.Total != 0 || len(got.Commits) != 0 || got.Commits == nil {
			t.Fatalf("to %s = %+v, %v; want nothing, as an empty list", to, got, err)
		}
	}
	// Only commit IDs are taken: nothing that git could read as an option or a range.
	for _, bad := range []string{"", "--all", "main", shas[0] + ".." + shas[3], "HEAD~1"} {
		if _, err := a.ArrivedCommits(tab.Path, bad, shas[3]); err == nil {
			t.Fatalf("from %q was accepted", bad)
		}
		if _, err := a.ArrivedCommits(tab.Path, shas[0], bad); err == nil {
			t.Fatalf("to %q was accepted", bad)
		}
	}
}

func TestOwnMoveAge(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(2)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// Nothing done here yet, and a commit made by another tool doesn't count.
	fx.CommitFile("outside.txt", "x", "Made in a terminal")
	if age, err := a.OwnMoveAge(tab.Path); err != nil || age != -1 {
		t.Fatalf("before anything = %d, %v; want -1", age, err)
	}
	// A commit made through the app does.
	fx.WriteFile("inside.txt", "y")
	fx.Git("add", "inside.txt")
	if _, err := a.CommitChanges(tab.Path, "Made here", false); err != nil {
		t.Fatal(err)
	}
	age, err := a.OwnMoveAge(tab.Path)
	if err != nil || age < 0 || age > 60_000 {
		t.Fatalf("after a commit here = %d, %v; want a recent age", age, err)
	}
	// Undoing it is the app's doing too: the age starts again from the undo.
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	again, _ := a.OwnMoveAge(tab.Path)
	if since := int(time.Since(start).Milliseconds()); again < 0 || again > since {
		t.Fatalf("after undo = %dms, want no more than the %dms since the undo began", again, since)
	}
}
