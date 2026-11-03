package git

import (
	"context"
	"slices"
	"strings"
)

// TrailerHints is what a repository's history suggests for a new commit's trailers.
type TrailerHints struct {
	// Keys are the trailer keys in use, most used first, as they're usually spelled.
	Keys []string `json:"keys"`
	// People are the "Name <email>" values of Co-authored-by and the like, most used first.
	People []string `json:"people"`
}

const (
	trailerHintCommits = "300"
	maxTrailerHints    = 6
)

// creditKeys are the trailer keys whose values name a person to suggest.
var creditKeys = []string{"co-authored-by", "reviewed-by", "acked-by", "tested-by", "reported-by", "suggested-by", "signed-off-by"}

// TrailerHints reads the trailers of recent commits. They're only suggestions, so a failure
// just means none.
func (r *Repo) TrailerHints(ctx context.Context) TrailerHints {
	hints := TrailerHints{Keys: []string{}, People: []string{}}
	out, err := r.ReadString(ctx, "log", "-"+trailerHintCommits, "--format=%(trailers:only,unfold)")
	if err != nil {
		return hints
	}
	keys, people := newTally(), newTally()
	for line := range strings.Lines(out) {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok || key == "" {
			continue
		}
		keys.add(strings.ToLower(key), key)
		if slices.Contains(creditKeys, strings.ToLower(key)) && strings.Contains(value, "<") {
			people.add(strings.ToLower(value), value)
		}
	}
	hints.Keys = keys.top(maxTrailerHints)
	hints.People = people.top(maxTrailerHints)
	return hints
}

// tally counts spellings of the same thing, remembering the first spelling of each.
type tally struct {
	counts map[string]int
	first  map[string]string
	order  []string
}

func newTally() *tally { return &tally{counts: map[string]int{}, first: map[string]string{}} }

func (t *tally) add(id, spelling string) {
	if t.counts[id] == 0 {
		t.first[id] = spelling
		t.order = append(t.order, id)
	}
	t.counts[id]++
}

// top returns the most counted spellings, ties in the order they were first seen.
func (t *tally) top(n int) []string {
	ids := slices.Clone(t.order)
	slices.SortStableFunc(ids, func(a, b string) int { return t.counts[b] - t.counts[a] })
	out := []string{}
	for _, id := range ids[:min(n, len(ids))] {
		out = append(out, t.first[id])
	}
	return out
}
