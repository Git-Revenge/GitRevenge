package fetchlog

import (
	"testing"
	"time"
)

func TestNoteReadRenameForget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	common := t.TempDir()
	if got := Read(dir, common); len(got) != 0 {
		t.Fatalf("nothing written yet: %v", got)
	}
	first := time.Unix(1_800_000_000, 0)
	Note(dir, common, []string{"origin", "upstream"}, first)
	later := first.Add(time.Hour)
	Note(dir, common, []string{"origin"}, later)

	got := Read(dir, common)
	if !got["origin"].Equal(later) || !got["upstream"].Equal(first) {
		t.Fatalf("times = %v", got)
	}
	if other := Read(dir, t.TempDir()); len(other) != 0 {
		t.Fatalf("another repository sees %v", other)
	}

	Rename(dir, common, "upstream", "mirror")
	Forget(dir, common, "origin")
	got = Read(dir, common)
	if len(got) != 1 || !got["mirror"].Equal(first) {
		t.Fatalf("after rename and forget: %v", got)
	}
}

func TestNoFolderKeepsNothing(t *testing.T) {
	t.Parallel()
	Note("", "x", []string{"origin"}, time.Now())
	if got := Read("", "x"); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
