package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestHistoryAndBlameBindings(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "first")
	fx.CommitFile("a.txt", "one\ntwo\n", "second")
	tab, _ := a.OpenRepo(fx.Dir)

	res, err := a.FileHistory(tab.Path, "a.txt")
	hist := res.Commits
	if err != nil || len(hist) != 2 || hist[0].Subject != "second" || res.Stop != nil {
		t.Fatalf("history = %+v, %v", hist, err)
	}
	lines, err := a.Blame(tab.Path, "a.txt", "")
	if err != nil || len(lines) != 2 || lines[1].Summary != "second" {
		t.Fatalf("blame = %+v, %v", lines, err)
	}
}

// #1078: the blame view's track, and each step's text and change. The work tree is a last
// step only when the file has changes not committed yet.
func TestBlameTrackAndItsSteps(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	first := fx.CommitFile("a.txt", "one\n", "first")
	second := fx.CommitFile("a.txt", "one\ntwo\n", "second")
	tab, _ := a.OpenRepo(fx.Dir)

	track, err := a.BlameTrack(tab.Path, "a.txt", "")
	if err != nil || len(track.Versions) != 2 || track.Versions[0].ID != first || track.Versions[1].ID != second || track.WorkTree || track.RenamedFrom != "" {
		t.Fatalf("track = %+v, %v", track, err)
	}
	// A step: the version's text, and what it changed against the one before.
	step, err := a.FileDiff(tab.Path, DiffCommit, second, "a.txt", "", false, ReadLines)
	if err != nil || step.Contents.New.Text != "one\ntwo\n" || step.Diff == nil || step.Diff.Added != 1 || step.Diff.Removed != 0 {
		t.Fatalf("second's step = %+v, %v", step, err)
	}

	fx.WriteFile("a.txt", "one\ntwo, changed\n")
	if track, _ = a.BlameTrack(tab.Path, "a.txt", ""); !track.WorkTree {
		t.Error("the work tree's change made no last step")
	}
	// Blamed at a commit in HEAD's history, the track still runs to the newest version and the
	// work tree, with home on the commit's version (#1083).
	if track, _ = a.BlameTrack(tab.Path, "a.txt", first); !track.WorkTree || len(track.Versions) != 2 || track.Home != 0 {
		t.Errorf("at the first commit: %+v", track)
	}
	// A commit that didn't change the file is at the version before it.
	fx.Git("stash")
	other := fx.CommitFile("b.txt", "b\n", "elsewhere")
	if track, _ = a.BlameTrack(tab.Path, "a.txt", other); len(track.Versions) != 2 || track.Home != 1 {
		t.Errorf("at a commit that left a.txt alone: %+v", track)
	}
	// One HEAD hasn't got has a track of its own history, with no work tree.
	fx.Git("checkout", "-q", "-b", "side", first)
	side := fx.CommitFile("a.txt", "one\nside\n", "on the side")
	fx.Git("checkout", "-q", "main")
	fx.Git("stash", "pop")
	if track, _ = a.BlameTrack(tab.Path, "a.txt", side); track.WorkTree || len(track.Versions) != 2 || track.Home != 1 || track.Versions[1].ID != side {
		t.Errorf("at a commit HEAD hasn't got: %+v", track)
	}
	last, err := a.FileDiff(tab.Path, DiffWorkTree, "", "a.txt", "", false, ReadLines)
	if err != nil || last.Contents.Old.Text != "one\ntwo\n" || last.Contents.New.Text != "one\ntwo, changed\n" || last.Diff == nil || last.Diff.Added != 1 || last.Diff.Removed != 1 {
		t.Fatalf("the work tree's step = %+v, %v", last, err)
	}
}

// #1081: Rewind these lines, through the app.
func TestBlameBlock(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\ntwo\nthree\n", "first")
	second := fx.CommitFile("a.txt", "one\nTWO\nthree\n", "second")
	tab, _ := a.OpenRepo(fx.Dir)
	got, err := a.BlameBlock(tab.Path, "a.txt", second, 2, 2)
	if err != nil || len(got) != 2 || got[0].Subject != "second" || got[1].Subject != "first" {
		t.Fatalf("block = %+v, %v", got, err)
	}
	if _, err := a.BlameBlock(tab.Path, "a.txt", "", 1, 1); err == nil {
		t.Error("rewound lines not committed yet")
	}
}
