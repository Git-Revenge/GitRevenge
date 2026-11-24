package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// goBranches makes main and theirs, which conflict on one Go line: ours adds a context
// argument at the start, theirs a step at the end. before runs first, on the base commit.
func goBranches(t *testing.T, before func(r *gittest.Repo)) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	if before != nil {
		before(r)
	}
	r.CommitFile("forecast.go", "package tides\n\nfunc Forecast(from int, n int) int {\n\treturn n\n}\n", "base")
	r.Branch("theirs")
	r.CommitFile("forecast.go", "package tides\n\nfunc Forecast(from int, n int, step int) int {\n\treturn n\n}\n", "a step")
	r.Checkout("main")
	r.CommitFile("forecast.go", "package tides\n\nfunc Forecast(ctx int, from int, n int) int {\n\treturn n\n}\n", "a context")
	return r
}

// goConflict is goBranches merged, stopped on the conflict.
func goConflict(t *testing.T) *gittest.Repo {
	t.Helper()
	r := goBranches(t, nil)
	if err := r.Merge("theirs"); err == nil {
		t.Fatal("expected a conflict")
	}
	return r
}

func TestConflictSuggestionsCombineDifferentChanges(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := goConflict(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	place := ConflictPlace{
		Base:    "func Forecast(from int, n int) int {\n",
		Ours:    "func Forecast(ctx int, from int, n int) int {\n",
		Theirs:  "func Forecast(from int, n int, step int) int {\n",
		HasBase: true,
	}
	got, err := a.ConflictSuggestions(tab.Path, "forecast.go", []ConflictPlace{place, {Ours: "a\n", Theirs: "b\n"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "Go" || got.Fallback != "" || len(got.Suggestions) != 2 {
		t.Fatalf("%+v", got)
	}
	if s := got.Suggestions[0]; !s.OK || s.Text != "func Forecast(ctx int, from int, n int, step int) int {\n" {
		t.Errorf("suggestion %+v", s)
	}
	if s := got.Suggestions[1]; s.OK || !strings.Contains(s.Reason, "original") {
		t.Errorf("a place without its original got %+v", s)
	}
	if got.Driver != "" {
		t.Errorf("driver %q with none set", got.Driver)
	}
}

func TestConflictSuggestionsNameTheMergeDriver(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := goConflict(t)
	fx.WriteFile(".gitattributes", "*.go merge=mergiraf\n")
	fx.Git("config", "merge.mergiraf.driver", "mergiraf merge --git %O %A %B -s %S -x %X -y %Y -p %P")
	// A merge driver runs a program, so the repository has to be trusted first.
	if err := a.TrustRepo(fx.Dir); err != nil {
		t.Fatal(err)
	}
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.ConflictSuggestions(tab.Path, "forecast.go", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Driver != "mergiraf" || !strings.HasPrefix(got.DriverCommand, "mergiraf merge") {
		t.Errorf("driver %q, command %q", got.Driver, got.DriverCommand)
	}
}

func TestConflictSuggestionsSkipFilesNotReadAsCode(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.WithConflict(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.ConflictSuggestions(tab.Path, "conflict.txt", []ConflictPlace{{Ours: "ours\n", Base: "base\n", Theirs: "theirs\n", HasBase: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "" || len(got.Suggestions) != 0 {
		t.Errorf("%+v", got)
	}
}
