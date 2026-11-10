package git

import (
	"context"
	"strings"
	"testing"
)

func TestConflictKeepsTheOriginalOfEachPlace(t *testing.T) {
	t.Parallel()
	fx := mergeInProgress(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	cf, err := repo.Conflict(context.Background(), "f.txt")
	if err != nil {
		t.Fatal(err)
	}
	// The file on disk has git's default markers, with no original between them.
	if strings.Contains(cf.Merged.Text, "|||||||") {
		t.Fatalf("the test expects the default conflict style:\n%s", cf.Merged.Text)
	}
	want := "one\n<<<<<<< ours\ntwo ours\n||||||| base\ntwo\n=======\ntwo theirs\n>>>>>>> theirs\nthree\n"
	if cf.Diff3 != want {
		t.Fatalf("diff3 =\n%s\nwant\n%s", cf.Diff3, want)
	}
}

func TestConflictHasNoDiff3WhenASideDeletedTheFile(t *testing.T) {
	t.Parallel()
	fx := conflictingBranches(t, 1)
	fx.Git("rm", "-q", "f.txt")
	fx.Git("commit", "-qm", "Delete f on main")
	if _, err := fx.TryGit("merge", "other"); err == nil {
		t.Fatal("expected a delete/modify conflict")
	}
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	cf, err := repo.Conflict(context.Background(), "f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if cf.Ours.Exists || !cf.Theirs.Exists || cf.Diff3 != "" {
		t.Fatalf("conflict = ours %v, theirs %v, diff3 %q", cf.Ours.Exists, cf.Theirs.Exists, cf.Diff3)
	}
}
