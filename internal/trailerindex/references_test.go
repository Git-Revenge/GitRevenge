package trailerindex

import (
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// A commit's references are the subject's, then the tickets its trailers name, in order and
// each once. These are what Shipped found with a list of keys of its own, and must still find.
func TestReferencesFindsWhatShippedAlwaysFound(t *testing.T) {
	t.Parallel()
	for msg, want := range map[string][]string{
		"Cache readings offline (TIDE-88)":                        {"TIDE-88"},
		"Paginate readings (#41)\n\nBody mentions OTHER-1.":       {"#41"},
		"Fix rounding\n\nDetails.\n\nRefs: TIDE-9, TIDE-10":       {"TIDE-9", "TIDE-10"},
		"TIDE-3 and TIDE-3 again\n\nCloses: TIDE-3\nSee: HARB-14": {"TIDE-3", "HARB-14"},
		"Plain subject\n\nSigned-off-by: Someone <s@example.com>": {},
		"lowercase-12 is not a reference":                         {},
		"Round tides\n\nCloses: #12\nFixes: #13, #14":             {"#12", "#13", "#14"},
		"Round tides (#12)\n\nCloses: #12":                        {"#12"},
	} {
		if got := References(msg, Rules{}, git.SubjectReferences); !slices.Equal(got, want) {
			t.Errorf("References(%q) = %v, want %v", msg, got, want)
		}
	}
}

// What the trailer reader reads as a ticket is one whichever key it's under, and what it
// doesn't read as one isn't, though it sits under a key that often holds tickets.
func TestReferencesReadsTrailersAsTheReadingDoes(t *testing.T) {
	t.Parallel()
	for msg, want := range map[string][]string{
		// Any key: the value is what names a ticket.
		"Subject\n\nBug: #12": {"#12"},
		"Subject\n\nBugLink: https://tracker.example/issues/88": {"https://tracker.example/issues/88"},
		"Subject\n\nIssue: VIZ-9250":                            {"VIZ-9250"},
		"Subject\n\nRefs: tide/api#12":                          {"tide/api#12"},
		// A commit that fixes another, whose quoted subject has a number in it, names no ticket.
		"Subject\n\nFixes: 00d60fd3b932 (\"KEYS: Provide keyctls [ver #2]\")": {},
		// Nor does a ticket among other words: the reading takes a value whole.
		"Subject\n\nCloses: #12 once the gauge is back": {},
		// Nor a person, a label, or something that only looks like a tracker's key.
		"Subject\n\nReviewed-by: Sam Ortiz <sam@tidewater.example>": {},
		"Subject\n\nChangelog: fixed":                               {},
		"Subject\n\nEncoding: UTF-8 text":                           {},
		"Subject\n\nSee: CVE-2021-4155":                             {},
		// A list with one thing in it that isn't a ticket isn't a list of tickets.
		"Subject\n\nCloses: #12, soon": {},
	} {
		if got := References(msg, Rules{}, git.SubjectReferences); !slices.Equal(got, want) {
			t.Errorf("References(%q) = %v, want %v", msg, got, want)
		}
	}
	// With nobody to read the subject, only the trailers speak.
	if got := References("Round tides (#12)\n\nCloses: #14", Rules{}, nil); !slices.Equal(got, []string{"#14"}) {
		t.Errorf("with no subject reader = %v", got)
	}
	if got := References("", Rules{}, git.SubjectReferences); got == nil || len(got) != 0 {
		t.Errorf("an empty message = %v, want an empty list", got)
	}
}

// A repository's rules for its trailer keys have their say here as they do in the panel.
func TestReferencesFollowTheRepositorysRules(t *testing.T) {
	t.Parallel()
	msg := "Subject\n\nTicket: tide 12, tide 14\nCloses: #3\nRefs: #4\nRviewed: #5"
	refs := func(rules Rules) string {
		return strings.Join(References(msg, rules, git.SubjectReferences), " ")
	}
	if got := refs(Rules{}); got != "#3 #4 #5" {
		t.Fatalf("with no rules = %q", got)
	}
	// A key the repository says holds tickets names one for each value, as written.
	if got := refs(Rules{Kinds: map[string]Kind{"ticket": KindTicket}}); got != "tide 12 tide 14 #3 #4 #5" {
		t.Fatalf("with Ticket said to be tickets = %q", got)
	}
	// A key it holds back names none, and neither does one it says holds something else.
	if got := refs(Rules{Show: map[string]string{"refs": ShowHeld}}); got != "#3 #5" {
		t.Fatalf("with Refs held back = %q", got)
	}
	if got := refs(Rules{Kinds: map[string]Kind{"closes": KindLabel}}); got != "#4 #5" {
		t.Fatalf("with Closes said to be labels = %q", got)
	}
	// A key read as another follows the other's rule.
	if got := refs(Rules{Join: map[string]string{"rviewed": "refs"}, Show: map[string]string{"refs": ShowHeld}}); got != "#3" {
		t.Fatalf("with Rviewed read as Refs, which is held back = %q", got)
	}
	// Smart links read a value as tickets before anything else does.
	linked := Rules{Tickets: func(value string) []Ticket {
		if strings.HasPrefix(value, "tide ") {
			var out []Ticket
			for part := range strings.SplitSeq(value, ",") {
				out = append(out, Ticket{Text: strings.TrimSpace(part), Href: "https://tracker.example/" + strings.TrimSpace(part)})
			}
			return out
		}
		return nil
	}}
	if got := refs(linked); got != "tide 12 tide 14 #3 #4 #5" {
		t.Fatalf("with a smart link for the tracker = %q", got)
	}
}

// A tracker's key by itself is a ticket in the reading too, so the panel and Shipped agree.
func TestATrackersKeyIsATicket(t *testing.T) {
	t.Parallel()
	x, _ := indexOf("Issue: VIZ-9250", "Issue: VIZ-4525", "Issue: VIZ-7610, VIZ-7611", "Encoding: UTF-8 text")
	r := x.Reading(Rules{})
	k, found := r.Key("issue")
	if !found || k.Kind != KindTicket || k.Values != 4 {
		t.Fatalf("Issue = %+v, %v, want four tickets", k, found)
	}
	if k, _ = r.Key("encoding"); k.Kind == KindTicket {
		t.Fatalf("Encoding = %+v: text with a hyphen and a number in it isn't a ticket", k)
	}
}
