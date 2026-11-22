package trailerindex

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// indexOf builds an index from commits written as their trailers, "Key: value" a line. The
// ids are made up: c0, c1 and so on, padded.
func indexOf(commits ...string) (*Index, []string) {
	x := New()
	x.idSize = 20
	var b passBatch
	local := map[string]uint32{}
	number := func(s string) uint32 {
		n, ok := local[s]
		if !ok {
			n = uint32(len(b.strs))
			b.strs = append(b.strs, s)
			local[s] = n
		}
		return n
	}
	var ids []string
	for i, text := range commits {
		id := make([]byte, 20)
		copy(id, fmt.Sprintf("c%d", i))
		ids = append(ids, fmt.Sprintf("%x", id))
		c := batchCommit{id: id}
		for line := range strings.SplitSeq(text, "\n") {
			key, value, ok := strings.Cut(line, ": ")
			if !ok {
				continue
			}
			c.pairs = append(c.pairs, number(key), number(value))
		}
		b.commits = append(b.commits, c)
	}
	x.addLocked(b, nil)
	return x, ids
}

func keyNamed(t *testing.T, r *Reading, id string) Key {
	t.Helper()
	k, ok := r.Key(id)
	if !ok {
		t.Fatalf("no key %q among %+v", id, r.Keys())
	}
	return k
}

func valueLabels(vs []Value) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = fmt.Sprintf("%s×%d", v.Label, v.Commits)
	}
	return out
}

// Letter case and word endings don't make a different key, and the key is shown the way
// it's written most.
func TestReadingJoinsSpellingsOfAKey(t *testing.T) {
	t.Parallel()
	x, ids := indexOf(
		"Co-authored-by: Sam Ortiz <sam@tidewater.example>",
		"Co-authored-by: Sam Ortiz <sam@tidewater.example>",
		"Co-Authored-By: Sarah Lind <sarah@tidewater.example>",
		"Closes: #12",
		"Close: #13",
		"closed: #14",
		"Fixes: #15",
		"Resolves: #16",
		"Refs: #17\nReferences: #18\nref: #19",
	)
	r := x.Reading(Rules{})

	co := keyNamed(t, r, "co-authored-by")
	if co.Label != "Co-authored-by" || co.Commits != 3 || len(co.Spellings) != 2 || co.Spellings[1] != (Spelling{Text: "Co-Authored-By", Commits: 1}) {
		t.Fatalf("co-authored-by = %+v", co)
	}
	closes := keyNamed(t, r, "closes")
	if closes.Commits != 3 || closes.Values != 3 || len(closes.Spellings) != 3 {
		t.Fatalf("closes = %+v, want Close, Closed and Closes as one", closes)
	}
	// Any spelling finds the key.
	if byEnding, _ := r.Key("Closed"); byEnding.ID != "closes" {
		t.Fatalf("Closed is %+v", byEnding)
	}
	if refs := keyNamed(t, r, "refs"); refs.Commits != 1 || refs.Lines != 3 || refs.Values != 3 {
		t.Fatalf("refs = %+v, want one commit with three values", refs)
	}
	// Fixes, Closes and Resolves mean the same to some projects and not to others: three keys.
	for _, id := range []string{"fixes", "resolves"} {
		if k := keyNamed(t, r, id); k.Commits != 1 {
			t.Fatalf("%s = %+v, want it on its own", id, k)
		}
	}
	if got := r.Commits("Close", ""); !slices.Equal(got, ids[3:6]) {
		t.Fatalf("commits with Closes under any spelling = %v, want %v", got, ids[3:6])
	}
}

func TestReadingKinds(t *testing.T) {
	t.Parallel()
	var commits []string
	for i := range 12 {
		commits = append(commits, fmt.Sprintf("Reviewed-by: Sam Ortiz <sam@tidewater.example>\nChange-Id: I%040d\nChangelog: %s\nFixes: %s",
			i, []string{"fixed", "added", "changed"}[i%3], []string{"#12", "656c296 (\"Round tide heights\")", "656c296c"}[i%3]))
	}
	commits = append(commits, "Note: revert if the feed changes", "Note: see the wiki", "Note: only on Windows", "Cc: stable@tidewater.example")
	x, _ := indexOf(commits...)
	r := x.Reading(Rules{
		Commits: func(ids []string) map[string]string {
			out := map[string]string{}
			for _, id := range ids {
				if strings.HasPrefix(id, "656c296") {
					out[id] = "656c296c" + strings.Repeat("0", 32)
				}
			}
			return out
		},
	})

	for id, want := range map[string]Kind{"reviewed-by": KindPerson, "change-id": KindIdentifier, "changelog": KindLabel, "note": KindText, "cc": KindPerson} {
		if k := keyNamed(t, r, id); k.Kind != want {
			t.Errorf("%s is %s, want %s (%+v)", id, k.Kind, want, k)
		}
	}
	// One key, values of two kinds: the kind is the value's.
	fixes := r.Values("Fixes")
	if len(fixes) != 2 {
		t.Fatalf("Fixes has %v, want a ticket and a commit", valueLabels(fixes))
	}
	kinds := map[Kind]Value{}
	for _, v := range fixes {
		kinds[v.Kind] = v
	}
	// Two spellings of one commit, by abbreviations of different lengths, are one value.
	if c := kinds[KindCommit]; c.Commits != 8 || !strings.HasPrefix(c.ID, "commit:656c296c") {
		t.Fatalf("the commit = %+v, want both abbreviations as one, on 8 commits", c)
	}
	if tk := kinds[KindTicket]; tk.Commits != 4 || tk.Label != "#12" {
		t.Fatalf("the ticket = %+v", tk)
	}
	if k := keyNamed(t, r, "fixes"); k.Kind != KindCommit {
		t.Fatalf("Fixes is %s, want what most of its values are", k.Kind)
	}
	if labels := valueLabels(r.Values("changelog")); !slices.Equal(labels, []string{"added×4", "changed×4", "fixed×4"}) {
		t.Fatalf("changelog = %v", labels)
	}
	// Without being told which ids are commits, none is.
	if plain := x.Reading(Rules{}); keyNamed(t, plain, "fixes").Kind == KindCommit {
		t.Fatal("an id nobody vouched for was read as a commit")
	}
}

// A commit says a thing once, however many times it writes it, and "#12, #14" is two.
func TestReadingCountsACommitOnceAndSplitsLists(t *testing.T) {
	t.Parallel()
	x, ids := indexOf(
		"Closes: #12, #14",
		"Closes: #14\nCloses: #14",
		"Closes: #12 and the rest",
	)
	r := x.Reading(Rules{})
	closes := keyNamed(t, r, "closes")
	if closes.Commits != 3 || closes.Lines != 4 {
		t.Fatalf("closes = %+v", closes)
	}
	if labels := valueLabels(r.Values("closes")); !slices.Equal(labels, []string{"#14×2", "#12×1", "#12 and the rest×1"}) {
		t.Fatalf("values = %v", labels)
	}
	if got := r.Commits("closes", "ticket:#14"); !slices.Equal(got, ids[:2]) {
		t.Fatalf("commits closing #14 = %v, want the list and the repeat", got)
	}
	if got := r.Commits("closes", "ticket:#12"); !slices.Equal(got, ids[:1]) {
		t.Fatalf("commits closing #12 = %v", got)
	}
	if got := r.Commits("closes", "ticket:#99"); len(got) != 0 {
		t.Fatalf("a ticket nobody closes: %v", got)
	}
}

// A ticket's number and its address are one value when a smart link sends the number
// there, and it's shown by the shorter of the two.
func TestReadingJoinsANumberAndItsAddress(t *testing.T) {
	t.Parallel()
	address := "https://tracker.tidewater.example/issues/14"
	x, ids := indexOf("Closes: #14", "Closes: "+address, "Closes: https://tracker.tidewater.example/issues/15/")
	link := func(value string) []Ticket {
		if n, ok := strings.CutPrefix(value, "#"); ok {
			return []Ticket{{Text: value, Href: "https://tracker.tidewater.example/issues/" + n}}
		}
		return nil
	}
	r := x.Reading(Rules{Tickets: link})
	values := r.Values("closes")
	if len(values) != 2 || values[0].Commits != 2 || values[0].Label != "#14" || values[0].Href != address || values[0].Kind != KindTicket {
		t.Fatalf("values = %+v, want #14 and its address as one", values)
	}
	if got := r.Commits("closes", values[0].ID); !slices.Equal(got, ids[:2]) {
		t.Fatalf("commits = %v", got)
	}
	// With no smart link, nothing says they're the same.
	if apart := x.Reading(Rules{}).Values("closes"); len(apart) != 3 {
		t.Fatalf("without a link: %v", valueLabels(apart))
	}
}

// People go through whatever says who they are, and bots are marked.
func TestReadingPeople(t *testing.T) {
	t.Parallel()
	x, _ := indexOf(
		"Reviewed-by: Sarah Lind <sarah@tidewater.example>",
		"Reviewed-by: sarah lind <slind@users.noreply.example>",
		"Reviewed-by: Sarah Lind <sarah@tidewater.example> # with the gauge fix",
		"Signed-off-by: dependabot[bot] <bot@users.noreply.example>",
		"Cc: <stable@tidewater.example> # 2.x",
		"Cc: stable@tidewater.example",
	)
	r := x.Reading(Rules{Person: func(name, email string) (string, string, bool) {
		if email == "slind@users.noreply.example" {
			return "Sarah Lind", "sarah@tidewater.example", false
		}
		return name, email, strings.HasSuffix(name, "[bot]")
	}})
	reviewed := r.Values("reviewed-by")
	if len(reviewed) != 1 || reviewed[0].Commits != 3 || reviewed[0].Label != "Sarah Lind <sarah@tidewater.example>" || reviewed[0].Kind != KindPerson {
		t.Fatalf("reviewed-by = %+v, want her three spellings as one person", reviewed)
	}
	if bot := r.Values("signed-off-by"); len(bot) != 1 || !bot[0].Bot {
		t.Fatalf("signed-off-by = %+v, want a bot", bot)
	}
	// An address alone is a person too, with or without brackets and a note.
	if cc := r.Values("cc"); len(cc) != 1 || cc[0].Commits != 2 || cc[0].Label != "stable@tidewater.example" {
		t.Fatalf("cc = %+v", cc)
	}
}

// A Message-Id is written like an address with no name, and is nobody: the kernel has
// nine thousand of them, each on one commit. An address many commits copy in is somebody.
func TestReadingTellsAMessageIDFromAnAddress(t *testing.T) {
	t.Parallel()
	var commits []string
	for i := range 12 {
		commits = append(commits, fmt.Sprintf("Message-Id: <2026112%d.9102-%d-sam@tidewater.example>\nCc: <stable@tidewater.example>", i%10, i))
	}
	x, _ := indexOf(commits...)
	r := x.Reading(Rules{})
	if id := keyNamed(t, r, "message-id"); id.Kind != KindIdentifier || id.Held != HeldIdentifier {
		t.Fatalf("message-id = %+v, want an identifier, held back", id)
	}
	if v := r.Values("message-id"); len(v) != 12 || v[0].Kind != KindIdentifier {
		t.Fatalf("its values = %+v", v[:1])
	}
	cc := keyNamed(t, r, "cc")
	if cc.Kind != KindPerson || cc.Held != "" || cc.Values != 1 {
		t.Fatalf("cc = %+v, want one person, offered", cc)
	}
}

func TestReadingHoldsBackWhatIsNotWorthFiltering(t *testing.T) {
	t.Parallel()
	var commits []string
	for i := range 60 {
		c := fmt.Sprintf("Signed-off-by: Sam Ortiz <sam@tidewater.example>\nChange-Id: I%040d", i)
		if i < 40 {
			c += "\nCo-authored-by: Sarah Lind <sarah@tidewater.example>"
		}
		commits = append(commits, c)
	}
	commits[0] += "\ndocs: add the plan and a README"
	commits[1] += "\nReviewed-by: Sarah Lind <sarah@tidewater.example>"
	commits[2] += "\nBench: 100k commits searched in 6.4ms\nSee-also: the wiki"
	x, _ := indexOf(commits...)
	r := x.Reading(Rules{Total: 60})

	want := map[string]string{
		"signed-off-by":  HeldEverywhere, // on every commit: it filters nothing
		"change-id":      HeldIdentifier, // a different value on each: it groups nothing
		"co-authored-by": "",             // on two commits in three, and worth filtering by
		"docs":           HeldRare,       // a sentence shaped like a trailer
		"bench":          HeldRare,
		"reviewed-by":    "", // on one commit, and plainly a trailer: its value is a person
		"see-also":       "", // on one commit, and a key in common use
	}
	for id, held := range want {
		if k := keyNamed(t, r, id); k.Held != held {
			t.Errorf("%s is held as %q, want %q (%+v)", id, k.Held, held, k)
		}
	}
	// A held key still answers: search finds it.
	if got := r.Commits("docs", ""); len(got) != 1 {
		t.Fatalf("commits with the held key = %v", got)
	}
	// In a history's first weeks every key is on most commits, and that isn't judged.
	if young := x.Reading(Rules{Total: 0}); keyNamed(t, young, "signed-off-by").Held != "" {
		t.Fatal("a key was held for being everywhere with no total to judge by")
	}
	// Most commits first.
	keys := r.Keys()
	if keys[0].ID != "change-id" && keys[0].ID != "signed-off-by" || keys[len(keys)-1].Commits != 1 {
		t.Fatalf("order = %+v", keys)
	}
}

// The first few values to show: most commits first, bots after people, narrowed on request.
func TestReadingTop(t *testing.T) {
	t.Parallel()
	var commits []string
	for range 9 {
		commits = append(commits, "Signed-off-by: dependabot[bot] <bot@users.noreply.example>")
	}
	for i := range 6 {
		commits = append(commits, "Signed-off-by: "+[]string{"Sam Ortiz <sam@tidewater.example>", "Sarah Lind <sarah@tidewater.example>", "Maya Okafor <maya@tidewater.example>"}[i%3])
	}
	x, _ := indexOf(commits...)
	r := x.Reading(Rules{Person: func(name, email string) (string, string, bool) { return name, email, strings.HasSuffix(name, "[bot]") }})

	top, all := r.Top("Signed-off-by", 2, nil)
	if all != 4 || len(top) != 2 || top[0].Bot || top[1].Bot {
		t.Fatalf("top two = %v of %d, want two people before the bot", valueLabels(top), all)
	}
	top, _ = r.Top("signed-off-by", 10, nil)
	if len(top) != 4 || !top[3].Bot || top[3].Commits != 9 {
		t.Fatalf("all four = %v, want the bot last for all its commits", valueLabels(top))
	}
	top, all = r.Top("signed-off-by", 10, func(v Value) bool { return ContainsFold(v.Label, "sa") })
	if all != 2 || len(top) != 2 {
		t.Fatalf("narrowed = %v of %d, want Sam and Sarah", valueLabels(top), all)
	}
	if none, n := r.Top("nothing", 3, nil); n != 0 || none == nil || len(none) != 0 {
		t.Fatalf("a key nobody wrote = %v, %d", none, n)
	}
}

func TestContainsFold(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		s, sub string
		want   bool
	}{
		{"Sarah Lind <sarah@tidewater.example>", "lind", true},
		{"SARAH LIND", "sarah l", true},
		{"Sarah", "sarahh", false},
		{"", "a", false},
		{"anything", "", true},
		{"Zoë", "zoë", true},
	} {
		if got := ContainsFold(c.s, c.sub); got != c.want {
			t.Errorf("ContainsFold(%q, %q) = %v", c.s, c.sub, got)
		}
	}
}

func TestReadingHoldsWhileTheIndexStaysAsItIs(t *testing.T) {
	t.Parallel()
	x, _ := indexOf("Closes: #12")
	r := x.Reading(Rules{})
	if r.Gen() != x.Gen() {
		t.Fatal("a reading just made is already out of date")
	}
	x.mu.Lock()
	x.addLocked(passBatch{strs: []string{"Closes", "#13"}, commits: []batchCommit{{id: make([]byte, 20), pairs: []uint32{0, 1}}}}, nil)
	x.mu.Unlock()
	if r.Gen() == x.Gen() {
		t.Fatal("the index took in a commit, and the reading still says it's current")
	}
	// What it read before is still what it answers.
	if k := keyNamed(t, r, "closes"); k.Commits != 1 {
		t.Fatalf("the old reading changed: %+v", k)
	}
	if k := keyNamed(t, x.Reading(Rules{}), "closes"); k.Commits != 2 {
		t.Fatalf("a new reading = %+v", k)
	}
}
