package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// peopleRepo is a small history: Maya under one identity, Priya under two, a bot, and a
// co-author who never wrote a commit of her own.
func peopleRepo(t *testing.T) (*App, *gittest.Repo, string) {
	t.Helper()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1) // Test User
	commitAs(fx, "Maya Okafor", "maya@tidewater.example", "m1.txt", "one")
	commitAs(fx, "Maya Okafor", "maya@tidewater.example", "m2.txt", "two", "Co-authored-by: Sarah Lind <sarah@tidewater.example>")
	commitAs(fx, "Maya Okafor", "maya@tidewater.example", "m3.txt", "three")
	commitAs(fx, "Priya Raman", "priya@tidewater.example", "p1.txt", "four")
	commitAs(fx, "Priya R", "priya@oldmill.example", "p2.txt", "five")
	commitAs(fx, "dependabot[bot]", "bot@users.noreply.example", "d1.txt", "Bump vite")
	return a, fx, openForMap(t, a, fx)
}

func person(t *testing.T, list PeopleList, name string) Person {
	t.Helper()
	for _, p := range list.People {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no %q among %+v", name, list.People)
	return Person{}
}

func TestPeopleWithNoMailmap(t *testing.T) {
	t.Parallel()
	a, fx, path := peopleRepo(t)

	list, err := a.People(path, PeopleRequest{})
	if err != nil {
		t.Fatal(err)
	}
	// Everyone is who their commits say: six identities, six people, and no file is made.
	if list.Everyone != 6 || list.Identities != 6 || list.Bots != 1 || list.Commits != 7 || list.Matching != 6 {
		t.Fatalf("totals = %+v", list)
	}
	if list.People[0].Name != "Maya Okafor" || list.People[0].Commits != 3 {
		t.Fatalf("most commits first: %+v", list.People[0])
	}
	if sarah := person(t, list, "Sarah Lind"); sarah.Commits != 0 || sarah.CoAuthored != 1 {
		t.Errorf("a co-author with no commits of her own = %+v", sarah)
	}
	if !person(t, list, "dependabot[bot]").Bot || person(t, list, "Maya Okafor").Bot {
		t.Error("the bot mark is on the wrong person")
	}
	if priya := person(t, list, "Priya R"); len(priya.Identities) != 1 || len(priya.Identities[0].Lines) != 0 {
		t.Errorf("an identity nothing maps = %+v", priya)
	}
	if _, err := os.Stat(filepath.Join(fx.Dir, ".mailmap")); err == nil {
		t.Fatal("reading people created a .mailmap")
	}
}

func TestPeopleReadThroughUnsavedMailmapText(t *testing.T) {
	t.Parallel()
	a, fx, path := peopleRepo(t)
	edited := MailmapText{Text: "Priya Raman <priya@tidewater.example> <priya@oldmill.example>\n", Edited: true}

	list, err := a.People(path, PeopleRequest{Mailmap: edited})
	if err != nil {
		t.Fatal(err)
	}
	if list.Everyone != 5 || list.Identities != 6 {
		t.Fatalf("with the edit: %d people from %d identities, want 5 from 6", list.Everyone, list.Identities)
	}
	priya := person(t, list, "Priya Raman")
	if priya.Commits != 2 || len(priya.Identities) != 2 {
		t.Fatalf("priya = %+v, want both identities under her", priya)
	}
	var joined PersonIdentity
	for _, id := range priya.Identities {
		if id.Email == "priya@oldmill.example" {
			joined = id
		}
	}
	if joined.Name != "Priya R" || joined.Commits != 1 || len(joined.Lines) != 1 || joined.Lines[0] != (git.MailmapRef{Source: git.MailmapWorkTree, Line: 1}) {
		t.Fatalf("the joined identity = %+v, want it joined by line 1 of .mailmap", joined)
	}
	// Nothing was saved: asking without the edit shows them apart again, and there's no file.
	list, err = a.People(path, PeopleRequest{})
	if err != nil || list.Everyone != 6 {
		t.Fatalf("without the edit: %d people, %v", list.Everyone, err)
	}
	if _, err := os.Stat(filepath.Join(fx.Dir, ".mailmap")); err == nil {
		t.Fatal("an unsaved edit created a .mailmap")
	}
}

func TestPeopleFilterAndPages(t *testing.T) {
	t.Parallel()
	a, _, path := peopleRepo(t)
	edited := MailmapText{Text: "Priya Raman <priya@tidewater.example> <priya@oldmill.example>\n", Edited: true}

	// A person is found by a name or address a commit has, not only the one shown.
	list, err := a.People(path, PeopleRequest{Mailmap: edited, Query: "OLDMILL"})
	if err != nil {
		t.Fatal(err)
	}
	if list.Matching != 1 || len(list.People) != 1 || list.People[0].Name != "Priya Raman" || list.Everyone != 5 {
		t.Fatalf("filtered = %+v", list)
	}
	if list, _ := a.People(path, PeopleRequest{Query: "nobody here"}); list.Matching != 0 || len(list.People) != 0 {
		t.Fatalf("a filter that matches nobody = %+v", list)
	}

	first, err := a.People(path, PeopleRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.People(path, PeopleRequest{Offset: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	past, err := a.People(path, PeopleRequest{Offset: 50, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.People) != 2 || len(second.People) != 2 || len(past.People) != 0 || first.Matching != 6 || first.People[0].Name == second.People[0].Name {
		t.Fatalf("pages: %d, %d, %d people; matching %d", len(first.People), len(second.People), len(past.People), first.Matching)
	}
}

func TestMailmapNotesForTextBeingEdited(t *testing.T) {
	t.Parallel()
	a, fx, path := peopleRepo(t)
	text := "# Tidewater\nPriya Raman <priya@tidewater.example> <priya@oldmill.example>\nJo Whitlock <jo@tidewater.example> <jo@harbour.example>\nLeo Brandt leo@tidewater.example\n"

	notes, err := a.MailmapNotes(path, MailmapText{Text: text, Edited: true}, git.MailmapWorkTree)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 4 {
		t.Fatalf("notes = %+v", notes)
	}
	if n := notes[1]; n.Kind != git.MailmapMapping || n.Identities != 1 || n.Commits != 1 {
		t.Errorf("the line that joins Priya = %+v", n)
	}
	if n := notes[2]; n.Kind != git.MailmapMapping || n.Identities != 0 || n.Replaced {
		t.Errorf("a line that matches nobody = %+v", n)
	}
	if n := notes[3]; n.Kind != git.MailmapSkipped || n.Why == "" {
		t.Errorf("a line git skips = %+v", n)
	}
	// With nothing being edited and no file saved, there are no lines to note.
	if notes, err := a.MailmapNotes(path, MailmapText{}, git.MailmapWorkTree); err != nil || notes == nil || len(notes) != 0 {
		t.Fatalf("no text gave %v, %v", notes, err)
	}
	// A source that's only read is noted from what's saved. mailmap.file is read last, so
	// its lines still decide the names, beside the edit that decides Priya's address.
	fx.WriteFile("mine.txt", "P. Raman <priya@oldmill.example>\nMaya O. <maya@tidewater.example>\n")
	fx.Git("config", "mailmap.file", "mine.txt")
	notes, err = a.MailmapNotes(path, MailmapText{Text: text, Edited: true}, git.MailmapFile)
	if err != nil || len(notes) != 2 {
		t.Fatalf("notes for mailmap.file = %+v, %v", notes, err)
	}
	if notes[0].Identities != 1 || notes[1].Identities != 1 || notes[1].Commits != 3 {
		t.Errorf("mailmap.file's lines = %+v", notes)
	}
}

func TestWhoIs(t *testing.T) {
	t.Parallel()
	a, _, path := peopleRepo(t)
	edited := MailmapText{Text: "Priya Raman <priya@tidewater.example> <priya@oldmill.example>\n", Edited: true}

	// An address a commit has.
	got, err := a.WhoIs(path, edited, "PRIYA@oldmill.example")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Known || got[0].Written.Name != "Priya R" || got[0].Shown != (git.Identity{Name: "Priya Raman", Email: "priya@tidewater.example"}) || got[0].Commits != 1 || len(got[0].Lines) != 1 || got[0].Lines[0].Line != 1 {
		t.Fatalf("by address = %+v", got)
	}
	// A name, and a name with its address.
	if got, _ := a.WhoIs(path, edited, "maya okafor"); len(got) != 1 || got[0].Shown.Name != "Maya Okafor" || len(got[0].Lines) != 0 || got[0].Commits != 3 {
		t.Fatalf("by name = %+v", got)
	}
	if got, _ := a.WhoIs(path, edited, "Priya R <priya@oldmill.example>"); len(got) != 1 || got[0].Shown.Name != "Priya Raman" {
		t.Fatalf("by name and address = %+v", got)
	}
	// An address no commit has is still answered, since a line can be written for it.
	got, _ = a.WhoIs(path, MailmapText{Text: "Jo Whitlock <jo@tidewater.example> <jo@harbour.example>\n", Edited: true}, "jo@harbour.example")
	if len(got) != 1 || got[0].Known || got[0].Shown.Name != "Jo Whitlock" || got[0].Commits != 0 {
		t.Fatalf("an address the history doesn't use = %+v", got)
	}
	// A name nobody has, and nothing at all.
	if got, _ := a.WhoIs(path, edited, "Nobody At All"); len(got) != 0 {
		t.Fatalf("an unknown name = %+v", got)
	}
	if got, err := a.WhoIs(path, edited, "   "); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("a blank question = %+v, %v", got, err)
	}
}

func TestSaveMailmapAsksBeforeCreating(t *testing.T) {
	t.Parallel()
	a, fx, path := peopleRepo(t)
	p := filepath.Join(fx.Dir, ".mailmap")
	line := "Priya Raman <priya@tidewater.example> <priya@oldmill.example>\n"

	err := a.SaveMailmap(path, "", line, false)
	if err == nil || err.Error() != "This repository has no .mailmap, and one is only created when you say so. Nothing was written." {
		t.Fatalf("without permission: %v", err)
	}
	if _, statErr := os.Stat(p); statErr == nil {
		t.Fatal("a .mailmap was created without permission")
	}
	if err := a.SaveMailmap(path, "", line, true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(p); string(data) != line {
		t.Fatalf("the new .mailmap holds %q", data)
	}
	// Saved, so the list reads through it with no text sent.
	list, err := a.People(path, PeopleRequest{})
	if err != nil || list.Everyone != 5 {
		t.Fatalf("after saving: %d people, %v", list.Everyone, err)
	}
	sources, err := a.MailmapSources(path)
	if err != nil || len(sources) != 1 || !sources[0].Exists || sources[0].Lines != 1 {
		t.Fatalf("sources = %+v, %v", sources, err)
	}
	// A stale base is refused in words a person can act on.
	err = a.SaveMailmap(path, "something else\n", line, false)
	if err == nil || err.Error() != "The .mailmap changed on disk since you started editing, so it wasn't saved. Reload it to see the change, then make your edits again." {
		t.Fatalf("with a stale base: %v", err)
	}
}

// The history is read once for a state of the refs, however many things ask.
func TestPeopleKeepTheHistoryUntilARefMoves(t *testing.T) {
	t.Parallel()
	a, fx, path := peopleRepo(t)
	if _, err := a.People(path, PeopleRequest{}); err != nil {
		t.Fatal(err)
	}
	a.people.mu.Lock()
	first := a.people.kept[path]
	a.people.mu.Unlock()
	if first == nil {
		t.Fatal("nothing was kept")
	}
	if _, err := a.WhoIs(path, MailmapText{}, "maya okafor"); err != nil {
		t.Fatal(err)
	}
	a.people.mu.Lock()
	again := a.people.kept[path]
	a.people.mu.Unlock()
	if again != first {
		t.Fatal("the history was read again with no ref moved")
	}

	commitAs(fx, "New Person", "new@tidewater.example", "n1.txt", "eight")
	list, err := a.People(path, PeopleRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if list.Everyone != 7 || list.Commits != 8 {
		t.Fatalf("after a new commit: %d people, %d commits", list.Everyone, list.Commits)
	}
}
