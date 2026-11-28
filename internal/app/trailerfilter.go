package app

import (
	"context"
	"encoding/json"
	"slices"
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
	// History is the history search in the filter (#981), or nil without one. With picks
	// as well, a commit must have a value picked and be one the search found.
	History *HistoryState `json:"history"`
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
	// history is the history search in the filter, or nil. What it keeps grows as it runs,
	// so what's worked out from it below is kept with the generation it was worked out at.
	history *historyRun
	// keep is what the picks keep while the filter highlights, for marking rows as they're
	// handed out; nil under Only these, where every row of the graph is one of them.
	keep map[string]struct{}

	// What the picks keep as one reading reads them, and the graph rows those commits are on
	// in one graph, sorted. Each is worked out once and kept here (a filter's picks never
	// change: another filter is another repoFilter), since the strip's Previous and Next ask
	// on every click, and on a long history working them out takes a fifth of a second (#899).
	keptBy   *trailerindex.Reading
	keptGen  int
	kept     map[string]struct{}
	rowsBy   *trailerindex.Reading
	rowsGen  int
	rowsIn   *graph.Graph
	rowsLen  int
	rowsKept []int
}

// keeps returns what f keeps as reading reads its picks: what the picks keep, what its
// history search has found, or with both, what both keep.
func (fs *trailerFilters) keeps(f *repoFilter, reading *trailerindex.Reading) map[string]struct{} {
	gen := f.history.generation()
	fs.mu.Lock()
	if f.keptBy == reading && f.keptGen == gen && f.kept != nil {
		defer fs.mu.Unlock()
		return f.kept
	}
	fs.mu.Unlock()
	var keep map[string]struct{}
	if len(f.picks) > 0 {
		keep = reading.Filter(f.picks)
	}
	if f.history != nil {
		found := f.history.keeps()
		if keep == nil {
			keep = found
		} else {
			for id := range keep {
				if _, ok := found[id]; !ok {
					delete(keep, id)
				}
			}
		}
	}
	if keep == nil {
		keep = map[string]struct{}{}
	}
	fs.mu.Lock()
	f.keptBy, f.keptGen, f.kept = reading, gen, keep
	fs.mu.Unlock()
	return keep
}

// rows returns the rows of g that f's picks keep as reading reads them, in order. The slice
// is shared: it mustn't be changed.
func (fs *trailerFilters) rows(f *repoFilter, reading *trailerindex.Reading, g *graph.Graph) []int {
	gen := f.history.generation()
	fs.mu.Lock()
	if f.rowsBy == reading && f.rowsGen == gen && f.rowsIn == g && f.rowsLen == g.Len() && f.rowsKept != nil {
		defer fs.mu.Unlock()
		return f.rowsKept
	}
	fs.mu.Unlock()
	keep := fs.keeps(f, reading)
	ids := make([]string, 0, len(keep))
	for id := range keep {
		ids = append(ids, id)
	}
	rows := rowsOf(g, ids, len(ids)).Rows
	fs.mu.Lock()
	f.rowsBy, f.rowsGen, f.rowsIn, f.rowsLen, f.rowsKept = reading, gen, g, g.Len(), rows
	fs.mu.Unlock()
	return rows
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
	for p, was := range fs.by {
		if fsutil.SamePath(p, path) {
			// A history search the new filter doesn't hold stops: replaced, cleared, or the
			// tab closed.
			if was.history != nil && (f == nil || f.history != was.history) {
				was.history.stop(false)
			}
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
	// A history search in the filter stays, whatever's picked.
	var history *historyRun
	if f := a.trailerFilters.get(r.Path); f != nil {
		history = f.history
	}
	if len(known) == 0 && history == nil {
		a.trailerFilters.set(r.Path, nil)
	} else {
		a.trailerFilters.set(r.Path, &repoFilter{picks: known, only: only, history: history})
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
// keptIn counts the commits of keep that g has rows for, going through whichever is smaller.
func keptIn(g *graph.Graph, keep map[string]struct{}) int {
	n := 0
	if rows := g.Len(); rows <= len(keep) {
		for i := range rows {
			if _, ok := keep[g.ID(i)]; ok {
				n++
			}
		}
		return n
	}
	for id := range keep {
		if _, ok := g.RowOf(id); ok {
			n++
		}
	}
	return n
}

func (a *App) applyTrailerFilter(r *session.Repo) TrailerFilterState {
	return a.applyFilter(r, true)
}

// applyFilter is applyTrailerFilter, but with graph false a graph under Only these keeps
// the commits it was last given: a history search's progress, while the graph is still
// reloading under the last of it (see historyChanged).
func (a *App) applyFilter(r *session.Repo, graph bool) TrailerFilterState {
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
	state.History = f.history.state()
	keep := a.trailerFilters.keeps(f, reading)
	state.Kept = len(keep)
	if r.Focus().Path != "" {
		// Under a path focus most of what the picks keep changed nothing there, and the total
		// is of the path's commits alone (#1062): the count is of the kept commits the graph
		// has read, so it never says more than the total.
		if g, _ := r.Graph(); g != nil {
			state.Kept = keptIn(g, keep)
		}
	}
	if !f.only {
		a.trailerFilters.setHighlighted(f, keep)
		r.SetFilter(nil)
		return state
	}
	a.trailerFilters.setHighlighted(f, nil)
	// The same picks keep more once more of the history has been read, or a commit made,
	// and a history search more as it goes.
	// While a history search runs, a reload stops where the search has got to: nothing older
	// has been searched, so the walk would find nothing more there (#995). Its end moves on as
	// the search does, and once the search stops or ends a reload walks everything.
	var searched any
	until := ""
	if f.history != nil {
		until = f.history.edge()
		searched = []any{f.history.opts, f.history.generation(), until}
	}
	if !graph {
		return state
	}
	sig, _ := json.Marshal([]any{f.picks, reading.Gen(), len(keep), searched})
	r.SetFilter(&session.Filter{Keep: keep, Sig: string(sig), Until: until})
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
	rows := a.trailerFilters.rows(f, a.trailerReading(context.Background(), r), g)
	stretch := max(1, g.Len()/rulerTicks)
	if stretch == 1 {
		return graph.SearchResult{Rows: rows, Total: len(rows)}, nil
	}
	ticks := make([]int, 0, rulerTicks)
	for _, row := range rows {
		if n := len(ticks); n == 0 || row/stretch != ticks[n-1]/stretch {
			ticks = append(ticks, row)
		}
	}
	return graph.SearchResult{Rows: ticks, Total: len(rows)}, nil
}

// TrailerFilterStep returns the row of the next commit the repository's filter keeps after
// row from, or with dir below 0 the one before it, going round from the end to the start as a
// search does: what the strip's buttons step to under Highlight. from of -1 starts before the
// first row. It's -1 when the filter keeps nothing, or there's no filter. Unlike
// TrailerFilterRows it skips nothing, however long the history.
func (a *App) TrailerFilterStep(path string, from, dir int) (int, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return -1, err
	}
	f := a.trailerFilters.get(r.Path)
	if f == nil {
		return -1, nil
	}
	g, err := r.FullGraph(context.Background())
	if err != nil || g == nil {
		return -1, err
	}
	rows := a.trailerFilters.rows(f, a.trailerReading(context.Background(), r), g)
	return stepRow(rows, from, dir), nil
}

// stepRow is the row after from in rows, sorted, or before it with dir below 0, going round
// at either end; -1 when rows is empty.
func stepRow(rows []int, from, dir int) int {
	if len(rows) == 0 {
		return -1
	}
	if dir < 0 {
		i, _ := slices.BinarySearch(rows, from)
		if i == 0 {
			return rows[len(rows)-1]
		}
		return rows[i-1]
	}
	i, found := slices.BinarySearch(rows, from)
	if found {
		i++
	}
	if i >= len(rows) {
		return rows[0]
	}
	return rows[i]
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
