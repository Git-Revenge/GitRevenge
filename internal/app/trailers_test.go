package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
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
