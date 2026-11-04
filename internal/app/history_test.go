package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestHistoryAndBlameBindings(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "one\n", "first")
	fx.CommitFile("a.txt", "one\ntwo\n", "second")
	tab, _ := a.OpenRepo(fx.Dir)

	hist, err := a.FileHistory(tab.Path, "a.txt")
	if err != nil || len(hist) != 2 || hist[0].Subject != "second" {
		t.Fatalf("history = %+v, %v", hist, err)
	}
	lines, err := a.Blame(tab.Path, "a.txt", "")
	if err != nil || len(lines) != 2 || lines[1].Summary != "second" {
		t.Fatalf("blame = %+v, %v", lines, err)
	}
}
