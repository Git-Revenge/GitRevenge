package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestPreviewMergeForecastsConflicts(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("topic")
	fx.CommitFile("f.txt", "theirs\n", "Change f")
	fx.CommitFile("g.txt", "g\n", "Add g")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "ours\n", "Change f on main")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	head := fx.Head()

	p, err := a.PreviewMerge(tab.Path, "topic")
	if err != nil {
		t.Fatal(err)
	}
	if p.Into != "main" || p.FastForward || p.AlreadyMerged || p.Unknown || p.CommitCount != 2 || p.IntoOnly != 1 || p.Files != 2 {
		t.Fatalf("preview = %+v", p)
	}
	if len(p.Conflicts) != 1 || p.Conflicts[0].Path != "f.txt" || p.Conflicts[0].Places != 1 {
		t.Fatalf("conflicts = %+v", p.Conflicts)
	}
	if fx.Head() != head || fx.Git("status", "--porcelain") != "" {
		t.Fatal("previewing a merge changed the repository")
	}
	if _, err := a.PreviewMerge(tab.Path, " "); err == nil {
		t.Fatal("a preview of nothing should be refused")
	}
}

func TestPreviewRebaseNamesWhereItStops(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("topic")
	fx.CommitFile("a.txt", "a\n", "Cache readings")
	fx.CommitFile("a.txt", "a2\n", "fixup! Cache readings")
	stops := fx.CommitFile("f.txt", "theirs\n", "Change f")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "ours\n", "Change f on main")
	fx.Checkout("topic")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}

	p, err := a.PreviewRebase(tab.Path, "main")
	if err != nil {
		t.Fatal(err)
	}
	if p.Branch != "topic" || p.UpToDate || p.CommitCount != 3 || p.Fixups != 1 || p.Pushed {
		t.Fatalf("preview = %+v", p)
	}
	if p.StopsAt != stops || p.StopsAtSubject != "Change f" || p.StopsAtNumber != 3 || len(p.Conflicts) != 1 {
		t.Fatalf("stops at %s %q (number %d) on %+v", p.StopsAt, p.StopsAtSubject, p.StopsAtNumber, p.Conflicts)
	}
	if fx.Git("status", "--porcelain") != "" || fx.Git("rev-parse", "--abbrev-ref", "HEAD") != "topic" {
		t.Fatal("previewing a rebase changed the repository")
	}
}
