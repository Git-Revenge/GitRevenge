package app

import (
	"context"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// The search box suggests as it's typed in: the trailer keys and values that contain the
// text, each with how many commits picking it would leave. A picked value joins the graph's
// smart filter; the text search itself is always the first row, so nothing typed is ever
// taken for a trailer against the person's wish.

// TrailerSuggestion is one key or value the search box offers.
type TrailerSuggestion struct {
	// Key is the key's ID and KeyLabel how the repository writes it.
	Key      string `json:"key"`
	KeyLabel string `json:"keyLabel"`
	// Value is the value's ID, or "" for a row that offers the key itself: picking that one
	// writes the key in the box, to choose among its values. Label is how to show the value
	// and Kind what it is.
	Value string `json:"value"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	// Commits is how many commits have it, among those the filter already on keeps.
	Commits int `json:"commits"`
}

// TrailerSuggestions is what the search box offers under some text.
type TrailerSuggestions struct {
	Items []TrailerSuggestion `json:"items"`
	// Match is the part of the text the rows were found by, to mark in them: all of it, or
	// what follows the colon when the text names a key.
	Match string `json:"match"`
	// Status says how far reading the history has got: until it's done, the rows and their
	// counts are of what's read so far.
	Status trailerindex.Status `json:"status"`
}

const (
	// trailerSuggestions is how many rows are offered when the request doesn't say.
	trailerSuggestions = 8
	// trailerSuggestedKeys is how many of them may be keys: the values are what's picked.
	trailerSuggestedKeys = 3
)

// TrailerSuggest returns the trailer keys and values of the repository at path that the
// text in the search box could mean. "review" offers the keys and values that contain it;
// "reviewed-by: sa" offers that key's values that contain "sa". Values already in the
// filter aren't offered again, and neither is one that would leave no commits. A limit of 0
// or less means the default.
func (a *App) TrailerSuggest(path, query string, limit int) (TrailerSuggestions, error) {
	out := TrailerSuggestions{Items: []TrailerSuggestion{}}
	r, err := a.openRepo(path)
	if err != nil {
		return out, err
	}
	out.Status = r.Trailers().Status()
	q := strings.TrimSpace(query)
	// "-changelog:none" and "has:reviewed-by" are written out, not picked.
	if q == "" || strings.HasPrefix(q, "-") {
		return out, nil
	}
	if limit <= 0 {
		limit = trailerSuggestions
	}
	reading := a.trailerReading(context.Background(), r)
	var picks []trailerindex.Pick
	if f := a.trailerFilters.get(r.Path); f != nil {
		picks = f.picks
	}
	picked := func(key, value string) bool {
		return slices.ContainsFunc(picks, func(p trailerindex.Pick) bool { return p.Key == key && p.Value == value })
	}
	offer := func(k trailerindex.Key, values []trailerindex.Value) []TrailerSuggestion {
		var rows []TrailerSuggestion
		for _, v := range values {
			if !picked(k.ID, v.ID) {
				rows = append(rows, TrailerSuggestion{Key: k.ID, KeyLabel: k.Label, Value: v.ID, Label: v.Label, Kind: string(v.Kind), Commits: v.Commits})
			}
		}
		return rows
	}

	// "key: text" names a key the repository has: its values.
	if key, rest, ok := git.SplitTrailerQuery(q); ok {
		if k, found := reading.Key(key); found && k.Held == "" {
			lower := strings.ToLower(rest)
			// A few more than asked for, since some may be picked already or leave nothing.
			top, _ := reading.Top(k.ID, limit+len(picks)+limit, func(v trailerindex.Value) bool { return trailerindex.ContainsFold(v.Label, lower) })
			out.Match = rest
			out.Items = firstOf(withinFilter(reading, picks, offer(k, top)), limit)
			return out, nil
		}
	}

	// Anything else: the keys whose name contains it, then the values that do, most commits first.
	lower := strings.ToLower(q)
	out.Match = q
	byValue := func(v trailerindex.Value) bool { return trailerindex.ContainsFold(v.Label, lower) }
	var keys, values []TrailerSuggestion
	for _, k := range reading.Keys() {
		if k.Held != "" {
			continue
		}
		if len(keys) < trailerSuggestedKeys && (trailerindex.ContainsFold(k.Label, lower) || slices.ContainsFunc(k.Spellings, func(s trailerindex.Spelling) bool { return trailerindex.ContainsFold(s.Text, lower) })) {
			keys = append(keys, TrailerSuggestion{Key: k.ID, KeyLabel: k.Label, Kind: string(k.Kind), Commits: k.Commits})
		}
		top, _ := reading.Top(k.ID, limit+len(picks), byValue)
		values = append(values, offer(k, top)...)
	}
	slices.SortStableFunc(values, func(x, y TrailerSuggestion) int { return y.Commits - x.Commits })
	// Counting within a filter is a pass over the history for each key: only the rows that
	// could show are counted.
	values = withinFilter(reading, picks, firstOf(values, 3*limit))
	slices.SortStableFunc(values, func(x, y TrailerSuggestion) int { return y.Commits - x.Commits })
	out.Items = firstOf(append(keys, values...), limit)
	return out, nil
}

func firstOf(rows []TrailerSuggestion, n int) []TrailerSuggestion {
	if rows == nil {
		return []TrailerSuggestion{}
	}
	return rows[:min(n, len(rows))]
}

// withinFilter recounts value rows as picking each would leave the graph, given the picks
// already on, and drops the ones that would leave nothing. A value of a key that's already
// picked is an either-or with those picks, so it's counted within the other keys' picks.
// With no picks the rows are as they were.
func withinFilter(reading *trailerindex.Reading, picks []trailerindex.Pick, rows []TrailerSuggestion) []TrailerSuggestion {
	if len(picks) == 0 || len(rows) == 0 {
		return rows
	}
	byKey := map[string][]int{}
	var order []string
	for i, row := range rows {
		if byKey[row.Key] == nil {
			order = append(order, row.Key)
		}
		byKey[row.Key] = append(byKey[row.Key], i)
	}
	for _, key := range order {
		var others []trailerindex.Pick
		for _, p := range picks {
			if p.Key != key {
				others = append(others, p)
			}
		}
		if len(others) == 0 {
			continue // only this key is picked: its other values count as they are
		}
		at := byKey[key]
		ids := make([]string, len(at))
		for n, i := range at {
			ids[n] = rows[i].Value
		}
		for n, count := range reading.CountWithin(key, ids, reading.Filter(others)) {
			rows[at[n]].Commits = count
		}
	}
	return slices.DeleteFunc(rows, func(row TrailerSuggestion) bool { return row.Commits == 0 })
}
