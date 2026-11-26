package app

import (
	"context"
	"errors"
	"maps"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// The history search (#981): the commits where a text appeared or went away, as part of a
// repository's smart filter, so Only these and Highlight work as they do for trailers and a
// trailer pick and the search must both hold. It goes through the commits the graph shows,
// newest first, in chunks handed to git several at a time, so the top of the graph is right
// first, and each chunk done is progress: how far back it has got, how long the rest will
// take, and where Stop leaves off and Carry on picks up.

// HistoryState is how a repository's history search stands, for the filter strip.
type HistoryState struct {
	Options git.HistoryOptions `json:"options"`
	// Found is how many commits it has found.
	Found int `json:"found"`
	// Searched is how many commits it has been through, newest first, and Total how many it
	// goes through in all, 0 until they're counted.
	Searched int `json:"searched"`
	Total    int `json:"total"`
	// BackTo is the committer date it has got back to: every commit since then is searched.
	// 0 until the first stretch is done.
	BackTo int64 `json:"backTo"`
	// Running is true while it searches. Stopped says it was stopped, to carry on later;
	// neither means it's done, or ended early with a Problem.
	Running bool `json:"running"`
	Stopped bool `json:"stopped"`
	// Ms is how long it has searched, and EstimateMs how long the whole search will take
	// from the rate so far: 0 until there's enough to tell.
	Ms         int64 `json:"ms"`
	EstimateMs int64 `json:"estimateMs"`
	// Problem says in words why it ended before the end, such as files a partial clone
	// never downloaded.
	Problem string `json:"problem"`
}

// historyChunk is how many commits go to one git: big enough that starting git is a small
// part of the time, small enough that progress moves and Stop is quick. The first chunks are
// smaller, from historyFirstChunk up, so the newest commits found show at once: on the
// kernel, a first chunk of 2,000 put the first one up after five seconds.
const (
	historyChunk      = 2000
	historyFirstChunk = 250
)

// chunkSize is how many commits chunk n of a search holds: doubling from first up to most.
func chunkSize(n, first, most int) int {
	if n >= 16 {
		return most
	}
	return min(most, first<<n)
}

// historyWorkers is how many gits search at once: half the machine, at most six.
var historyWorkers = min(max(runtime.NumCPU()/2, 1), 6)

// historyEvery is how often a running search re-applies the filter and tells the window,
// so the graph follows it without reloading on every chunk.
const historyEvery = time.Second

// historyRun is one repository's history search.
type historyRun struct {
	opts git.HistoryOptions

	mu    sync.Mutex
	found map[string]struct{}
	// gen changes whenever found does: the filter's signature carries it.
	gen      int
	searched int
	total    int
	backTo   int64
	// after is the last commit of the stretch searched from the newest: Carry on starts
	// after it.
	after   string
	running bool
	stopped bool
	problem string
	spent   time.Duration
	since   time.Time
	cancel  context.CancelFunc
	// changes holds what HistoryChanges has worked out, by commit.
	changes map[string][]git.HistoryChange
}

func newHistoryRun(o git.HistoryOptions) *historyRun {
	return &historyRun{opts: o, found: map[string]struct{}{}, changes: map[string][]git.HistoryChange{}}
}

// generation is what the filter's keep set depends on; 0 without a search.
func (h *historyRun) generation() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.gen
}

// keeps returns a copy of what the search has found.
func (h *historyRun) keeps() map[string]struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return maps.Clone(h.found)
}

func (h *historyRun) state() *HistoryState {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	spent := h.spent
	if h.running {
		spent += time.Since(h.since)
	}
	st := &HistoryState{Options: h.opts, Found: len(h.found), Searched: h.searched, Total: h.total, BackTo: h.backTo, Running: h.running, Stopped: h.stopped, Ms: spent.Milliseconds(), Problem: h.problem}
	// A rate needs a few seconds and some commits behind it.
	if h.total > 0 && h.searched >= historyChunk && spent > 2*time.Second {
		st.EstimateMs = int64(float64(spent.Milliseconds()) * float64(h.total) / float64(h.searched))
	}
	return st
}

// stop ends a running search, keeping what it found; stopped says whether to offer Carry on.
func (h *historyRun) stop(stopped bool) {
	if h == nil {
		return
	}
	h.mu.Lock()
	cancel := h.cancel
	if h.running {
		h.stopped = stopped
	}
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// startHistory runs the search from where it is, in the background, telling the window as
// it goes. A search already running is left to run.
func (a *App) startHistory(r *session.Repo, h *historyRun) {
	ctx, cancel := context.WithCancel(r.Git.Context())
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		cancel()
		return
	}
	h.cancel = cancel
	h.running, h.stopped, h.problem = true, false, ""
	h.since = time.Now()
	after := h.after
	h.mu.Unlock()
	opts := a.historyLogOptions(r)
	go func() {
		defer cancel()
		err := a.searchHistory(ctx, r, h, opts, after)
		h.mu.Lock()
		h.spent += time.Since(h.since)
		h.running = false
		switch {
		case err == nil:
			h.stopped = false
		case errors.Is(err, context.Canceled):
			// Stopped, replaced or closed: the state says which.
		default:
			h.stopped = false
			h.problem = historyProblem(err, h.backTo)
		}
		h.mu.Unlock()
		a.historyChanged(r, h, true)
	}()
}

// historyLogOptions is what the graph shows of the repository: what a search goes through.
func (a *App) historyLogOptions(r *session.Repo) git.LogOptions {
	opts := git.LogOptions{}
	m, err := a.sessions()
	if err != nil {
		return opts
	}
	rules := session.Rules(m.Settings().Get().Repo(a.repoKey(r.Path)))
	if rs, err := r.Git.Remotes(context.Background()); err == nil {
		for _, rm := range rs {
			opts.Remotes = append(opts.Remotes, rm.Name)
		}
	}
	opts.Exclude = rules.Hide()
	return opts
}

// searchHistory goes through the commits after after (all of them for ""), newest first, in
// chunks on historyWorkers gits at once.
func (a *App) searchHistory(ctx context.Context, r *session.Repo, h *historyRun, opts git.LogOptions, after string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	since := h.opts.Since

	go func() {
		n, err := r.Git.HistoryCount(ctx, opts, since)
		if err != nil {
			return
		}
		h.mu.Lock()
		h.total = n
		h.mu.Unlock()
	}()

	type chunk struct {
		n       int
		commits []git.HistoryCommit
	}
	type result struct {
		n     int
		last  git.HistoryCommit
		size  int
		found []string
		err   error
	}
	size := a.historyChunkSize
	if size == 0 {
		size = historyChunk
	}
	work := make(chan chunk)
	results := make(chan result)
	var workers sync.WaitGroup
	for range historyWorkers {
		workers.Go(func() {
			for c := range work {
				ids := make([]string, len(c.commits))
				for i, hc := range c.commits {
					ids[i] = hc.ID
				}
				found, err := r.Git.HistoryChunk(ctx, h.opts, ids)
				select {
				case results <- result{n: c.n, last: c.commits[len(c.commits)-1], size: len(c.commits), found: found, err: err}:
				case <-ctx.Done():
					return
				}
			}
		})
	}

	// The commits, newest first, cut into chunks: the listing waits for a worker to be free.
	listed := make(chan error, 1)
	go func() {
		defer close(work)
		skipping := after != ""
		n := 0
		var cur []git.HistoryCommit
		send := func() bool {
			select {
			case work <- chunk{n: n, commits: cur}:
				n++
				cur = nil
				return true
			case <-ctx.Done():
				return false
			}
		}
		err := r.Git.HistoryCommits(ctx, opts, since, func(c git.HistoryCommit) error {
			if skipping {
				skipping = c.ID != after
				return nil
			}
			cur = append(cur, c)
			if len(cur) == chunkSize(n, min(historyFirstChunk, size), size) && !send() {
				return ctx.Err()
			}
			return nil
		})
		if err == nil && len(cur) > 0 && !send() {
			err = ctx.Err()
		}
		listed <- err
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	// Chunks finish out of order; the stretch searched from the newest moves on only when
	// every chunk before it is done.
	done := map[int]result{}
	next := 0
	var firstErr error
	for res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
				cancel()
			}
			continue
		}
		h.mu.Lock()
		for _, id := range res.found {
			h.found[id] = struct{}{}
		}
		if len(res.found) > 0 {
			h.gen++
		}
		done[res.n] = res
		for d, ok := done[next]; ok; d, ok = done[next] {
			delete(done, next)
			h.searched += d.size
			h.backTo, h.after = d.last.At, d.last.ID
			next++
		}
		h.mu.Unlock()
		a.historyChanged(r, h, false)
	}
	if firstErr != nil {
		return firstErr
	}
	if err := <-listed; err != nil {
		return err
	}
	return ctx.Err()
}

// historyChanged re-applies the filter, so the graph shows what the search has found, and
// tells the window: at most once a historyEvery while it runs, and always at the end.
func (a *App) historyChanged(r *session.Repo, h *historyRun, final bool) {
	a.historyNotes.Lock()
	if a.historyNoted == nil {
		a.historyNoted = map[*historyRun]time.Time{}
	}
	if !final && time.Since(a.historyNoted[h]) < historyEvery {
		a.historyNotes.Unlock()
		return
	}
	if final {
		delete(a.historyNoted, h)
	} else {
		a.historyNoted[h] = time.Now()
	}
	a.historyNotes.Unlock()
	// Only while it's still this repository's search.
	if f := a.trailerFilters.get(r.Path); f == nil || f.history != h {
		return
	}
	// Under Only these, each new set of commits found reloads the graph, and under a filter
	// that keeps few a reload walks the whole history before it shows: on a big one, longer
	// than a second. A reload started before the last one ends would stop it, and the graph
	// would never narrow, so while one is under way the graph keeps what it was last given
	// and only the strip hears. The end of the search always reaches the graph.
	graph := final
	if !graph {
		_, loading := r.Graph()
		graph = !loading
	}
	st := a.applyFilter(r, graph)
	a.emit("history:progress", HistoryProgress{Path: r.Path, Filter: st})
}

// HistoryProgress is sent as "history:progress" while a history search runs and when it
// ends: the repository's filter as it now stands.
type HistoryProgress struct {
	Path   string             `json:"path"`
	Filter TrailerFilterState `json:"filter"`
}

// historyProblem says in words why a search ended early.
func historyProblem(err error, backTo int64) string {
	var pe *git.PatternError
	switch {
	case errors.As(err, &pe):
		return "Not a regular expression: " + pe.Reason + "."
	case errors.Is(err, git.ErrNotDownloaded):
		where := "This is a partial clone, and some commits' files were never downloaded, so the search stopped"
		if backTo > 0 {
			where += " at " + time.Unix(backTo, 0).UTC().Format("Jan 2, 2006")
		}
		return where + ". A search never downloads anything."
	case errors.Is(err, git.ErrFindText):
		return "The history search looks for text within one line."
	}
	return err.Error()
}

// SetHistorySearch starts searching the repository's history for o, as part of its smart
// filter, stopping a search already under way. Text left empty takes the search out of the
// filter. It answers at once, with the filter as it stands; "history:progress" events follow.
func (a *App) SetHistorySearch(path string, o git.HistoryOptions) (TrailerFilterState, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerFilterState{Picks: []FilterPick{}}, err
	}
	if o.Text != "" && (len(o.Text) > 4096 || strings.ContainsAny(o.Text, "\r\n")) {
		return TrailerFilterState{Picks: []FilterPick{}}, userError("The history search looks for text within one line.")
	}
	f := a.trailerFilters.get(r.Path)
	var picks []trailerindex.Pick
	only := true
	if f != nil {
		picks, only = f.picks, f.only
		f.history.stop(false)
	}
	if o.Text == "" {
		if len(picks) == 0 {
			a.trailerFilters.set(r.Path, nil)
		} else {
			a.trailerFilters.set(r.Path, &repoFilter{picks: picks, only: only})
		}
		return a.applyTrailerFilter(r), nil
	}
	h := newHistoryRun(o)
	a.trailerFilters.set(r.Path, &repoFilter{picks: picks, only: only, history: h})
	a.startHistory(r, h)
	return a.applyTrailerFilter(r), nil
}

// StopHistorySearch stops the repository's history search, keeping what it found, to
// carry on later.
func (a *App) StopHistorySearch(path string) (TrailerFilterState, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerFilterState{Picks: []FilterPick{}}, err
	}
	if f := a.trailerFilters.get(r.Path); f != nil {
		f.history.stop(true)
	}
	return a.applyTrailerFilter(r), nil
}

// CarryOnHistorySearch goes on with a stopped history search from where it stopped.
func (a *App) CarryOnHistorySearch(path string) (TrailerFilterState, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerFilterState{Picks: []FilterPick{}}, err
	}
	if f := a.trailerFilters.get(r.Path); f != nil && f.history != nil {
		a.startHistory(r, f.history)
	}
	return a.applyTrailerFilter(r), nil
}

// HistoryChanges says how each of some commits the history search found changed its text,
// file by file: the +1 and -1 beside them. Commits it hasn't found are left out.
func (a *App) HistoryChanges(path string, ids []string) (map[string][]git.HistoryChange, error) {
	out := map[string][]git.HistoryChange{}
	r, err := a.openRepo(path)
	if err != nil {
		return out, err
	}
	f := a.trailerFilters.get(r.Path)
	if f == nil || f.history == nil {
		return out, nil
	}
	h := f.history
	var ask []string
	h.mu.Lock()
	for _, id := range ids {
		if _, ok := h.found[id]; !ok {
			continue
		}
		if c, ok := h.changes[id]; ok {
			out[id] = c
		} else {
			ask = append(ask, id)
		}
	}
	h.mu.Unlock()
	if len(ask) == 0 {
		return out, nil
	}
	// Not stopped by a newer call: each asks about its own commits (the graph's rows in view,
	// the commit panel's one), and an answer cut short would read as no change. Closing the
	// repository stops it.
	got, err := r.Git.HistoryChanges(r.Git.Context(), h.opts, ask)
	if errors.Is(err, context.Canceled) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	h.mu.Lock()
	for _, id := range ask {
		h.changes[id] = got[id]
		if got[id] != nil {
			out[id] = got[id]
		}
	}
	h.mu.Unlock()
	return out, nil
}
