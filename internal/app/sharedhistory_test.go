package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// #473
func TestSharedHistory(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	path := tab.Path
	local.Git("checkout", "-q", "-b", "feature", "main")
	local.CommitFile("a.txt", "a\n", "add a")
	local.CommitFile("b.txt", "b\n", "add b")
	local.Git("checkout", "-q", "main")
	if err := a.Integrate(path, "merge", []string{"feature"}); err != nil {
		t.Fatal(err)
	}

	// Not pushed yet, so nothing to warn about.
	got, err := a.PushedUndo(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 0 {
		t.Errorf("before pushing: %+v", got)
	}

	if err := a.PushCurrent(path, false); err != nil {
		t.Fatal(err)
	}
	got, err = a.PushedUndo(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || len(got.Commits) != 2 || got.Upstream != "origin/main" {
		t.Fatalf("after pushing: %+v", got)
	}
	if got.Commits[0].Subject != "add b" || got.Commits[1].Subject != "add a" || got.Commits[0].Author == "" || got.Commits[0].Merge {
		t.Errorf("newest first, with authors: %+v", got.Commits)
	}
	if got, _ := a.PushedUndo(path, false); got.Total != 0 {
		t.Errorf("nothing to redo: %+v", got)
	}

	// Undo still works, and stays local.
	if err := a.Undo(path); err != nil {
		t.Fatal(err)
	}
	onRemote := strings.TrimSpace(remote.Git("rev-parse", "main"))
	if onRemote != strings.TrimSpace(local.Git("rev-parse", "origin/main")) {
		t.Error("the remote should keep them")
	}
	// Redoing gets them back, which takes nothing away.
	if got, _ := a.PushedUndo(path, false); got.Total != 0 {
		t.Errorf("redo: %+v", got)
	}

	// With new work on top, the branch and the remote have parted.
	local.CommitFile("c.txt", "c\n", "add c")
	err = a.PushCurrent(path, false)
	if err == nil || !strings.Contains(err.Error(), "undid or rewrote") {
		t.Errorf("rejected push: %v", err)
	}
	loss, err := a.ForcePushLoss(path)
	if err != nil {
		t.Fatal(err)
	}
	if loss.Total != 2 || loss.Commits[0].Subject != "add b" {
		t.Errorf("force push would replace: %+v", loss)
	}
	if strings.TrimSpace(remote.Git("rev-parse", "main")) != onRemote {
		t.Error("the rejected push moved the remote")
	}
}

func TestRejectedPushFromSomeoneElse(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// Someone else pushes, and it's been fetched, but it was never on this branch here.
	clone := gittest.Clone(t, remote)
	clone.CommitFile("theirs.txt", "theirs\n", "their work")
	clone.Git("push", "-q", "origin", "main")
	local.Git("fetch", "-q", "origin")
	local.CommitFile("mine.txt", "mine\n", "my work")
	err = a.PushCurrent(tab.Path, false)
	if err == nil || strings.Contains(err.Error(), "undid") {
		t.Errorf("should be the usual message: %v", err)
	}
	loss, err := a.ForcePushLoss(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	if loss.Total != 1 || loss.Commits[0].Subject != "their work" {
		t.Errorf("got %+v", loss)
	}
}
