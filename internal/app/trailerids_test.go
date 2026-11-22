package app

import (
	"errors"
	"maps"
	"slices"
	"testing"
)

// A reading asks git about an abbreviated commit id once: one that named a commit is kept,
// and one that named none is asked again only after the index has read new commits (#897).
func TestKeptReadingAsksAboutACommitIDOnce(t *testing.T) {
	t.Parallel()
	var asked [][]string
	answers := map[string]string{"abc123": "abc123" + "0000000000000000000000000000000000", "def456": "def456" + "1111111111111111111111111111111111"}
	ask := func(ids []string) (map[string]string, error) {
		asked = append(asked, slices.Clone(ids))
		out := map[string]string{}
		for _, id := range ids {
			if full, ok := answers[id]; ok {
				out[id] = full
			}
		}
		return out, nil
	}
	var k keptReading
	want := map[string]string{"abc123": answers["abc123"], "def456": answers["def456"]}

	got := k.commitIDs(1, []string{"abc123", "def456", "fade00"}, ask)
	if !maps.Equal(got, want) || len(asked) != 1 || len(asked[0]) != 3 {
		t.Fatalf("first reading: %v, asked %v", got, asked)
	}
	// Another reading of the same index, after a rule changed: nothing is asked.
	got = k.commitIDs(1, []string{"abc123", "def456", "fade00"}, ask)
	if !maps.Equal(got, want) || len(asked) != 1 {
		t.Fatalf("second reading: %v, asked %v", got, asked)
	}
	// New commits were read: only the id that named nothing is asked again, and it has come.
	answers["fade00"] = "fade00" + "2222222222222222222222222222222222"
	got = k.commitIDs(2, []string{"abc123", "def456", "fade00", "beef99"}, ask)
	want["fade00"] = answers["fade00"]
	if !maps.Equal(got, want) || len(asked) != 2 || !slices.Equal(asked[1], []string{"fade00", "beef99"}) {
		t.Fatalf("after new commits: %v, asked %v", got, asked)
	}
	// Only what was asked about is answered.
	if got := k.commitIDs(2, []string{"def456"}, ask); len(got) != 1 || got["def456"] == "" || len(asked) != 2 {
		t.Fatalf("one id: %v, asked %v", got, asked)
	}
	// A question git couldn't answer is asked again the next time.
	var failing keptReading
	fail := func(ids []string) (map[string]string, error) {
		asked = append(asked, ids)
		return nil, errors.New("git went away")
	}
	if got := failing.commitIDs(1, []string{"abc123"}, fail); len(got) != 0 {
		t.Fatalf("with git failing: %v", got)
	}
	if got := failing.commitIDs(1, []string{"abc123"}, ask); got["abc123"] == "" {
		t.Fatalf("asked again once git answers: %v", got)
	}
}
