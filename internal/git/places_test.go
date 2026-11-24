package git

import (
	"reflect"
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

func TestIsMarker(t *testing.T) {
	for line, want := range map[string]bool{"<<<<<<<": true, "<<<<<<< HEAD": true, "<<<<<<<<": false, "<<<<<< HEAD": false, "<<<<<<<HEAD": false} {
		if got := isMarker(line, '<'); got != want {
			t.Errorf("isMarker(%q) = %v", line, got)
		}
	}
}
