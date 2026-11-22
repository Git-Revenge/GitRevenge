package trailerindex

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestOneOff(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"reviewed-by", "rviewed-by", true},   // a letter dropped
		{"reviewed-by", "reviewed-byy", true}, // a letter added
		{"reviewed-by", "reviewed-bz", true},  // a letter wrong
		{"reviewed-by", "reviewed-yb", true},  // two neighbours swapped
		{"reviewed-by", "reviewed-by", false}, // the same
		{"reviewed-by", "review-by", false},   // two letters dropped
		{"reviewed-by", "reported-by", false},
		{"cc", "c", true},
		{"tested-by", "testde-by", true},
		{"tested-by", "tetsed-yb", false}, // two swaps
		{"", "a", true},
		{"", "", false},
	} {
		if got := oneOff(c.a, c.b); got != c.want {
			t.Errorf("oneOff(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		if got := oneOff(c.b, c.a); got != c.want {
			t.Errorf("oneOff(%q, %q) = %v, want %v", c.b, c.a, got, c.want)
		}
	}
}

// many is n commit messages, each made by line from its number.
func many(n int, line func(i int) string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = line(i)
	}
	return out
}

func asked(qs []Question, kind string) []string {
	var out []string
	for _, q := range qs {
		if q.Kind != kind {
			continue
		}
		if kind == CarePerson {
			out = append(out, strings.Join(q.Examples, " | "))
		} else {
			out = append(out, q.OtherLabel+" as "+q.KeyLabel)
		}
	}
	return out
}

// A key one letter off a much more used one is asked about; one that's merely alike, or
// used about as much, isn't.
func TestCareAsksAboutASlipOfSpelling(t *testing.T) {
	t.Parallel()
	sam := "Sam Ortiz <sam@tidewater.example>"
	messages := many(30, func(int) string { return "Reviewed-by: " + sam + "\nTested-by: " + sam })
	messages = append(messages,
		"Rviewed-by: "+sam, "Rviewed-by: "+sam, // a slip, twice
		"Reported-by: "+sam, // another key altogether
	)
	// Two keys a letter apart and used as much as each other: neither is a slip.
	messages = append(messages, many(8, func(int) string { return "Area: tides\nAreas: tides" })...)
	x, _ := indexOf(messages...)
	r := x.Reading(Rules{})

	qs, total := r.Care(nil, 0)
	if got := asked(qs, CareSpelling); !slices.Equal(got, []string{"Rviewed-by as Reviewed-by"}) {
		t.Fatalf("spelling questions = %v, want the one slip", got)
	}
	q := qs[0]
	if q.Kind != CareSpelling || q.Key != "reviewed-by" || q.Commits != 30 || q.Other != "rviewed-by" || q.OtherCommits != 2 {
		t.Fatalf("the question = %+v, and it should come first", q)
	}
	if total != len(qs) {
		t.Fatalf("total = %d of %d", total, len(qs))
	}

	// Answered no: it isn't asked again.
	apart := func(a, b string) bool {
		return a == "reviewed-by" && b == "rviewed-by" || a == "rviewed-by" && b == "reviewed-by"
	}
	if qs, _ = r.Care(apart, 0); len(asked(qs, CareSpelling)) != 0 {
		t.Fatalf("kept apart, and asked again: %v", asked(qs, CareSpelling))
	}
	// Answered yes: there's one key, and nothing to ask.
	joined := x.Reading(Rules{Join: map[string]string{"rviewed-by": "reviewed-by"}})
	if qs, _ = joined.Care(nil, 0); len(asked(qs, CareSpelling)) != 0 {
		t.Fatalf("joined, and asked again: %v", asked(qs, CareSpelling))
	}
}

// Two keys full of the same things are asked about, and the ticket verbs are offered to
// each other, to read as Closes.
func TestCareAsksAboutKeysHoldingTheSameThings(t *testing.T) {
	t.Parallel()
	sam := "Sam Ortiz <sam@tidewater.example>"
	var messages []string
	// Ticket and Refs name the same tickets; Closes and Fixes each settle their own.
	for i := range 12 {
		messages = append(messages,
			fmt.Sprintf("Ticket: #%d\nSigned-off-by: %s", i%6, sam),
			fmt.Sprintf("Refs: #%d\nReviewed-by: %s", i%6, sam),
			fmt.Sprintf("Closes: #%d", 100+i),
			fmt.Sprintf("Fixes: #%d", 200+i),
		)
	}
	// A key of labels that shares nothing with anything.
	messages = append(messages, many(10, func(int) string { return "Changelog: fixed" })...)
	x, _ := indexOf(messages...)
	r := x.Reading(Rules{})

	qs, _ := r.Care(nil, 0)
	got := asked(qs, CareSame)
	slices.Sort(got)
	// Sam signs off and reviews: people under two keys aren't the same thing.
	if !slices.Equal(got, []string{"Fixes as Closes", "Refs as Ticket"}) && !slices.Equal(got, []string{"Fixes as Closes", "Ticket as Refs"}) {
		t.Fatalf("same-things questions = %v", got)
	}
	for _, q := range qs {
		if q.Kind != CareSame {
			continue
		}
		if q.Other == "fixes" && (q.Key != "closes" || q.Shared != 0) {
			t.Fatalf("the ticket verbs = %+v, want Fixes read as Closes though they share no ticket", q)
		}
		if q.Other != "fixes" && (q.Shared != 6 || len(q.Examples) != 3 || !strings.HasPrefix(q.Examples[0], "#")) {
			t.Fatalf("Ticket and Refs = %+v, want six tickets shared and three shown", q)
		}
	}

	// Joined by a rule, the question goes, and the row reads Closes.
	joined := x.Reading(Rules{Join: map[string]string{"fixes": "closes"}, Labels: map[string]string{"closes": "Closes"}})
	if qs, _ = joined.Care(nil, 0); slices.Contains(asked(qs, CareSame), "Fixes as Closes") {
		t.Fatal("joined, and asked again")
	}
	if k, _ := joined.Key("fixes"); k.Label != "Closes" || k.Commits != 24 {
		t.Fatalf("joined = %+v", k)
	}
}

// One person written two ways is shown, by address and by name; a bot isn't.
func TestCareShowsOnePersonTwoWays(t *testing.T) {
	t.Parallel()
	messages := []string{
		"Reviewed-by: Sarah Lind <sarah@tidewater.example>",
		"Reviewed-by: Sarah Lind <sarah@tidewater.example>",
		"Reviewed-by: sarah lind <slind@users.noreply.example>", // the same name, another address
		"Signed-off-by: S. Lind <sarah@tidewater.example>",      // the same address, another name
		"Reviewed-by: Sam Ortiz <sam@tidewater.example>",        // someone else, written one way
	}
	x, _ := indexOf(messages...)
	r := x.Reading(Rules{})
	if qs, _ := r.Care(nil, 0); len(qs) != 0 {
		t.Fatalf("people are asked about among the keys: %+v", qs)
	}
	qs, total := r.People(0)
	got := asked(qs, CarePerson)
	if len(got) != 2 || total != 2 {
		t.Fatalf("person questions = %v (%d in all), want one by address and one by name", got, total)
	}
	for _, g := range got {
		if !strings.Contains(g, "Sarah Lind <sarah@tidewater.example>") || strings.Contains(g, "Sam Ortiz") {
			t.Fatalf("a person question shows %q", g)
		}
	}
	// A question about a person names no keys: the mailmap answers it, not a rule.
	for _, q := range qs {
		if q.Key != "" || q.Other != "" || len(q.Examples) < 2 {
			t.Fatalf("a person question = %+v", q)
		}
	}

	// Once the mailmap joins them there's one person, and nothing to show.
	one := x.Reading(Rules{Person: func(name, email string) (string, string, bool) {
		if strings.Contains(strings.ToLower(name), "lind") {
			return "Sarah Lind", "sarah@tidewater.example", false
		}
		return name, email, false
	}})
	if qs, _ = one.People(0); len(qs) != 0 {
		t.Fatalf("joined by the mailmap, and still shown: %v", asked(qs, CarePerson))
	}
	// Only so many are handed over, with how many there are.
	if few, n := r.People(1); len(few) != 1 || n != 2 {
		t.Fatalf("limited to one: %d of %d", len(few), n)
	}
}

// A long history can raise hundreds of questions: only so many are handed over, the most
// telling first, with how many there are.
func TestCareIsRankedAndLimited(t *testing.T) {
	t.Parallel()
	var messages []string
	for k := range 12 {
		key := fmt.Sprintf("Team%c-key", 'a'+k)
		messages = append(messages, many(10+k, func(int) string { return key + ": x" })...)
		// One slip for each, a letter dropped from the end.
		messages = append(messages, key[:len(key)-1]+": x")
	}
	x, _ := indexOf(messages...)
	r := x.Reading(Rules{})
	all, total := r.Care(nil, 0)
	if total != 12 || len(all) != 12 {
		t.Fatalf("%d questions of %d, want 12", len(all), total)
	}
	few, total := r.Care(nil, 5)
	if total != 12 || len(few) != 5 {
		t.Fatalf("limited to 5: %d of %d", len(few), total)
	}
	// The slip on the most commits comes first, then the one whose key is used most, and
	// it's the same every time.
	for i := 1; i < len(all); i++ {
		a, b := all[i-1], all[i]
		if b.OtherCommits > a.OtherCommits || (b.OtherCommits == a.OtherCommits && b.Commits > a.Commits) {
			t.Fatalf("question %d comes after one that changes fewer commits", i)
		}
	}
	again, _ := r.Care(nil, 5)
	for i := range few {
		if few[i].Key != again[i].Key || few[i].Other != again[i].Other {
			t.Fatalf("the order changed between two askings: %+v then %+v", few[i], again[i])
		}
	}
	// With nothing to ask there's an empty list, not nothing.
	empty, _ := indexOf("Changelog: fixed")
	if qs, n := empty.Reading(Rules{}).Care(nil, 0); qs == nil || len(qs) != 0 || n != 0 {
		t.Fatalf("nothing to ask = %v, %d", qs, n)
	}
}

// What was joined without asking is there to be shown.
func TestJoinedWithoutAsking(t *testing.T) {
	t.Parallel()
	x, _ := indexOf(
		"Closes: #1, #2\nReviewed-by: Sam Ortiz <sam@tidewater.example>",
		"closed: #3\nreviewed-by: Sam Ortiz <sam@tidewater.example>",
		"Co-authored-by: dependabot[bot] <bot@users.noreply.example>",
		"Changelog: fixed",
	)
	r := x.Reading(Rules{Person: func(name, email string) (string, string, bool) {
		return name, email, strings.HasSuffix(name, "[bot]")
	}})
	j := r.Joined()
	var keys []string
	for _, k := range j.Spellings {
		keys = append(keys, k.ID)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"closes", "reviewed-by"}) {
		t.Fatalf("keys written more than one way = %v", keys)
	}
	if j.Several != 1 || j.Bots != 1 {
		t.Fatalf("joined = %+v, want one line naming several and one bot", j)
	}
}
