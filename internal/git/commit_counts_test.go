package git

import (
	"context"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestChangedFilesCountsLines(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.WriteFile("kept.txt", "one\ntwo\nthree\n")
	fx.WriteFile("moved.txt", "a\nb\nc\nd\ne\nf\ng\nh\n")
	fx.WriteFile("gone.txt", "bye\nbye\n")
	fx.Commit("start")
	fx.WriteFile("kept.txt", "one\nTWO\nthree\nfour\n")
	fx.WriteFile("new.txt", "hello\n")
	fx.WriteFile("data.bin", "\x00\x01\x02\x00")
	fx.RemoveFile("gone.txt")
	fx.Git("mv", "moved.txt", "renamed.txt")
	id := fx.Commit("change")

	repo := OpenRepo(newTestRunner(t), fx.Dir)
	files, err := repo.ChangedFiles(context.Background(), CommitDetail{ID: id, Parents: []string{id + "~1"}})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]FileChange{}
	for _, f := range files {
		got[f.Path] = f
	}
	check := func(path string, added, deleted int, binary bool) {
		t.Helper()
		f, ok := got[path]
		if !ok {
			t.Fatalf("%s isn't in %+v", path, files)
		}
		if f.Added != added || f.Deleted != deleted || f.Binary != binary {
			t.Errorf("%s: got +%d -%d binary=%v", path, f.Added, f.Deleted, f.Binary)
		}
	}
	check("kept.txt", 2, 1, false)
	check("new.txt", 1, 0, false)
	check("gone.txt", 0, 2, false)
	check("data.bin", 0, 0, true)
	check("renamed.txt", 0, 0, false)
	if got["renamed.txt"].OldPath != "moved.txt" {
		t.Errorf("rename lost its old path: %+v", got["renamed.txt"])
	}
}
