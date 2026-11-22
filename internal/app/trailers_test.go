package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/links"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// trailersRead waits for the repository's trailers to have been read to the end.
func trailersRead(t *testing.T, a *App, path string) trailerindex.Status {
	t.Helper()
	var st TrailerProgress
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var err error
		if st, err = a.TrailerStatus(path); err != nil {
			t.Fatal(err)
		}
		if st.Done && !st.Reading {
			return st.Status
		}
	}
	t.Fatalf("the trailers were never read: %+v", st)
	return st.Status
}

func keyOf(t *testing.T, keys TrailerKeys, id string) trailerindex.Key {
	t.Helper()
	for _, k := range keys.Keys {
		if k.ID == id {
			return k
		}
	}
	t.Fatalf("no key %q among %+v", id, keys.Keys)
	return trailerindex.Key{}
}

// The trailers are read through what the repository says about itself: its mailmap for who
// a person is, its bot patterns, its smart links for what a ticket is, and its own commits
// for what an id names.
func TestTrailerKeysAreReadThroughTheRepositorysRules(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sarah Lind", "sarah@tidewater.example", "a.txt", "Round tide heights", "Reviewed-by: sarah lind <slind@users.noreply.example>", "Closes: TIDE-7")
	broke := fx.Head()
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Raise an alert on high wind", "Reviewed-By: Sarah Lind <sarah@tidewater.example>", "Closes: https://tracker.tidewater.example/browse/TIDE-7", "Fixes: "+broke[:10]+" (\"Round tide heights\")")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Bump vite", "Signed-off-by: dependabot[bot] <bot@users.noreply.example>", "Fixes: 0123456789 (\"not a commit here\")")
	fx.WriteFile(".mailmap", "Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>\n")
	path := openForMap(t, a, fx)
	if err := a.SaveLinkRule(path, links.Rule{Name: "Tide", Pattern: `\bTIDE-\d+\b`, URL: "https://tracker.tidewater.example/browse/{0}", In: []string{links.InTrailers}, Enabled: true, Scope: links.Local}, ""); err != nil {
		t.Fatal(err)
	}
	trailersRead(t, a, path)

	keys, err := a.TrailerKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if !keys.Status.Done {
		t.Fatalf("status = %+v", keys.Status)
	}
	// Two spellings of the key, and two of her: one key, one person.
	reviewed := keyOf(t, keys, "reviewed-by")
	if reviewed.Commits != 2 || reviewed.Values != 1 || reviewed.Kind != trailerindex.KindPerson || len(reviewed.Spellings) != 2 {
		t.Fatalf("reviewed-by = %+v", reviewed)
	}
	people, err := a.TrailerValues(path, "Reviewed-by", "", 0, 0)
	if err != nil || people.Matching != 1 || people.Values[0].Label != "Sarah Lind <sarah@tidewater.example>" || people.Values[0].Commits != 2 {
		t.Fatalf("who reviewed = %+v, %v", people, err)
	}
	// The ticket by its key and by its address is one ticket, shown by the shorter.
	closes, err := a.TrailerValues(path, "closes", "", 0, 0)
	if err != nil || closes.Matching != 1 || closes.Values[0].Label != "TIDE-7" || closes.Values[0].Commits != 2 || closes.Values[0].Kind != trailerindex.KindTicket {
		t.Fatalf("closes = %+v, %v", closes, err)
	}
	// An id that names a commit here is a commit; one that doesn't is only text.
	fixes, err := a.TrailerValues(path, "fixes", "", 0, 0)
	if err != nil || fixes.Matching != 2 {
		t.Fatalf("fixes = %+v, %v", fixes, err)
	}
	kinds := map[trailerindex.Kind]string{}
	for _, v := range fixes.Values {
		kinds[v.Kind] = v.ID
	}
	if kinds[trailerindex.KindCommit] != "commit:"+broke || kinds[trailerindex.KindText] == "" {
		t.Fatalf("fixes = %+v, want the real id as a commit and the other as text", fixes.Values)
	}
	if bot, err := a.TrailerValues(path, "signed-off-by", "", 0, 0); err != nil || len(bot.Values) != 1 || !bot.Values[0].Bot {
		t.Fatalf("signed-off-by = %+v, %v", bot, err)
	}
	// A query narrows the values, and a page is a page.
	if none, _ := a.TrailerValues(path, "reviewed-by", "nobody", 0, 0); none.Matching != 0 || none.Values == nil {
		t.Fatalf("a query nobody matches = %+v", none)
	}
	if page, _ := a.TrailerValues(path, "fixes", "", 1, 1); len(page.Values) != 1 || page.Matching != 2 {
		t.Fatalf("the second page of one = %+v", page)
	}

	// Take the mailmap away and she's two people again, without reading the history again.
	if err := os.Remove(filepath.Join(fx.Dir, ".mailmap")); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		people, err := a.TrailerValues(path, "reviewed-by", "", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if people.Matching == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("without the mailmap, who reviewed = %+v", people)
		}
	}
}

// Opening a repository reads its trailers in the background, into a file of the app's own
// and nothing in the repository.
func TestOpeningARepositoryReadsItsTrailers(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sarah Lind", "sarah@tidewater.example", "a.txt", "Round tide heights", "Signed-off-by: Sarah Lind <sarah@tidewater.example>")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Raise an alert on high wind", "Closes #12", "Reviewed-by: Sarah Lind <sarah@tidewater.example>")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Tidy the alert tests")
	before := fx.Git("status", "--porcelain", "--ignored")
	path := openForMap(t, a, fx)

	st := trailersRead(t, a, path)
	if st.Commits != 2 || st.Read != 3 {
		t.Fatalf("status = %+v, want 2 commits with trailers of 3 read", st)
	}
	r, err := a.openRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Trailers().Commits("Reviewed-by", "Sarah Lind <sarah@tidewater.example>"); len(got) != 1 {
		t.Fatalf("commits reviewed by Sarah: %v", got)
	}
	if keys := r.Trailers().Keys(); len(keys) != 3 {
		t.Fatalf("keys = %+v, want Signed-off-by, Closes and Reviewed-by", keys)
	}

	// The index is a file beside the test's settings, as it's in the cache folder for real.
	files, err := os.ReadDir(a.trailerDir())
	if err != nil || len(files) != 1 || !strings.HasSuffix(files[0].Name(), ".trailers") {
		t.Fatalf("index files = %v, %v", files, err)
	}
	// Nothing was written into the repository, its git folder included.
	if after := fx.Git("status", "--porcelain", "--ignored"); after != before {
		t.Fatalf("the work tree changed:\n%s", after)
	}
	_ = filepath.WalkDir(filepath.Join(fx.Dir, ".git"), func(p string, _ os.DirEntry, _ error) error {
		if strings.Contains(strings.ToLower(filepath.Base(p)), "trailer") {
			t.Errorf("%s is in the git folder", p)
		}
		return nil
	})

	// A commit made afterwards is read once the graph has caught up with it. The repository
	// is on screen, so it's watched, and the graph reloads on its own.
	commitAs(fx, "Sarah Lind", "sarah@tidewater.example", "d.txt", "Draft the tide table", "Refs: TIDE-7")
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if got := r.Trailers().Commits("Refs", "TIDE-7"); len(got) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the new commit's trailers were never read: %+v", r.Trailers().Status())
		}
	}
}
