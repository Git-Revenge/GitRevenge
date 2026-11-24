package app

import (
	"reflect"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestConflictStopTalliesSuggestionsAndWhatADriverSettled(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := goBranches(t, func(r *gittest.Repo) {
		// A merge driver that takes theirs whole, for the .txt files.
		r.Git("config", "merge.keeptheirs.driver", "cp %B %A")
		r.CommitFile(".gitattributes", "*.txt merge=keeptheirs\n", "attributes")
		r.CommitFile("notes.txt", "one\n", "notes")
		r.CommitFile("only.txt", "one\n", "only")
	})
	fx.Checkout("theirs")
	fx.CommitFile("notes.txt", "one\ntheirs\n", "their notes")
	fx.CommitFile("only.txt", "one\ntheirs\n", "only theirs")
	fx.Checkout("main")
	fx.CommitFile("notes.txt", "ours\none\n", "our notes")
	if err := fx.Merge("theirs"); err == nil {
		t.Fatal("expected a conflict")
	}
	// A merge driver runs a program, so the repository has to be trusted first.
	if err := a.TrustRepo(fx.Dir); err != nil {
		t.Fatal(err)
	}
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := a.ConflictStop(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []ConflictTally{{Path: "forecast.go", Places: 1, Suggested: 1, Language: "Go"}}; !reflect.DeepEqual(st.Files, want) {
		t.Errorf("files %+v", st.Files)
	}
	// only.txt changed on one side, so git never ran the driver on it.
	if want := []DriverSettled{{Path: "notes.txt", Driver: "keeptheirs"}}; !reflect.DeepEqual(st.Settled, want) {
		t.Errorf("settled %+v", st.Settled)
	}
	if want := []MergeDriver{{Name: "keeptheirs", Command: "cp %B %A"}}; !reflect.DeepEqual(st.Drivers, want) {
		t.Errorf("drivers %+v", st.Drivers)
	}
}

func TestConflictStopIsEmptyWithNothingInProgress(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := goBranches(t, nil)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := a.ConflictStop(tab.Path)
	if err != nil || len(st.Files) != 0 || len(st.Settled) != 0 {
		t.Fatalf("%+v, %v", st, err)
	}
}

func TestPreviewMergeSaysWhichPlacesLookCombinable(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := goBranches(t, nil)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.PreviewMerge(tab.Path, "theirs")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Conflicts) != 1 || p.Conflicts[0].Places != 1 || p.Conflicts[0].Combinable != 1 {
		t.Fatalf("conflicts %+v", p.Conflicts)
	}
	r, err := a.PreviewRebase(tab.Path, "theirs")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Conflicts) != 1 || r.Conflicts[0].Combinable != 1 {
		t.Fatalf("rebase conflicts %+v", r.Conflicts)
	}
}
