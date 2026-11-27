package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// conflicting has main and side changing f.txt and h.txt in the same line: merging side
// conflicts in both.
func conflicting(t *testing.T) *gittest.Repo {
	t.Helper()
	fx := gittest.New(t)
	fx.WriteFile("f.txt", "a\nb\nc\n")
	fx.WriteFile("h.txt", "x\n")
	fx.Commit("base")
	fx.Branch("side")
	fx.WriteFile("f.txt", "a\nSIDE\nc\n")
	fx.WriteFile("h.txt", "side\n")
	fx.Commit("side")
	fx.Checkout("main")
	fx.WriteFile("f.txt", "a\nMAIN\nc\n")
	fx.WriteFile("h.txt", "main\n")
	fx.Commit("main")
	return fx
}

func TestConflictContextSaysWhatRerereResolved(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := conflicting(t)
	fx.Git("config", "rerere.enabled", "true")
	fx.Git("config", "rerere.autoUpdate", "true")
	path := openForMap(t, a, fx)
	_ = fx.Merge("side")
	fx.WriteFile("f.txt", "a\nBOTH\nc\n")
	fx.Git("rerere")
	fx.Git("merge", "--abort")

	_ = fx.Merge("side")
	got, err := a.ConflictContext(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Rerere.On || !slices.Equal(got.Rerere.Resolved, []string{"f.txt"}) || len(got.Rerere.Recurring) != 0 {
		t.Fatalf("rerere = %+v", got.Rerere)
	}
	// Staged by rerere, it isn't listed as merged cleanly.
	if slices.Contains(got.Clean, "f.txt") {
		t.Errorf("clean = %v", got.Clean)
	}

	if err := a.ForgetResolution(path, "f.txt"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(fx.Dir, "f.txt")); !strings.Contains(string(b), "<<<<<<<") {
		t.Errorf("f.txt = %q", b)
	}
	if got, _ = a.ConflictContext(path); len(got.Rerere.Resolved) != 0 {
		t.Errorf("after forgetting: %+v", got.Rerere)
	}
}

// With rerere off, a conflict resolved in the app before is noticed coming back, and the offer
// to turn rerere on is made once.
func TestConflictContextOffersRerereForAConflictThatCameBack(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := conflicting(t)
	path := openForMap(t, a, fx)
	_ = fx.Merge("side")
	if got, _ := a.ConflictContext(path); got.Rerere.On || len(got.Rerere.Recurring) != 0 {
		t.Fatalf("the first time: %+v", got.Rerere)
	}
	if err := a.TakeConflictSide(path, "h.txt", "ours"); err != nil {
		t.Fatal(err)
	}
	if err := a.ResolveConflict(path, "f.txt", "a\nBOTH\nc\n"); err != nil {
		t.Fatal(err)
	}
	fx.Git("merge", "--abort")

	_ = fx.Merge("side")
	got, err := a.ConflictContext(path)
	if err != nil || !slices.Equal(got.Rerere.Recurring, []string{"f.txt", "h.txt"}) {
		t.Fatalf("again: %+v, %v", got.Rerere, err)
	}
	if err := a.DeclineRerere(path); err != nil {
		t.Fatal(err)
	}
	if got, _ = a.ConflictContext(path); len(got.Rerere.Recurring) != 0 {
		t.Errorf("after declining: %+v", got.Rerere)
	}
	if err := a.TurnOnRerere(path); err != nil {
		t.Fatal(err)
	}
	if v := fx.Git("config", "rerere.enabled"); strings.TrimSpace(v) != "true" {
		t.Errorf("rerere.enabled = %q", v)
	}
	// Turned on mid-conflict, the conflicts still open are recorded, to be reused once resolved.
	if st := strings.Fields(fx.Git("rerere", "status")); !slices.Equal(st, []string{"f.txt", "h.txt"}) {
		t.Errorf("rerere status = %v", st)
	}
}
