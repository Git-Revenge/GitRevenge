package app

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// windTab is an open repository where km/h is in two files, the way the demo's map pin still
// says it.
func windTab(t *testing.T, a *App) string {
	t.Helper()
	fx := gittest.New(t)
	fx.WriteFile("public/map.js", "pin.title = `${s.name}: ${s.wind} km/h`\n")
	fx.WriteFile("notes.md", "km/h, and km/h again\nnothing here\n")
	fx.Commit("wind")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return tab.Path
}

func TestFindInFilesSendsItsLinesAndSaysHowItEnded(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	path := windTab(t, a)
	var mu sync.Mutex
	var batches []FindLines
	a.emitHook = func(name string, data any) {
		if b, ok := data.(FindLines); ok && name == "find:lines" {
			mu.Lock()
			batches = append(batches, b)
			mu.Unlock()
		}
	}
	res, err := a.FindInFiles(path, "search-1", git.GrepOptions{Text: "km/h"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines != 2 || res.Files != 2 || res.Truncated || res.Superseded {
		t.Fatalf("result = %+v", res)
	}
	mu.Lock()
	defer mu.Unlock()
	var lines []git.GrepLine
	for _, b := range batches {
		if b.Token != "search-1" || b.Path != path {
			t.Fatalf("batch for %q in %q", b.Token, b.Path)
		}
		lines = append(lines, b.Lines...)
	}
	// Every line arrives before the call answers, in git's order.
	if len(lines) != 2 || lines[0].File != "notes.md" || len(lines[0].Matches) != 2 || lines[1].File != "public/map.js" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestFindCountCountsWithoutSendingLines(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	path := windTab(t, a)
	var sent atomic.Bool
	a.emitHook = func(name string, _ any) {
		if name == "find:lines" {
			sent.Store(true)
		}
	}
	got, err := a.FindCount(path, git.GrepOptions{Text: "KM/H"})
	if err != nil || got != (FindCounted{Lines: 2, Files: 2}) {
		t.Fatalf("count = %+v, %v", got, err)
	}
	got, err = a.FindCount(path, git.GrepOptions{Text: "KM/H", MatchCase: true})
	if err != nil || got != (FindCounted{}) {
		t.Fatalf("count matching case = %+v, %v", got, err)
	}
	if sent.Load() {
		t.Fatal("a count sent lines")
	}
}

func TestFindSaysWhatsWrongInWords(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	path := windTab(t, a)
	for _, c := range []struct {
		o    git.GrepOptions
		want string
	}{
		{git.GrepOptions{Text: "kmalloc_array(", Regex: true}, "Not a regular expression: a ( is never closed. Turn off Regular expression"},
		{git.GrepOptions{Text: "km/h", Rev: "no-such-branch"}, "There's no branch, tag or commit called “no-such-branch”."},
		{git.GrepOptions{Text: "two\nlines"}, "Find looks for text within one line."},
	} {
		_, err := a.FindInFiles(path, "t", c.o)
		var ue userError
		if !errors.As(err, &ue) || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("%+v: err = %v", c.o, err)
		}
		if _, err := a.FindCount(path, c.o); err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("count %+v: err = %v", c.o, err)
		}
	}
	if _, err := a.FindInFiles(t.TempDir(), "t", git.GrepOptions{Text: "km/h"}); err == nil {
		t.Error("searched a repository that isn't open")
	}
}
