package trailerindex

import (
	"cmp"
	"slices"
	"strings"
)

// Real histories are inconsistent, and some of what's inconsistent isn't the app's to
// settle: whether "Rviewed-by" is a slip for "Reviewed-by" or a key of its own, whether
// "Ticket" and "Refs" are one thing. Those are questions for whoever knows the repository.
// This finds them, and says what it found, and leaves the answer to a rule.

// The kinds of question.
const (
	// CareSpelling: a key one letter off a much more used one.
	CareSpelling = "spelling"
	// CareSame: two keys holding the same things.
	CareSame = "same"
	// CarePerson: one person written two ways, which the mailmap would join.
	CarePerson = "person"
)

// Question is one thing about a repository's trailers that needs someone's answer.
type Question struct {
	Kind string `json:"kind"`
	// Key is the key that would stay and Other the one that would be read as it, each by
	// ID, with how it's written and how many commits have it. A question about a person
	// has neither.
	Key          string `json:"key"`
	KeyLabel     string `json:"keyLabel"`
	Commits      int    `json:"commits"`
	Other        string `json:"other"`
	OtherLabel   string `json:"otherLabel"`
	OtherCommits int    `json:"otherCommits"`
	// Shared is how many values two keys have in common.
	Shared int `json:"shared"`
	// Examples shows what was found: values the two keys share, or the ways a person is written.
	Examples []string `json:"examples"`
}

// Joined says what a reading joined without asking, so nothing happens out of sight.
type Joined struct {
	// Spellings are the keys written more than one way that are read as one: letter case
	// and word endings.
	Spellings []Key `json:"spellings"`
	// Several is how many different lines name several values at once, as "#12, #14" does.
	Several int `json:"several"`
	// Bots is how many of the people named are bots.
	Bots int `json:"bots"`
}

const (
	// A slip is much rarer than what it's a slip for: at most this share of its commits.
	slipShare = 0.2
	// Keys shorter than this are one letter off too many things to ask about.
	slipMinLen = 4
	// Two keys hold the same things when at least this share of the smaller one's values,
	// and at least this many, are the other's too.
	sameShare = 0.2
	sameLeast = 2
	// How many examples a question carries.
	careExamples = 3
)

// ticketVerbs are the keys that say a commit settles a ticket, in the order one would stand
// for the others: joined, the row reads "Closes".
var ticketVerbs = []string{"closes", "fixes", "resolves"}

// Care returns the questions a reading raises about its keys, the most telling first, and
// how many there are in all: at most limit are returned, since a long history can raise
// hundreds. apart says whether two keys, by ID, were asked about before and are to stay
// apart; nil never says so. People written two ways are asked about apart (see People).
func (r *Reading) Care(apart func(a, b string) bool, limit int) (questions []Question, total int) {
	skip := func(a, b string) bool { return apart != nil && apart(a, b) }
	var out []Question
	pair := func(kind string, key, other int, shared int, examples []string) {
		k, o := r.keys[key], r.keys[other]
		out = append(out, Question{Kind: kind, Key: k.ID, KeyLabel: k.Label, Commits: k.Commits, Other: o.ID, OtherLabel: o.Label, OtherCommits: o.Commits, Shared: shared, Examples: examples})
	}

	// A key one letter off a much more used one. A slip is usually rare enough to be held
	// back, so held keys are looked at too.
	asked := map[[2]int]bool{}
	for a := range r.keys {
		for b := range r.keys {
			big, small := r.keys[a], r.keys[b]
			if a == b || len(small.ID) < slipMinLen || float64(small.Commits) > slipShare*float64(big.Commits) {
				continue
			}
			if !oneOff(big.ID, small.ID) || skip(big.ID, small.ID) {
				continue
			}
			asked[[2]int{a, b}], asked[[2]int{b, a}] = true, true
			pair(CareSpelling, a, b, 0, nil)
		}
	}

	// Two keys holding the same things: tickets or labels, never people, who sign off one
	// commit and review the next under keys that mean different things.
	holds := func(k Key) bool {
		return k.Held == "" && (k.Kind == KindTicket || k.Kind == KindLabel || k.Kind == KindText)
	}
	byValue := map[string][]int{}
	for k, key := range r.keys {
		if !holds(key) {
			continue
		}
		for _, v := range r.values[k] {
			byValue[v.ID] = append(byValue[v.ID], k)
		}
	}
	shared := map[[2]int][]string{}
	for id, keys := range byValue {
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				at := [2]int{keys[i], keys[j]}
				shared[at] = append(shared[at], id)
			}
		}
	}
	same := func(a, b int, ids []string) {
		// The one on more commits stays, except that a ticket verb stands for the others
		// in the order they're listed.
		key, other := a, b
		if r.keys[b].Commits > r.keys[a].Commits {
			key, other = b, a
		}
		va, vb := slices.Index(ticketVerbs, r.keys[a].ID), slices.Index(ticketVerbs, r.keys[b].ID)
		if va >= 0 && vb >= 0 {
			key, other = a, b
			if vb < va {
				key, other = b, a
			}
		}
		if asked[[2]int{a, b}] || skip(r.keys[a].ID, r.keys[b].ID) {
			return
		}
		asked[[2]int{a, b}], asked[[2]int{b, a}] = true, true
		slices.Sort(ids)
		examples := make([]string, 0, careExamples)
		for _, id := range ids[:min(careExamples, len(ids))] {
			for _, v := range r.values[key] {
				if v.ID == id {
					examples = append(examples, v.Label)
					break
				}
			}
		}
		pair(CareSame, key, other, len(ids), examples)
	}
	for at, ids := range shared {
		smaller := min(len(r.values[at[0]]), len(r.values[at[1]]))
		if len(ids) >= sameLeast && float64(len(ids)) >= sameShare*float64(smaller) {
			same(at[0], at[1], ids)
		}
	}
	// The ticket verbs are offered to each other whether or not they share a ticket: a
	// ticket is closed once, so they rarely do.
	for i, a := range ticketVerbs {
		for _, b := range ticketVerbs[i+1:] {
			ka, okA := r.keyAt[a]
			kb, okB := r.keyAt[b]
			if okA && okB && ka != kb && holds(r.keys[ka]) && holds(r.keys[kb]) && r.keys[ka].Kind == KindTicket && r.keys[kb].Kind == KindTicket {
				same(ka, kb, shared[[2]int{min(ka, kb), max(ka, kb)}])
			}
		}
	}

	// The most telling first: a slip, then the same things. Within each, the one whose
	// answer changes the most commits, which is the rarer key's: every slip of a key on a
	// million commits would otherwise come before anything else.
	rank := map[string]int{CareSpelling: 0, CareSame: 1}
	slices.SortFunc(out, func(a, b Question) int {
		return cmp.Or(
			cmp.Compare(rank[a.Kind], rank[b.Kind]),
			cmp.Compare(b.OtherCommits, a.OtherCommits),
			cmp.Compare(b.Commits, a.Commits),
			cmp.Compare(a.Key+a.Other, b.Key+b.Other),
		)
	})
	total = len(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []Question{}
	}
	return out, total
}

// People returns the people written two ways, those on the most commits first, and how
// many there are in all: at most limit are returned. A long history has thousands, and
// they aren't a rule's to answer: they join once the mailmap says so.
func (r *Reading) People(limit int) (questions []Question, total int) {
	var out []Question
	// One person, two spellings: the same address under two names, or the same name at two
	// addresses. They join once the mailmap says so, which isn't decided here.
	type who struct {
		label   string
		commits int
	}
	people := map[int32]*who{}
	for k := range r.keys {
		for _, v := range r.values[k] {
			if v.Kind != KindPerson || v.Bot {
				continue
			}
			n := r.personAt(v.ID)
			if n < 0 {
				continue
			}
			if people[n] == nil {
				people[n] = &who{label: v.Label}
			}
			people[n].commits += v.Commits
		}
	}
	byName, byEmail := map[string][]*who{}, map[string][]*who{}
	for _, p := range people {
		name, email := splitPerson(p.label)
		if name != "" && email != "" {
			byName[strings.ToLower(name)] = append(byName[strings.ToLower(name)], p)
			byEmail[strings.ToLower(email)] = append(byEmail[strings.ToLower(email)], p)
		}
	}
	seen := map[string]bool{}
	for _, groups := range []map[string][]*who{byEmail, byName} {
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			slices.SortFunc(group, func(a, b *who) int { return cmp.Or(cmp.Compare(b.commits, a.commits), cmp.Compare(a.label, b.label)) })
			labels := make([]string, len(group))
			commits := 0
			for i, p := range group {
				labels[i] = p.label
				commits += p.commits
			}
			sig := strings.Join(labels, "\n")
			if seen[sig] {
				continue
			}
			seen[sig] = true
			out = append(out, Question{Kind: CarePerson, Commits: commits, Examples: labels[:min(len(labels), careExamples+1)]})
		}
	}

	slices.SortFunc(out, func(a, b Question) int {
		return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(strings.Join(a.Examples, "\n"), strings.Join(b.Examples, "\n")))
	})
	total = len(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []Question{}
	}
	return out, total
}

// personAt returns which value a person's ID names, or -1.
func (r *Reading) personAt(id string) int32 {
	r.peopleOnce.Do(func() {
		r.peopleAt = map[string]int32{}
		for n, c := range r.vals {
			if c.kind == KindPerson {
				r.peopleAt[c.id] = int32(n)
			}
		}
	})
	if n, ok := r.peopleAt[id]; ok {
		return n
	}
	return -1
}

// splitPerson cuts "Name <address>" in two; a label that isn't one has no address.
func splitPerson(label string) (name, email string) {
	open := strings.LastIndex(label, " <")
	if open < 0 || !strings.HasSuffix(label, ">") {
		return label, ""
	}
	return label[:open], label[open+2 : len(label)-1]
}

// oneOff reports whether two keys are one slip of the fingers apart: a letter dropped,
// added or wrong, or two neighbours swapped.
func oneOff(a, b string) bool {
	if a == b {
		return false
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	if len(a)-len(b) > 1 {
		return false
	}
	// The first place they differ.
	i := 0
	for i < len(b) && a[i] == b[i] {
		i++
	}
	if len(a) > len(b) {
		// One letter more: the rest must line up after skipping it.
		return a[i+1:] == b[i:]
	}
	// The same length: one letter wrong, or two neighbours swapped.
	if a[i+1:] == b[i+1:] {
		return true
	}
	return i+1 < len(a) && a[i] == b[i+1] && a[i+1] == b[i] && a[i+2:] == b[i+2:]
}

// Joined returns what the reading joined without asking.
func (r *Reading) Joined() Joined {
	j := Joined{Spellings: []Key{}, Several: len(r.several)}
	for _, k := range r.keys {
		if len(k.Spellings) > 1 {
			j.Spellings = append(j.Spellings, k)
		}
	}
	for _, c := range r.vals {
		if c.kind == KindPerson && c.bot {
			j.Bots++
		}
	}
	return j
}
