package app

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/fsutil"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// A smart filter is the values picked to filter a repository's graph by. Search moves
// between matches; a filter changes what the graph shows. It has two modes: Only these, in
// which commits that don't match leave the graph, and Highlight, in which nothing leaves and
// the matches stand out.
//
// A filter lasts until it's cleared or the tab closes, as a focus does: it's kept here and
// nowhere on disk. Which mode a repository last used is the window's to remember.

// FilterPick is one value picked to filter by, with what to show for it.
type FilterPick struct {
	// Key and Value are IDs, as the Smart filters panel has them.
	Key   string `json:"key"`
	Value string `json:"value"`
	// KeyLabel and ValueLabel are how to show them, and Kind what the value is.
	KeyLabel   string `json:"keyLabel"`
	ValueLabel string `json:"valueLabel"`
	Kind       string `json:"kind"`
}

// TrailerFilterState is a repository's smart filter as it stands.
type TrailerFilterState struct {
	// Picks are the values picked. Picks of one key are either-or; picks of different keys
	// must all hold.
	Picks []FilterPick `json:"picks"`
	// Only is true for Only these, false for Highlight.
	Only bool `json:"only"`
	// Kept is how many commits the picks keep, in the whole history read so far. Under a
	// focus or with branches hidden the graph may show fewer.
	Kept int `json:"kept"`
	// Total is how many commits the graph would show with no filter; 0 until it's counted.
	Total int `json:"total"`
}

// trailerFilters holds each open repository's filter.
type trailerFilters struct {
	mu sync.Mutex
	by map[string]*repoFilter
	// applying is held while a filter is read and given to the graph, so that two of those
	// take turns: the end of a trailer read applies the filter while the window may be
	// changing it, and whichever goes second must be the one that saw the latest.
	applying sync.Mutex
}

type repoFilter struct {
	picks []trailerindex.Pick
	only  bool
	// keep is what the picks keep while the filter highlights, for marking rows as they're
	// handed out; nil under Only these, where every row of the graph is one of them.
	keep map[string]struct{}
}

// highlighted returns the commits a repository's filter keeps while it highlights, or nil
// when nothing is being highlighted.
func (fs *trailerFilters) highlighted(path string) map[string]struct{} {
	if f := fs.get(path); f != nil {
		fs.mu.Lock()
		defer fs.mu.Unlock()
		return f.keep
	}
	return nil
}

// setHighlighted records what a repository's filter keeps while it highlights.
func (fs *trailerFilters) setHighlighted(f *repoFilter, keep map[string]struct{}) {
	fs.mu.Lock()
	f.keep = keep
	fs.mu.Unlock()
}

func (fs *trailerFilters) get(path string) *repoFilter {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for p, f := range fs.by {
		if fsutil.SamePath(p, path) {
			return f
		}
	}
	return nil
}

func (fs *trailerFilters) set(path string, f *repoFilter) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for p := range fs.by {
		if fsutil.SamePath(p, path) {
			delete(fs.by, p)
		}
	}
	if f == nil {
		return
	}
	if fs.by == nil {
		fs.by = map[string]*repoFilter{}
	}
	fs.by[path] = f
}

// SetTrailerFilter sets the smart filter of the repository at path: the values picked, by
// their IDs, and whether only they show. No picks clears it. A pick of a value the
// repository no longer has is dropped.
func (a *App) SetTrailerFilter(path string, picks []trailerindex.Pick, only bool) (TrailerFilterState, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerFilterState{Picks: []FilterPick{}}, err
	}
	reading := a.trailerReading(context.Background(), r)
	var known []trailerindex.Pick
	for _, p := range picks {
		if _, ok := reading.Value(p.Key, p.Value); ok {
			known = append(known, trailerindex.Pick{Key: reading.KeyID(p.Key), Value: p.Value})
		}
	}
	if len(known) == 0 {
		a.trailerFilters.set(r.Path, nil)
	} else {
		a.trailerFilters.set(r.Path, &repoFilter{picks: known, only: only})
	}
	return a.applyTrailerFilter(r), nil
}

// TrailerFilter returns the smart filter of the repository at path.
func (a *App) TrailerFilter(path string) (TrailerFilterState, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerFilterState{Picks: []FilterPick{}}, err
	}
	return a.applyTrailerFilter(r), nil
}

// applyTrailerFilter works out what the repository's filter keeps, as the trailers are read
// now, and has the graph show it: only those commits under Only these, everything under
// Highlight or with no filter. Asking again with nothing changed reloads nothing.
func (a *App) applyTrailerFilter(r *session.Repo) TrailerFilterState {
	a.trailerFilters.applying.Lock()
	defer a.trailerFilters.applying.Unlock()
	state := TrailerFilterState{Picks: []FilterPick{}}
	// How many commits there are with no filter: counted exactly once a load has finished,
	// and git's quick count, which is a commit or two over where there are stashes, until then.
	on, looked, total := r.Filtered()
	state.Total = max(total, looked)
	if g, loading := r.Graph(); !loading && g != nil {
		if on && looked > 0 {
			state.Total = looked
		} else if !on {
			state.Total = g.Len()
		}
	}
	f := a.trailerFilters.get(r.Path)
	if f == nil {
		r.SetFilter(nil)
		return state
	}
	reading := a.trailerReading(context.Background(), r)
	for _, p := range f.picks {
		k, _ := reading.Key(p.Key)
		v, _ := reading.Value(p.Key, p.Value)
		state.Picks = append(state.Picks, FilterPick{Key: p.Key, Value: p.Value, KeyLabel: k.Label, ValueLabel: v.Label, Kind: string(v.Kind)})
	}
	state.Only = f.only
	keep := reading.Filter(f.picks)
	state.Kept = len(keep)
	if !f.only {
		a.trailerFilters.setHighlighted(f, keep)
		r.SetFilter(nil)
		return state
	}
	a.trailerFilters.setHighlighted(f, nil)
	// The same picks keep more once more of the history has been read, or a commit made.
	sig, _ := json.Marshal([]any{f.picks, reading.Gen(), len(keep)})
	r.SetFilter(&session.Filter{Keep: keep, Sig: string(sig)})
	return state
}

// rulerTicks is about how many rows TrailerFilterRows gives: a ruler is a few hundred
// pixels tall, and a tick can't be finer than one.
const rulerTicks = 4000

// TrailerFilterRows returns graph rows of the commits the repository's filter keeps, for
// the ruler under Highlight: one for each stretch of the history that has any, which is
// all of them until the history is longer than a ruler can show. Total is exact. Which
// rows keep their ink isn't read from here: each row says so itself (see GraphRows). It
// looks through the whole history, as a search does, so it first reads the rest of the graph.
func (a *App) TrailerFilterRows(path string) (graph.SearchResult, error) {
	none := graph.SearchResult{Rows: []int{}}
	r, err := a.openRepo(path)
	if err != nil {
		return none, err
	}
	f := a.trailerFilters.get(r.Path)
	if f == nil {
		return none, nil
	}
	g, err := r.FullGraph(context.Background())
	if err != nil || g == nil {
		return none, err
	}
	keep := a.trailerReading(context.Background(), r).Filter(f.picks)
	ids := make([]string, 0, len(keep))
	for id := range keep {
		ids = append(ids, id)
	}
	all := rowsOf(g, ids, len(ids))
	stretch := max(1, g.Len()/rulerTicks)
	if stretch == 1 {
		return all, nil
	}
	ticks := make([]int, 0, rulerTicks)
	for _, row := range all.Rows {
		if n := len(ticks); n == 0 || row/stretch != ticks[n-1]/stretch {
			ticks = append(ticks, row)
		}
	}
	all.Rows = ticks
	return all, nil
}

// TrailerFilterDays says how many commits were authored in each of some spans of time, in
// the history the filter of the repository at path is looking through: what a day held
// before Only these took commits out of it, for "1 of 3 commits" on its line. spans is
// starts and ends in turn, in Unix seconds, an end not included. The list is empty with no
// filter narrowing the graph, or while the history is still being looked through.
func (a *App) TrailerFilterDays(path string, spans []int64) ([]int, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return []int{}, err
	}
	pairs := make([][2]int64, 0, len(spans)/2)
	for i := 0; i+1 < len(spans); i += 2 {
		pairs = append(pairs, [2]int64{spans[i], spans[i+1]})
	}
	counts, ok := r.Authored(pairs)
	if !ok {
		return []int{}, nil
	}
	return counts, nil
}

// TrailerFilterLines returns the trailers of one commit that the filter of the repository
// at path picks, as the commit wrote them: the lines to mark in its details. A person is
// matched as they're shown, so a line under an old name is marked too.
func (a *App) TrailerFilterLines(path, sha string) ([]git.Trailer, error) {
	none := []git.Trailer{}
	r, err := a.openRepo(path)
	if err != nil {
		return none, err
	}
	f := a.trailerFilters.get(r.Path)
	if f == nil {
		return none, nil
	}
	lines, held := r.Trailers().Trailers(sha)
	hits, ok := a.trailerReading(context.Background(), r).Picked(sha, f.picks)
	// The reading may be a moment older than the index: only a commit both know is marked.
	if !held || !ok || len(hits) != len(lines) {
		return none, nil
	}
	for i, line := range lines {
		if hits[i] {
			none = append(none, line)
		}
	}
	return none, nil
}
