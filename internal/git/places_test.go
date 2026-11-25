package git

import (
	"reflect"
	"strings"
	"testing"
)

func TestConflictTextsSplitAFileAtItsMarkers(t *testing.T) {
	text := "a\n<<<<<<< ours\nb := 3\n||||||| 735c732\nb := 1\n=======\nb := 2\n>>>>>>> x\nc\n" +
		"<<<<<<< HEAD\r\nd\r\n=======\r\ne\r\n>>>>>>> topic\r\n" +
		"=======\nnot a marker\n"
	got := ConflictTexts(text)
	want := []ConflictText{
		{Ours: "b := 3\n", Base: "b := 1\n", Theirs: "b := 2\n", HasBase: true},
		{Ours: "d\r\n", Theirs: "e\r\n"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestConflictTextsKeepEmptySides(t *testing.T) {
	got := ConflictTexts("<<<<<<< ours\n||||||| base\nold\n=======\n>>>>>>> theirs\n")
	want := []ConflictText{{Base: "old\n", HasBase: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

// git's default style joins places a line apart; diff3's keeps them apart. They're still the
// same merge, until something is resolved or edited (#974).
func TestSameMergeComparesWhatEachSideReads(t *testing.T) {
	joined := "feed:\n<<<<<<< HEAD\n  timeout_s: 10\nalerts:\n  wind_knots: 32\n=======\n  max_age_min: 30\n  timeout_s: 5\nalerts:\n  wind_kmh: 55\n>>>>>>> topic\n"
	apart := "feed:\n<<<<<<< ours\n  timeout_s: 10\n||||||| base\n  timeout_s: 5\n=======\n  max_age_min: 30\n  timeout_s: 5\n>>>>>>> theirs\nalerts:\n" +
		"<<<<<<< ours\n  wind_knots: 32\n||||||| base\n  wind_kmh: 60\n=======\n  wind_kmh: 55\n>>>>>>> theirs\n"
	for _, c := range []struct {
		name, disk string
		same       bool
	}{
		{"joined", joined, true},
		{"as written", apart, true},
		{"resolved as ours", "feed:\n  timeout_s: 10\nalerts:\n  wind_knots: 32\n", false},
		{"edited outside a place", strings.Replace(joined, "feed:", "feed: # nightly", 1), false},
		{"a place with no end", strings.TrimSuffix(joined, ">>>>>>> topic\n"), false},
	} {
		if got := SameMerge(c.disk, apart); got != c.same {
			t.Errorf("%s: SameMerge = %v", c.name, got)
		}
	}
}

func TestIsMarker(t *testing.T) {
	for line, want := range map[string]bool{"<<<<<<<": true, "<<<<<<< HEAD": true, "<<<<<<<<": false, "<<<<<< HEAD": false, "<<<<<<<HEAD": false} {
		if got := isMarker(line, '<'); got != want {
			t.Errorf("isMarker(%q) = %v", line, got)
		}
	}
}
