package git

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestNotes(t *testing.T) {
	t.Parallel()
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

func TestNotedCommits(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	shas := fx.Linear(3)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if ids, err := repo.NotedCommits(ctx); err != nil || len(ids) != 0 {
		t.Fatalf("no notes refs = %v, %v", ids, err)
	}
	// One commit noted in two refs, another in one: each is listed once.
	fx.Git("notes", "add", "-m", "deployed", shas[0])
	fx.Git("notes", "--ref=review", "add", "-m", "looks good", shas[0])
	fx.Git("notes", "--ref=review", "add", "-m", "needs a test", shas[2])
	ids, err := repo.NotedCommits(ctx)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(ids)
	want := []string{shas[0], shas[2]}
	slices.Sort(want)
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("noted = %v, want %v", ids, want)
	}
	// A note taken away takes its commit off the list.
	fx.Git("notes", "--ref=review", "remove", shas[2])
	if ids, _ := repo.NotedCommits(ctx); !reflect.DeepEqual(ids, []string{shas[0]}) {
		t.Fatalf("after removing = %v", ids)
	}
}
