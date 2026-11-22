package trailerindex

import (
	"cmp"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// A Reading is the index as something to filter by. The index holds what people wrote, key
// and value; a reading says which spellings are the same key, what kind of thing each value
// is, which values are the same thing written two ways, and which keys aren't worth
// offering as a filter.
//
// Projects write trailers in every way there is, and nothing here assumes one of them. What
// was written is kept. Only spellings are joined without asking: letter case and word
// endings of a key, a person under the names a mailmap joins, a ticket as a number and as
// its address. Fixes, Closes and Resolves stay three keys: some projects mean the same by
// them and some don't.
//
// A reading is worked out from the index and the rules in one go, and holds for as long as
// neither changes. The index never has to be read again for a change of rules.

// Kind is what a value is. It's told from the value, never from the key: one key can hold
// values of several kinds, as in "Fixes: #12" and "Fixes: 656c296".
type Kind string

// The kinds of value.
const (
	// KindPerson is a name and an address, or an address alone.
	KindPerson Kind = "person"
	// KindTicket is a reference to an issue or a pull request: "#12", an address of one, or
	// whatever one of the repository's smart links matches.
	KindTicket Kind = "ticket"
	// KindCommit is the id of a commit in the repository.
	KindCommit Kind = "commit"
	// KindLabel is one of a few values used over many commits, as in "Changelog: fixed".
	KindLabel Kind = "label"
	// KindIdentifier is a value that's different on nearly every commit, as a Change-Id is.
	KindIdentifier Kind = "identifier"
	// KindText is anything else.
	KindText Kind = "text"
)

// Why a key isn't offered as a filter. Search still finds a held key.
const (
	// HeldIdentifier: its values are different on nearly every commit, so they group nothing.
	HeldIdentifier = "identifier"
	// HeldEverywhere: nearly every commit has it, so it filters nothing.
	HeldEverywhere = "everywhere"
	// HeldRare: so few commits have it that it's likely a sentence shaped like a trailer.
	HeldRare = "rare"
)

// The thresholds, set against three histories: this project's own (570 commits), the
// Tidewater demo (1,500) and the Linux kernel (1.5 million).
const (
	// everywhereShare is how much of a history a key is on before it filters nothing. The
	// kernel's Signed-off-by is on 92% of commits. This project's Co-authored-by is on 83%,
	// and which co-author is still worth filtering by.
	everywhereShare = 0.9
	// everywhereFrom is how many commits a history needs before that's judged: in a
	// repository's first weeks every key is on most commits.
	everywhereFrom = 50
	// rareUnder is how many commits a key needs. Of the kernel's 2,176 keys, 1,425 are on
	// one commit and 266 on two, nearly all of them sentences with a colon in.
	rareUnder = 3
	// identifierShare is how many of a key's plain values are different before they're
	// identifiers, judged once there are identifierFrom of them. The kernel's Link,
	// Message-Id and Change-Id are all above 0.95.
	identifierShare = 0.8
	identifierFrom  = 10
	// labelShare is how few of a key's plain values are different for them to be labels,
	// judged once there are labelFrom of them. The kernel's Assisted-by is at 0.09.
	labelShare = 0.25
	labelFrom  = 5
)

// Ticket is one reference to an issue or a pull request found in a value.
type Ticket struct {
	// Text is the reference as written, and Href where it leads, if that's known. Two
	// references with one Href are the same ticket.
	Text string
	Href string
}

// Rules is what a reading needs to know about a repository beyond its trailers. Every part
// may be left out.
type Rules struct {
	// Person returns who a name and address written in a trailer is shown as, which is
	// where the mailmap comes in, and whether they're a bot.
	Person func(name, email string) (shownName, shownEmail string, bot bool)
	// Tickets returns the tickets a whole value names by the repository's smart links, or
	// nil when it isn't made of tickets.
	Tickets func(value string) []Ticket
	// Commits says which of these ids, full or abbreviated, name a commit in the
	// repository, and gives the full id of each that does.
	Commits func(ids []string) map[string]string
	// Total is how many commits the history has, for telling a key that's on nearly all of
	// them. 0 leaves that unjudged.
	Total int
}

// Spelling is one way a key is written, and how many commits write it so.
type Spelling struct {
	Text    string `json:"text"`
	Commits int    `json:"commits"`
}

// Key is one key to filter by: every spelling of it, joined.
type Key struct {
	// ID names the key in questions to the reading. Label is its most written spelling.
	ID        string     `json:"id"`
	Label     string     `json:"label"`
	Spellings []Spelling `json:"spellings"`
	// Kind is what most of its values are.
	Kind Kind `json:"kind"`
	// Commits is how many commits have it, Lines how many trailers, and Values how many
	// different values, after joining.
	Commits int `json:"commits"`
	Lines   int `json:"lines"`
	Values  int `json:"values"`
	// Held says why the key isn't offered as a filter, or "" when it is.
	Held string `json:"held"`
	// Known marks a key in common use across projects, such as Signed-off-by.
	Known bool `json:"known"`
}

// Value is one value of a key: every spelling of it, joined.
type Value struct {
	// ID names the value in questions to the reading. Label is how to show it.
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  Kind   `json:"kind"`
	// Commits is how many commits have the key with this value.
	Commits int `json:"commits"`
	// Bot marks a person the bot author patterns match.
	Bot bool `json:"bot,omitempty"`
	// Href is where a ticket leads, when a smart link or the value itself says.
	Href string `json:"href,omitempty"`
}

// Reading is an index read through a set of rules. It's safe to ask from several
// goroutines, and never changes.
type Reading struct {
	gen    uint64
	keys   []Key
	keyAt  map[string]int
	values [][]Value

	// What Commits scans: the index as it was, and what each of its strings is here.
	idSize int
	ids    []byte
	first  []uint32
	pairs  []uint32
	// keyOf is the key a string is a spelling of, or -1. valueOf is the value a string
	// names, or -1; a string that names several, as "#12, #14" does, is in several.
	keyOf   []int32
	valueOf []int32
	several map[uint32][]int32
	// vals are the values, across keys: a person is the same one under every key.
	vals []canon
}

// canon is one value, however it was spelled.
type canon struct {
	id, label, href string
	// kind is person, ticket, commit or text here. Whether text is a label or an identifier
	// depends on the key it's under.
	kind Kind
	bot  bool
	// bare marks an address with no name. "Cc: <stable@tidewater.example>" is somebody; a
	// Message-Id is written the same way and is nobody. Which it is shows under a key: a
	// different one on nearly every commit is an identifier.
	bare bool
}

// endings maps a key, in lower case, to the spelling its word endings are joined under.
var endings = map[string]string{
	"close": "closes", "closed": "closes",
	"fix": "fixes", "fixed": "fixes",
	"resolve": "resolves", "resolved": "resolves",
	"ref": "refs", "reference": "refs", "references": "refs",
}

// known are the keys in common use, by ID. A known key used once is still a trailer.
var known = map[string]bool{
	"signed-off-by": true, "co-authored-by": true, "co-developed-by": true, "reviewed-by": true,
	"acked-by": true, "tested-by": true, "reported-by": true, "requested-by": true, "suggested-by": true,
	"cc": true, "fixes": true, "closes": true, "resolves": true, "refs": true, "link": true, "see-also": true,
}

// KeyID returns the ID of the key a spelling belongs to: letter case and word endings
// don't make a different key.
func KeyID(spelling string) string {
	id := strings.ToLower(strings.TrimSpace(spelling))
	if joined, ok := endings[id]; ok {
		return joined
	}
	return id
}

var (
	// A reference by number, with or without the repository it's in: "#12", "tide/api#12".
	ticketNumber = regexp.MustCompile(`^(?:[\w.-]+/[\w.-]+)?#\d+$`)
	// The address of an issue, a pull or merge request, or a bug, on the hosts and trackers
	// in common use.
	ticketAddress = regexp.MustCompile(`^https?://\S+?(?:/issues/\d+|/pull/\d+|/pulls/\d+|/merge_requests/\d+|/show_bug\.cgi\?id=\d+|/browse/[A-Z][A-Z0-9_]*-\d+)(?:[/?#]\S*)?$`)
	// What may be a commit's id at the start of a value: "656c296", "656c296 (\"subject\")".
	commitStart = regexp.MustCompile(`^[0-9a-fA-F]{7,64}(?:$|[\s(:,.;])`)
)

// ticketsIn returns the tickets a value names, or nil when it isn't made of tickets: one
// reference, or several with commas between.
func ticketsIn(value string, rules Rules) []Ticket {
	if rules.Tickets != nil {
		if found := rules.Tickets(value); len(found) > 0 {
			return found
		}
	}
	one := func(s string) (Ticket, bool) {
		switch {
		case ticketNumber.MatchString(s):
			return Ticket{Text: s}, true
		case strings.HasPrefix(s, "http") && ticketAddress.MatchString(s):
			return Ticket{Text: s, Href: s}, true
		}
		return Ticket{}, false
	}
	if t, ok := one(value); ok {
		return []Ticket{t}
	}
	if !strings.Contains(value, ",") {
		return nil
	}
	var out []Ticket
	for part := range strings.SplitSeq(value, ",") {
		t, ok := one(strings.TrimSpace(part))
		if !ok {
			return nil
		}
		out = append(out, t)
	}
	return out
}

// personIn reads a value as a person: "Name <address>", with anything after it left off, or
// an address alone.
func personIn(value string) (git.Identity, bool) {
	if open := strings.LastIndexByte(value, '<'); open >= 0 {
		id := git.ParseIdentity(value)
		if strings.Contains(id.Email, "@") && !strings.ContainsAny(id.Email, " \t") {
			return id, true
		}
		return git.Identity{}, false
	}
	if at := strings.IndexByte(value, '@'); at > 0 && at < len(value)-1 && !strings.ContainsAny(value, " \t,;:()") && strings.Count(value, "@") == 1 {
		return git.Identity{Email: value}, true
	}
	return git.Identity{}, false
}

func identityLabel(id git.Identity) string {
	if id.Name == "" {
		return id.Email
	}
	return id.Name + " <" + id.Email + ">"
}

// Reading reads the index, as it stands, through rules. On a long history it takes a
// moment, so keep the result for as long as the index and the rules stay as they are (see
// Gen).
func (x *Index) Reading(rules Rules) *Reading {
	// What's held only grows, and what's there is never changed, so the read works on the
	// index as it was at this moment without holding anyone up.
	x.mu.RLock()
	r := &Reading{gen: x.gen, idSize: x.idSize, ids: x.ids, first: x.first, pairs: x.pairs, keyAt: map[string]int{}}
	strs := x.strs
	x.mu.RUnlock()
	commits := len(r.first) - 1

	// Which strings are keys and which are values: most are one or the other.
	isKey, isValue := make([]bool, len(strs)), make([]bool, len(strs))
	for i := 0; i < len(r.pairs); i += 2 {
		isKey[r.pairs[i]], isValue[r.pairs[i+1]] = true, true
	}

	// Keys: each spelling's ID.
	r.keyOf = make([]int32, len(strs))
	spelling := map[int32][]uint32{}
	for n, s := range strs {
		r.keyOf[n] = -1
		if !isKey[n] {
			continue
		}
		id := KeyID(s)
		at, ok := r.keyAt[id]
		if !ok {
			at = len(r.keys)
			r.keyAt[id] = at
			r.keys = append(r.keys, Key{ID: id, Known: known[id]})
		}
		r.keyOf[n] = int32(at)
		spelling[int32(at)] = append(spelling[int32(at)], uint32(n))
	}

	// Values: what each string names. Commit ids are asked about together, at the end.
	r.valueOf = make([]int32, len(strs))
	r.several = map[uint32][]int32{}
	at := map[string]int32{}
	value := func(c canon) int32 {
		n, ok := at[c.id]
		if !ok {
			n = int32(len(r.vals))
			at[c.id] = n
			r.vals = append(r.vals, c)
			return n
		}
		// A ticket is shown by its shortest spelling: "#14" before its address.
		if c.kind == KindTicket && len(c.label) < len(r.vals[n].label) {
			r.vals[n].label = c.label
		}
		return n
	}
	var maybeCommits []uint32
	for n, s := range strs {
		r.valueOf[n] = -1
		if !isValue[n] {
			continue
		}
		if who, ok := personIn(s); ok {
			bot := false
			if rules.Person != nil {
				who.Name, who.Email, bot = rules.Person(who.Name, who.Email)
			}
			label := identityLabel(who)
			r.valueOf[n] = value(canon{id: "person:" + label, label: label, kind: KindPerson, bot: bot, bare: who.Name == ""})
			continue
		}
		if tickets := ticketsIn(s, rules); len(tickets) > 0 {
			var all []int32
			for _, t := range tickets {
				id := t.Text
				if t.Href != "" {
					id = strings.TrimRight(t.Href, "/")
				}
				all = append(all, value(canon{id: "ticket:" + id, label: t.Text, href: t.Href, kind: KindTicket}))
			}
			r.valueOf[n] = all[0]
			if len(all) > 1 {
				r.several[uint32(n)] = all
			}
			continue
		}
		if rules.Commits != nil && commitStart.MatchString(s) {
			maybeCommits = append(maybeCommits, uint32(n))
			continue
		}
		r.valueOf[n] = value(canon{id: "text:" + s, label: s, kind: KindText})
	}
	if len(maybeCommits) > 0 {
		start := func(s string) string {
			end := 0
			for end < len(s) && isHexDigit(s[end]) {
				end++
			}
			return strings.ToLower(s[:end])
		}
		asked := make([]string, 0, len(maybeCommits))
		seen := map[string]bool{}
		for _, n := range maybeCommits {
			if id := start(strs[n]); !seen[id] {
				seen[id] = true
				asked = append(asked, id)
			}
		}
		full := rules.Commits(asked)
		for _, n := range maybeCommits {
			s := strs[n]
			if id, ok := full[start(s)]; ok {
				r.valueOf[n] = value(canon{id: "commit:" + id, label: s, kind: KindCommit})
			} else {
				r.valueOf[n] = value(canon{id: "text:" + s, label: s, kind: KindText})
			}
		}
	}

	// One pass over the commits: how many have each key, and each value under each key. A
	// commit counts once for a key and once for a value, however many times it says it.
	type tally struct {
		commits, lines int
		by             map[int32]int32
		spelled        map[uint32]int
	}
	tallies := make([]tally, len(r.keys))
	for i := range tallies {
		tallies[i].by, tallies[i].spelled = map[int32]int32{}, map[uint32]int{}
	}
	var seenKeys []int32
	var seenPairs []uint64
	var one [1]int32
	for c := range commits {
		mine := r.pairs[r.first[c]:r.first[c+1]]
		seenKeys, seenPairs = seenKeys[:0], seenPairs[:0]
		for j := 0; j < len(mine); j += 2 {
			k := r.keyOf[mine[j]]
			t := &tallies[k]
			t.lines++
			if !slices.Contains(seenKeys, k) {
				seenKeys = append(seenKeys, k)
				t.commits++
				t.spelled[mine[j]]++
			}
			named := r.several[mine[j+1]]
			if named == nil {
				one[0] = r.valueOf[mine[j+1]]
				named = one[:]
			}
			for _, v := range named {
				pair := uint64(uint32(k))<<32 | uint64(uint32(v))
				if !slices.Contains(seenPairs, pair) {
					seenPairs = append(seenPairs, pair)
					t.by[v]++
				}
			}
		}
	}

	// Each key: its spellings, its values, what kind most of them are, and whether it's
	// worth offering.
	r.values = make([][]Value, len(r.keys))
	for k := range r.keys {
		key, t := &r.keys[k], &tallies[k]
		key.Commits, key.Lines, key.Values = t.commits, t.lines, len(t.by)
		for _, n := range spelling[int32(k)] {
			key.Spellings = append(key.Spellings, Spelling{Text: strs[n], Commits: t.spelled[n]})
		}
		slices.SortFunc(key.Spellings, func(a, b Spelling) int {
			return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(a.Text, b.Text))
		})
		key.Label = key.Spellings[0].Text

		// Plain values are labels when a few of them cover many commits, identifiers when
		// nearly every commit has its own, and text otherwise.
		plain, plainUses := 0, 0
		for v, n := range t.by {
			if r.vals[v].kind == KindText || r.vals[v].bare {
				plain++
				plainUses += int(n)
			}
		}
		plainKind := KindText
		switch {
		case plainUses >= identifierFrom && float64(plain) >= identifierShare*float64(plainUses):
			plainKind = KindIdentifier
		case plainUses >= labelFrom && float64(plain) <= labelShare*float64(plainUses):
			plainKind = KindLabel
		}
		uses := map[Kind]int{}
		vals := make([]Value, 0, len(t.by))
		for v, n := range t.by {
			c := r.vals[v]
			kind := c.kind
			// An address with no name is a person unless this key has a new one on nearly
			// every commit.
			if kind == KindText || (c.bare && plainKind == KindIdentifier) {
				kind = plainKind
			}
			uses[kind] += int(n)
			vals = append(vals, Value{ID: c.id, Label: c.label, Kind: kind, Commits: int(n), Bot: c.bot, Href: c.href})
		}
		slices.SortFunc(vals, func(a, b Value) int {
			return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(a.Label, b.Label))
		})
		r.values[k] = vals
		// The kind most of its values are; between equals, the more telling one.
		for _, kind := range []Kind{KindPerson, KindTicket, KindCommit, KindLabel, KindIdentifier, KindText} {
			if uses[kind] > uses[key.Kind] || key.Kind == "" {
				key.Kind = kind
			}
		}

		plainly := key.Kind == KindPerson || key.Kind == KindTicket || key.Kind == KindCommit
		switch {
		case key.Commits < rareUnder && !key.Known && !plainly:
			key.Held = HeldRare
		case key.Kind == KindIdentifier:
			key.Held = HeldIdentifier
		case rules.Total >= everywhereFrom && float64(key.Commits) >= everywhereShare*float64(rules.Total):
			key.Held = HeldEverywhere
		}
	}

	// Most commits first, as the keys are listed. The numbers keyOf holds follow.
	order := make([]int, len(r.keys))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		return cmp.Or(cmp.Compare(r.keys[b].Commits, r.keys[a].Commits), cmp.Compare(r.keys[a].ID, r.keys[b].ID))
	})
	moved := make([]int32, len(order))
	keys, values := make([]Key, len(order)), make([][]Value, len(order))
	for to, from := range order {
		moved[from] = int32(to)
		keys[to], values[to] = r.keys[from], r.values[from]
		r.keyAt[keys[to].ID] = to
	}
	r.keys, r.values = keys, values
	for n, k := range r.keyOf {
		if k >= 0 {
			r.keyOf[n] = moved[k]
		}
	}
	return r
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// Gen says which state of the index this reading is of: it's still good while the index's
// Gen is the same.
func (r *Reading) Gen() uint64 { return r.gen }

// Gen counts the index's changes. A reading made at one Gen holds until the next.
func (x *Index) Gen() uint64 {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.gen
}

// Keys returns every key, most commits first, held ones among them.
func (r *Reading) Keys() []Key { return slices.Clone(r.keys) }

// Key returns one key by its ID or by any spelling of it.
func (r *Reading) Key(id string) (Key, bool) {
	at, ok := r.keyAt[KeyID(id)]
	if !ok {
		return Key{}, false
	}
	return r.keys[at], true
}

// Values returns a key's values, most commits first. The key is named by its ID or by any
// spelling of it.
func (r *Reading) Values(key string) []Value {
	at, ok := r.keyAt[KeyID(key)]
	if !ok {
		return []Value{}
	}
	return slices.Clone(r.values[at])
}

// Top returns up to n of a key's values to show, and how many there are in all: most
// commits first, with bots after everyone else, since a bot's thousand commits say less
// than a person's ten. match narrows the values to those it accepts; nil takes them all.
// The key is named by its ID or by any spelling of it.
func (r *Reading) Top(key string, n int, match func(Value) bool) (top []Value, matching int) {
	top = []Value{}
	at, ok := r.keyAt[KeyID(key)]
	if !ok {
		return top, 0
	}
	var bots []Value
	for _, v := range r.values[at] {
		if match != nil && !match(v) {
			continue
		}
		matching++
		switch {
		case v.Bot && len(bots) < n:
			bots = append(bots, v)
		case !v.Bot && len(top) < n:
			top = append(top, v)
		}
	}
	if room := n - len(top); room > 0 {
		top = append(top, bots[:min(room, len(bots))]...)
	}
	return top, matching
}

// ContainsFold reports whether s contains sub, whatever the case of either's ASCII
// letters. sub must already be in lower case. It allocates nothing, which matters when a
// keystroke asks it of a million values.
func ContainsFold(s, lowerSub string) bool {
	if lowerSub == "" {
		return true
	}
	for i := 0; i+len(lowerSub) <= len(s); i++ {
		j := 0
		for j < len(lowerSub) {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != lowerSub[j] {
				break
			}
			j++
		}
		if j == len(lowerSub) {
			return true
		}
	}
	return false
}

// CommitsWhere returns the ids of the commits that have a key with any value match
// accepts, in one pass however many values that is. The key is named by its ID or by any
// spelling of it. A nil match takes every commit with the key.
func (r *Reading) CommitsWhere(key string, match func(Value) bool) []string {
	out := []string{}
	k, ok := r.keyAt[KeyID(key)]
	if !ok {
		return out
	}
	var wanted []bool
	if match != nil {
		at := make(map[string]int32, len(r.vals))
		for n, c := range r.vals {
			at[c.id] = int32(n)
		}
		wanted = make([]bool, len(r.vals))
		some := false
		for _, v := range r.values[k] {
			if match(v) {
				wanted[at[v.ID]], some = true, true
			}
		}
		if !some {
			return out
		}
	}
	has := func(n uint32) bool {
		if wanted == nil {
			return true
		}
		if several := r.several[n]; several != nil {
			for _, v := range several {
				if wanted[v] {
					return true
				}
			}
			return false
		}
		return wanted[r.valueOf[n]]
	}
	for c := 0; c < len(r.first)-1; c++ {
		mine := r.pairs[r.first[c]:r.first[c+1]]
		for j := 0; j < len(mine); j += 2 {
			if r.keyOf[mine[j]] == int32(k) && has(mine[j+1]) {
				out = append(out, hex.EncodeToString(r.ids[c*r.idSize:(c+1)*r.idSize]))
				break
			}
		}
	}
	return out
}

// Commits returns the ids of the commits that have a key with a value, both named by ID. An
// empty value asks for every commit with the key.
func (r *Reading) Commits(key, value string) []string {
	out := []string{}
	k, ok := r.keyAt[KeyID(key)]
	if !ok {
		return out
	}
	want := int32(-1)
	if value != "" {
		want = -2
		for n, c := range r.vals {
			if c.id == value {
				want = int32(n)
				break
			}
		}
		if want == -2 {
			return out
		}
	}
	for c := 0; c < len(r.first)-1; c++ {
		mine := r.pairs[r.first[c]:r.first[c+1]]
		for j := 0; j < len(mine); j += 2 {
			if r.keyOf[mine[j]] != int32(k) {
				continue
			}
			if want >= 0 && r.valueOf[mine[j+1]] != want && !slices.Contains(r.several[mine[j+1]], want) {
				continue
			}
			out = append(out, hex.EncodeToString(r.ids[c*r.idSize:(c+1)*r.idSize]))
			break
		}
	}
	return out
}
