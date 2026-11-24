package semdiff

import (
	"context"
	"strings"
	"testing"
)

func TestAPlaceBothSidesChangedDifferentlyIsCombined(t *testing.T) {
	base := "func Forecast(from time.Time, n int) ([]Reading, error) {\n"
	ours := "func Forecast(ctx context.Context, from time.Time, n int) ([]Reading, error) {\n"
	theirs := "func Forecast(from time.Time, n int, step time.Duration) ([]Reading, error) {\n"
	s := MergePlace(context.Background(), "go", base, ours, theirs)
	if !s.OK {
		t.Fatalf("not combined: %s", s.Reason)
	}
	want := "func Forecast(ctx context.Context, from time.Time, n int, step time.Duration) ([]Reading, error) {\n"
	if s.Text != want {
		t.Fatalf("text %q, want %q", s.Text, want)
	}
	if !strings.Contains(s.Reason, "ours added `ctx context.Context,`") || !strings.Contains(s.Reason, "theirs added `, step time.Duration`") {
		t.Errorf("reason %q", s.Reason)
	}
	// The origins mark what each side brought.
	var got []string
	for _, o := range s.Origins {
		got = append(got, o.Side+":"+string([]rune(s.Text)[o.From:o.To]))
	}
	if strings.Join(got, "|") != "ours:ctx context.Context, |theirs:, step time.Duration" {
		t.Errorf("origins %q", got)
	}
	// Each side's own changes against the original, for the panes above the result.
	if o := stretches(ours, s.Ours); o != "ctx context.Context," {
		t.Errorf("ours changed %q", o)
	}
	if th := stretches(theirs, s.Theirs); th != ", step time.Duration" {
		t.Errorf("theirs changed %q", th)
	}
}

func stretches(text string, ss []Stretch) string {
	var out []string
	for _, s := range ss {
		out = append(out, string([]rune(text)[s.From:s.To]))
	}
	return strings.Join(out, "|")
}

func TestEachSideSaysWhatItChangedEvenWhenTheyClash(t *testing.T) {
	s := MergePlace(context.Background(), "go", "const maxHours = 48 // two days\n", "const maxHours = 72 // two days\n", "const maxHours = 96 // four days\n")
	if s.OK {
		t.Fatalf("combined %q", s.Text)
	}
	if o, th := stretches("const maxHours = 72 // two days\n", s.Ours), stretches("const maxHours = 96 // four days\n", s.Theirs); o != "72" || th != "96 // four days" {
		t.Errorf("ours %q, theirs %q", o, th)
	}
}

func TestAPlaceBothSidesChangedTheSameTokenIsNotCombined(t *testing.T) {
	s := MergePlace(context.Background(), "go", "const maxHours = 48\n", "const maxHours = 72\n", "const maxHours = 96\n")
	if s.OK {
		t.Fatalf("combined %q", s.Text)
	}
	if s.Reason != "both sides changed `48`" {
		t.Errorf("reason %q", s.Reason)
	}
}

func TestBothSidesMakingTheSameChangeIsOne(t *testing.T) {
	s := MergePlace(context.Background(), "go", "a := 1\nb := 2\n", "a := 10\nb := 2\n", "a := 10\nb := 20\n")
	if !s.OK || s.Text != "a := 10\nb := 20\n" {
		t.Fatalf("%+v", s)
	}
}

func TestAdditionsInTheSameSpotAreNotCombined(t *testing.T) {
	s := MergePlace(context.Background(), "go", "f(a)\n", "f(a, b)\n", "f(a, c)\n")
	if s.OK {
		t.Fatalf("combined %q", s.Text)
	}
}

func TestAStringIsOneToken(t *testing.T) {
	// Two edits inside one string literal are both changes to that token.
	s := MergePlace(context.Background(), "go", "x := \"one two\"\n", "x := \"ONE two\"\n", "x := \"one TWO\"\n")
	if s.OK {
		t.Fatalf("combined inside a string: %q", s.Text)
	}
}

func TestAFragmentThatDoesntParseStillMerges(t *testing.T) {
	// Half a function, as a conflicted place often is.
	s := MergePlace(context.Background(), "go", "\tif a {\n\t\tx()\n", "\tif a && b {\n\t\tx()\n", "\tif a {\n\t\ty()\n")
	if !s.OK || s.Text != "\tif a && b {\n\t\ty()\n" {
		t.Fatalf("%+v", s)
	}
}
