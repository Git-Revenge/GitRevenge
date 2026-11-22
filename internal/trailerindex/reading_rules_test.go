package trailerindex

import (
	"slices"
	"testing"
)

// A repository's own rules change how its trailers are read, and nothing else: one key read
// as another, what a key's values are, and where a key is offered.
func TestReadingFollowsTheRepositorysRules(t *testing.T) {
	t.Parallel()
	sam := "Sam Ortiz <sam@tidewater.example>"
	x, ids := indexOf(
		"Reviewed-by: "+sam+"\nFixes: #1\nTicket: TIDE-1\nChangelog: fixed",
		"Rviewed-by: "+sam+"\nFixes: #2\nTicket: TIDE-2\nChangelog: fixed",
		"Reviewed-by: "+sam+"\nFixes: #3\nTicket: TIDE-3\nChangelog: added",
		"Closes: #4\nNote: once only\nChangelog: added",
	)

	// With no rules, each is its own key.
	plain := x.Reading(Rules{})
	if _, found := plain.Key("rviewed-by"); !found {
		t.Fatal("with no rules, Rviewed-by should be a key of its own")
	}
	if k, _ := plain.Key("closes"); k.Commits != 1 {
		t.Fatalf("with no rules, Closes is on %d commits, want 1", k.Commits)
	}
	if k, _ := plain.Key("note"); k.Held != HeldRare {
		t.Fatalf("with no rules, a key on one commit is held as %q, want rare", k.Held)
	}

	r := x.Reading(Rules{
		Join:   map[string]string{"rviewed-by": "reviewed-by", "fixes": "closes"},
		Labels: map[string]string{"closes": "Closes"},
		Kinds:  map[string]Kind{"ticket": KindTicket},
		Show:   map[string]string{"note": ShowPanel, "changelog": ShowHeld},
	})

	// One key read as another: its commits and spellings are the other key's.
	reviewed, found := r.Key("Rviewed-by")
	if !found || reviewed.ID != "reviewed-by" || reviewed.Commits != 3 || reviewed.Label != "Reviewed-by" {
		t.Fatalf("Rviewed-by is read as %+v, %v, want Reviewed-by on 3 commits", reviewed, found)
	}
	var spelled []string
	for _, sp := range reviewed.Spellings {
		spelled = append(spelled, sp.Text)
	}
	if !slices.Equal(spelled, []string{"Reviewed-by", "Rviewed-by"}) {
		t.Fatalf("its spellings = %v, want both, the most written first", spelled)
	}
	for _, k := range r.Keys() {
		if k.ID == "rviewed-by" || k.ID == "fixes" {
			t.Fatalf("%s is still a key of its own", k.ID)
		}
	}
	if r.KeyID("RVIEWED-BY") != "reviewed-by" || r.KeyID("fixed") != "closes" || r.KeyID("Changelog") != "changelog" {
		t.Fatalf("key ids = %q, %q, %q", r.KeyID("RVIEWED-BY"), r.KeyID("fixed"), r.KeyID("Changelog"))
	}

	// Joined, the row reads as the rule names it, though the other spelling is used more.
	closes, _ := r.Key("fixes")
	if closes.ID != "closes" || closes.Commits != 4 || closes.Label != "Closes" || closes.Kind != KindTicket {
		t.Fatalf("Fixes joined to Closes = %+v, want Closes on 4 commits", closes)
	}
	// A pick, a filter and a row's chips all find it by either name.
	if got := r.Filter([]Pick{{Key: "Fixes", Value: "ticket:#4"}}); len(got) != 1 {
		t.Fatalf("filtering Fixes by a ticket written under Closes keeps %d commits, want 1", len(got))
	}
	if _, ok := got(r.Filter([]Pick{{Key: "closes", Value: "ticket:#1"}}), ids[0]); !ok {
		t.Fatal("filtering Closes by a ticket written under Fixes misses its commit")
	}
	if chips := r.Chips(ids[0], []string{"Closes"}); len(chips) != 1 || chips[0].KeyLabel != "Closes" || chips[0].Label != "#1" {
		t.Fatalf("the chips of a commit that wrote Fixes = %+v", chips)
	}

	// What its values are: said, not guessed.
	ticket, _ := r.Key("ticket")
	if ticket.Kind != KindTicket || ticket.Held != "" {
		t.Fatalf("Ticket = %+v, want tickets, offered", ticket)
	}
	for _, v := range r.Values("ticket") {
		if v.Kind != KindTicket {
			t.Fatalf("a value of Ticket is %q, want a ticket", v.Kind)
		}
	}
	if was, _ := plain.Key("ticket"); was.Kind == KindTicket {
		t.Fatal("with no rules, TIDE-1 is not known to be a ticket")
	}

	// Where it is offered: said, whatever the guess.
	if k, _ := r.Key("note"); k.Held != "" {
		t.Fatalf("a key the rules offer is held as %q", k.Held)
	}
	if k, _ := r.Key("changelog"); k.Held != HeldRule {
		t.Fatalf("a key the rules hold back is held as %q, want rule", k.Held)
	}
}

func got(kept map[string]struct{}, id string) (struct{}, bool) {
	v, ok := kept[id]
	return v, ok
}

// Rules that go round in a circle, or join a key to itself, still end somewhere.
func TestJoinsThatGoRoundEnd(t *testing.T) {
	t.Parallel()
	x, _ := indexOf("A-key: one", "B-key: two", "C-key: three")
	r := x.Reading(Rules{Join: map[string]string{"a-key": "b-key", "b-key": "a-key", "c-key": "c-key"}})
	if n := len(r.Keys()); n == 0 || n > 3 {
		t.Fatalf("%d keys", n)
	}
	if id := r.KeyID("c-key"); id != "c-key" {
		t.Fatalf("a key joined to itself is %q", id)
	}
	// A join to a join is followed to the end.
	chained := x.Reading(Rules{Join: map[string]string{"a-key": "b-key", "b-key": "c-key"}})
	if k, found := chained.Key("A-key"); !found || k.ID != "c-key" || k.Commits != 3 {
		t.Fatalf("a-key read through b-key = %+v, %v, want c-key on all three", k, found)
	}
}
