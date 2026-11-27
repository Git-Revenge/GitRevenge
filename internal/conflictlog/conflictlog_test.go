package conflictlog

import (
	"slices"
	"testing"
	"time"
)

func TestResolvedThenSeen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo, other := "/work/repo/.git", "/work/other/.git"
	if got := Seen(dir, repo, []string{"a"}); !slices.Equal(got, []bool{false}) {
		t.Fatalf("nothing kept: %v", got)
	}
	Resolved(dir, repo, "a", time.Unix(100, 0))
	if got := Seen(dir, repo, []string{"a", "b"}); !slices.Equal(got, []bool{true, false}) {
		t.Errorf("seen = %v", got)
	}
	// Each repository keeps its own.
	if got := Seen(dir, other, []string{"a"}); !slices.Equal(got, []bool{false}) {
		t.Errorf("another repository: %v", got)
	}
	if Offered(dir, repo) {
		t.Error("offered before it was")
	}
	SetOffered(dir, repo)
	if !Offered(dir, repo) || Offered(dir, other) {
		t.Error("offered once, for that repository")
	}
	// Offering keeps what was resolved.
	if got := Seen(dir, repo, []string{"a"}); !slices.Equal(got, []bool{true}) {
		t.Errorf("after offering: %v", got)
	}
}

func TestResolvedKeepsTheNewest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := range keep + 3 {
		Resolved(dir, "/r/.git", string(rune('A'+i%26))+string(rune(i)), time.Unix(int64(i), 0))
	}
	rec := read(dir, "/r/.git")
	if len(rec.Resolved) != keep {
		t.Fatalf("kept %d", len(rec.Resolved))
	}
	if _, ok := rec.Resolved[string(rune('A'))+string(rune(0))]; ok {
		t.Error("the oldest wasn't dropped")
	}
}

func TestNoFolderKeepsNothing(t *testing.T) {
	t.Parallel()
	Resolved("", "/r/.git", "a", time.Now())
	SetOffered("", "/r/.git")
	if Offered("", "/r/.git") || Seen("", "/r/.git", []string{"a"})[0] {
		t.Error("kept something without a folder")
	}
}
