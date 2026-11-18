package app

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// What git said about one remote whose repository GitHub no longer has (#806).
const goneOnGitHub = "ERROR: Repository not found.\n" +
	"fatal: Could not read from remote repository.\n\n" +
	"Please make sure you have the correct access rights\n" +
	"and the repository exists.\n"

func missedRemote(name, output string) git.FetchFailure {
	return git.FetchFailure{Remote: name, Output: output}
}

func partFetchedProblem(t *testing.T, missed []git.FetchFailure, reached int) *Problem {
	t.Helper()
	var pe *problemErr
	if err := partFetched(&git.Error{Stderr: "git's words\n"}, missed, reached); !errors.As(err, &pe) {
		t.Fatalf("partFetched = %v (%T), want a Problem", err, err)
	}
	return pe.p
}

func TestPartFetchedNamesTheRemotesItMissed(t *testing.T) {
	t.Parallel()
	const unreachable = "fatal: unable to access 'https://example.org/a.git/': Could not resolve host: example.org\n"
	var seven []git.FetchFailure
	for _, n := range []string{"ghsa-3r48", "ghsa-8crh", "ghsa-m5wc", "ghsa-qvcv", "ghsa-r6gg", "ghsa-w5mm", "ghsa-xv2q"} {
		seven = append(seven, missedRemote(n, goneOnGitHub))
	}
	for _, c := range []struct {
		name          string
		missed        []git.FetchFailure
		reached       int
		title, detail string
		actions       []ProblemAction
	}{
		{
			name: "several gone", missed: seven, reached: 1,
			title:  "Fetch couldn't find 7 of your 8 remotes",
			detail: "No repository was found for ghsa-3r48, ghsa-8crh, ghsa-m5wc and 4 more. They may have been deleted, or your account can't see them. Your other remote was fetched.",
			actions: []ProblemAction{{Kind: "remove-remotes", Label: "Remove them…",
				Names: []string{"ghsa-3r48", "ghsa-8crh", "ghsa-m5wc", "ghsa-qvcv", "ghsa-r6gg", "ghsa-w5mm", "ghsa-xv2q"}}},
		},
		{
			name: "one gone", missed: seven[:1], reached: 2,
			title:   "Fetch couldn't find the remote ghsa-3r48",
			detail:  "No repository was found at its URL. It may have been deleted, or your account can't see it. Your other 2 remotes were fetched.",
			actions: []ProblemAction{{Kind: "remove-remotes", Label: "Remove ghsa-3r48…", Names: []string{"ghsa-3r48"}}},
		},
		{
			// A remote that's only out of reach is nothing to remove.
			name: "one unreachable", missed: []git.FetchFailure{missedRemote("mirror", unreachable)}, reached: 1,
			title:   "Fetch couldn't reach the remote mirror",
			detail:  "Couldn't reach example.org. Your other remote was fetched.",
			actions: []ProblemAction{{Kind: "retry", Label: "Try again"}},
		},
		{
			name: "one for a reason the app doesn't know", missed: []git.FetchFailure{missedRemote("mirror", "fatal: something new\n")}, reached: 1,
			title:   "Fetch couldn't reach the remote mirror",
			detail:  "Your other remote was fetched.",
			actions: []ProblemAction{{Kind: "retry", Label: "Try again"}},
		},
		{
			name: "gone and unreachable", reached: 3,
			missed: []git.FetchFailure{seven[0], missedRemote("mirror", unreachable), seven[1], missedRemote("backup", unreachable)},
			title:  "Fetch couldn't reach 4 of your 7 remotes",
			detail: "No repository was found for ghsa-3r48 and ghsa-8crh. They may have been deleted, or your account can't see them. mirror and backup weren't fetched: Details has what git said. Your other 3 remotes were fetched.",
			actions: []ProblemAction{
				{Kind: "remove-remotes", Label: "Remove the 2 not found…", Names: []string{"ghsa-3r48", "ghsa-8crh"}},
				{Kind: "retry", Label: "Try again"},
			},
		},
	} {
		p := partFetchedProblem(t, c.missed, c.reached)
		if p.Title != c.title {
			t.Errorf("%s: Title = %q, want %q", c.name, p.Title, c.title)
		}
		if p.Detail != c.detail {
			t.Errorf("%s: Detail = %q, want %q", c.name, p.Detail, c.detail)
		}
		if !reflect.DeepEqual(p.Actions, c.actions) {
			t.Errorf("%s: Actions = %+v, want %+v", c.name, p.Actions, c.actions)
		}
		if p.Output != "git's words" || p.Full != "git's words" {
			t.Errorf("%s: Output = %q, Full = %q, want git's words", c.name, p.Output, p.Full)
		}
	}
}

func TestSomeOf(t *testing.T) {
	t.Parallel()
	for want, names := range map[string][]string{
		"":                   nil,
		"a":                  {"a"},
		"a and b":            {"a", "b"},
		"a, b and c":         {"a", "b", "c"},
		"a, b, c and 1 more": {"a", "b", "c", "d"},
	} {
		if got := someOf(names); got != want {
			t.Errorf("someOf(%v) = %q, want %q", names, got, want)
		}
	}
}

// A fetch of every remote, some of which are gone, still fetches the rest and says which
// it missed, however git was told to take them (#806).
func TestFetchWithRemotesThatAreGone(t *testing.T) {
	for _, parallel := range []string{"1", "0"} {
		t.Run("fetch.parallel="+parallel, func(t *testing.T) {
			isolateGit(t)
			a := newTestApp(t)
			local, remote := gittest.WithRemote(t, 1)
			other := gittest.Clone(t, remote)
			other.CommitFile("theirs.txt", "t", "their work")
			other.Git("push", "-q", "origin", "main")
			nowhere := t.TempDir()
			local.Git("remote", "add", "ghsa-one", filepath.Join(nowhere, "one.git"))
			local.Git("remote", "add", "ghsa-two", filepath.Join(nowhere, "two.git"))
			local.Git("config", "fetch.parallel", parallel)
			tab, err := a.OpenRepo(local.Dir)
			if err != nil {
				t.Fatal(err)
			}

			err = a.Fetch(tab.Path, "")
			if err == nil {
				t.Fatal("Fetch said nothing about the remotes it couldn't find")
			}
			if got := local.Git("rev-parse", "origin/main"); got != other.Head() {
				t.Errorf("origin/main = %s, want %s: the remote that's there wasn't fetched", got, other.Head())
			}
			p := a.TakeProblem(err.Error())
			if p == nil {
				t.Fatalf("err = %v, want a Problem", err)
			}
			if want := "Fetch couldn't find 2 of your 3 remotes"; p.Title != want {
				t.Errorf("Title = %q, want %q", p.Title, want)
			}
			if want := "No repository was found for ghsa-one and ghsa-two. They may have been deleted, or your account can't see them. Your other remote was fetched."; p.Detail != want {
				t.Errorf("Detail = %q, want %q", p.Detail, want)
			}
			want := []ProblemAction{{Kind: "remove-remotes", Label: "Remove them…", Names: []string{"ghsa-one", "ghsa-two"}}}
			if !reflect.DeepEqual(p.Actions, want) {
				t.Errorf("Actions = %+v, want %+v", p.Actions, want)
			}
			if !strings.Contains(p.Output, "does not appear to be a git repository") {
				t.Errorf("Output = %q, want git's words", p.Output)
			}

			// Once they're removed, as the banner offers, the fetch has nothing to say.
			for _, name := range p.Actions[0].Names {
				if err := a.RemoteAction(tab.Path, "remove", []string{name}); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.Fetch(tab.Path, ""); err != nil {
				t.Errorf("Fetch after removing them: %v", err)
			}
		})
	}
}

// With nothing fetched there's no "rest were fetched" to report: it's the failure it was.
func TestFetchWithEveryRemoteGone(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local := gittest.New(t)
	local.Linear(1)
	nowhere := t.TempDir()
	local.Git("remote", "add", "one", filepath.Join(nowhere, "one.git"))
	local.Git("remote", "add", "two", filepath.Join(nowhere, "two.git"))
	// A remote kept out of fetches wasn't asked, so it wasn't fetched either.
	local.Git("remote", "add", "parked", filepath.Join(nowhere, "parked.git"))
	local.Git("config", "remote.parked.skipFetchAll", "true")
	tab, _ := a.OpenRepo(local.Dir)

	err := a.Fetch(tab.Path, "")
	if err == nil || !strings.HasPrefix(err.Error(), "Fetch failed: no repository was found at that URL") {
		t.Fatalf("err = %v", err)
	}
}

// Notes still come in from the remotes that answered.
func TestFetchSyncsNotesFromTheRemotesItReached(t *testing.T) {
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
	bob.Git("remote", "add", "gone", filepath.Join(t.TempDir(), "gone.git"))
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

	err := a.Fetch(tb.Path, "")
	if err == nil || !strings.HasPrefix(err.Error(), "Fetch couldn't find the remote gone") {
		t.Fatalf("err = %v", err)
	}
	if notes, _ := a.CommitNotes(tb.Path, head); len(notes) != 1 || notes[0].Text != "reviewed" {
		t.Errorf("bob's notes after fetch = %+v, want the one on origin", notes)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(notices) != 0 {
		t.Errorf("notices = %v, want none: the remote that's gone isn't asked for notes", notices)
	}
}

// A group's fetch says the same in the repository's row.
func TestGroupFetchWithARemoteThatIsGone(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	other := gittest.Clone(t, remote)
	other.CommitFile("theirs.txt", "t", "their work")
	other.Git("push", "-q", "origin", "main")
	local.Git("remote", "add", "gone", filepath.Join(t.TempDir(), "gone.git"))
	_ = a.GroupAction("create", []string{"g"})
	if err := a.GroupAction("add", []string{"g", local.Dir}); err != nil {
		t.Fatal(err)
	}

	results, err := a.GroupBatch("g", "fetch")
	if err != nil || len(results) != 1 {
		t.Fatalf("results = %+v, %v", results, err)
	}
	want := "Fetch couldn't find the remote gone. No repository was found at its URL. It may have been deleted, or your account can't see it. Your other remote was fetched."
	if results[0].OK || results[0].Message != want {
		t.Errorf("result = %+v, want the message %q", results[0], want)
	}
	if got := local.Git("rev-parse", "origin/main"); got != other.Head() {
		t.Errorf("origin/main = %s, want %s", got, other.Head())
	}
}
