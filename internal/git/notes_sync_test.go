package git

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestNotesSync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	alice, remote := gittest.WithRemote(t, 2)
	bob := gittest.Clone(t, remote)
	a := OpenRepo(newTestRunner(t), alice.Dir)
	b := OpenRepo(newTestRunner(t), bob.Dir)
	head := alice.Head()
	parent := alice.Git("rev-parse", "HEAD~1")

	// Alice writes a note and pushes notes; Bob fetches and gets it.
	if err := a.SetNote(ctx, DefaultNotesRef, head, "reviewed by alice"); err != nil {
		t.Fatal(err)
	}
	if err := a.PushNotes(ctx, "origin", NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := b.FetchNotes(ctx, "origin", NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if d, err := b.IntegrateNotes(ctx, "origin"); err != nil || len(d) != 0 {
		t.Fatalf("integrate = %v, %v", d, err)
	}
	if n, _ := b.Notes(ctx, head); len(n) != 1 || n[0].Text != "reviewed by alice" {
		t.Fatalf("bob's notes = %+v", n)
	}

	// Bob adds to them and pushes; Alice fast-forwards.
	if err := b.SetNote(ctx, DefaultNotesRef, parent, "bob was here"); err != nil {
		t.Fatal(err)
	}
	if err := b.PushNotes(ctx, "origin", NetOptions{}); err != nil {
		t.Fatal(err)
	}
	_ = a.FetchNotes(ctx, "origin", NetOptions{})
	if d, _ := a.IntegrateNotes(ctx, "origin"); len(d) != 0 {
		t.Fatalf("alice diverged: %v", d)
	}
	if n, _ := a.Notes(ctx, parent); len(n) != 1 {
		t.Fatalf("alice didn't fast-forward: %+v", n)
	}

	// Both write without syncing: diverged, left alone, and Bob's push is refused.
	_ = a.SetNote(ctx, DefaultNotesRef, head, "alice again")
	_ = a.PushNotes(ctx, "origin", NetOptions{})
	_ = b.SetNote(ctx, DefaultNotesRef, head, "bob again")
	before := bob.Git("rev-parse", DefaultNotesRef)
	_ = b.FetchNotes(ctx, "origin", NetOptions{})
	d, err := b.IntegrateNotes(ctx, "origin")
	if err != nil || !slices.Equal(d, []string{DefaultNotesRef}) {
		t.Fatalf("diverged = %v, %v", d, err)
	}
	if bob.Git("rev-parse", DefaultNotesRef) != before {
		t.Fatal("a diverged notes ref was moved")
	}
	var gerr *Error
	if err := b.PushNotes(ctx, "origin", NetOptions{}); !errors.As(err, &gerr) {
		t.Fatalf("push of diverged notes = %v, want a rejection", err)
	}
}

func TestNotesSyncWithoutNotes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	local, _ := gittest.WithRemote(t, 1)
	r := OpenRepo(newTestRunner(t), local.Dir)
	if err := r.FetchNotes(ctx, "origin", NetOptions{}); err != nil {
		t.Fatalf("fetch with no notes anywhere: %v", err)
	}
	if d, err := r.IntegrateNotes(ctx, "origin"); err != nil || len(d) != 0 {
		t.Fatalf("integrate = %v, %v", d, err)
	}
	if err := r.PushNotes(ctx, "origin", NetOptions{}); err != nil {
		t.Fatalf("push with no notes: %v", err)
	}
}
