package git

import (
	"context"
	"reflect"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestNotes(t *testing.T) {
	fx := gittest.New(t)
	shas := fx.Linear(2)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if notes, err := repo.Notes(ctx, shas[1]); err != nil || len(notes) != 0 {
		t.Fatalf("no notes = %v, %v", notes, err)
	}
	fx.Git("notes", "--ref=review", "add", "-m", "looks good", shas[1])
	if err := repo.SetNote(ctx, DefaultNotesRef, shas[1], "Deployed to staging.\nTwice."); err != nil {
		t.Fatal(err)
	}
	notes, err := repo.Notes(ctx, shas[1])
	want := []Note{{DefaultNotesRef, "Deployed to staging.\nTwice."}, {"refs/notes/review", "looks good"}}
	if err != nil || !reflect.DeepEqual(notes, want) {
		t.Fatalf("notes = %+v, %v; want %+v", notes, err, want)
	}
	// The other commit has no notes in either ref.
	if notes, _ := repo.Notes(ctx, shas[0]); len(notes) != 0 {
		t.Fatalf("other commit notes = %+v", notes)
	}

	if err := repo.SetNote(ctx, "commits", shas[1], "  "); err != nil {
		t.Fatal(err)
	}
	if notes, _ := repo.Notes(ctx, shas[1]); len(notes) != 1 || notes[0].Ref != "refs/notes/review" {
		t.Fatalf("after removing = %+v", notes)
	}
}
