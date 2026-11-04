package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestSyncNotes(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	var mu sync.Mutex
	var notices []string
	a.emitHook = func(name string, data any) {
		if n, ok := data.(Notice); ok && name == "notice" {
			mu.Lock()
			notices = append(notices, n.Message)
			mu.Unlock()
		}
	}
	alice, remote := gittest.WithRemote(t, 1)
	bob := gittest.Clone(t, remote)
	ta, _ := a.OpenRepo(alice.Dir)
	tb, _ := a.OpenRepo(bob.Dir)
	for _, path := range []string{ta.Path, tb.Path} {
		p, _ := a.RepoPreferences(path)
		p.SyncNotes = true
		if err := a.SetRepoPreferences(path, p); err != nil {
			t.Fatal(err)
		}
	}
	head := alice.Head()

	if err := a.SetCommitNote(ta.Path, head, "reviewed"); err != nil {
		t.Fatal(err)
	}
	if err := a.PushCurrent(ta.Path, false); err != nil {
		t.Fatal(err)
	}
	if err := a.Fetch(tb.Path, ""); err != nil {
		t.Fatal(err)
	}
	notes, _ := a.CommitNotes(tb.Path, head)
	if len(notes) != 1 || notes[0].Text != "reviewed" {
		t.Fatalf("bob's notes after fetch = %+v", notes)
	}

	// Diverged notes: a pull leaves them and says so.
	_ = a.SetCommitNote(ta.Path, head, "alice's take")
	_ = a.PushCurrent(ta.Path, false)
	_ = a.SetCommitNote(tb.Path, head, "bob's take")
	if err := a.Pull(tb.Path, ""); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.CommitNotes(tb.Path, head); n[0].Text != "bob's take" {
		t.Fatalf("bob's diverged note was overwritten: %+v", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(notices) == 0 || !strings.Contains(notices[len(notices)-1], "diverged") {
		t.Fatalf("notices = %v", notices)
	}
}

func TestSyncNotesOffByDefault(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	alice, remote := gittest.WithRemote(t, 1)
	tab, _ := a.OpenRepo(alice.Dir)
	_ = a.SetCommitNote(tab.Path, alice.Head(), "local only")
	if err := a.PushCurrent(tab.Path, false); err != nil {
		t.Fatal(err)
	}
	if out, _ := remote.TryGit("for-each-ref", "refs/notes/"); strings.TrimSpace(out) != "" {
		t.Fatalf("notes were pushed with the setting off: %s", out)
	}
}
