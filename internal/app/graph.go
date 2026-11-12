package app

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// maxGraphPage caps one GraphRows request so a bad caller can't ask for a million rows.
const maxGraphPage = 1000

// GraphPage is a window of commit graph rows.
type GraphPage struct {
	Rows []graph.Row `json:"rows"`
	// Refs maps commit IDs in Rows to the refs pointing at them.
	Refs map[string][]git.Ref `json:"refs"`
	// Total is the number of rows laid out so far.
	Total   int  `json:"total"`
	Loading bool `json:"loading"`
	// Paused means the reader stopped ahead of the viewport; asking for later rows resumes it.
	Paused bool `json:"paused"`
	// Estimate is about how many rows the whole history has, or 0 until known.
	Estimate int `json:"estimate"`
}

// GraphProgress is the payload of the "graph:progress" event.
type GraphProgress struct {
	Path string `json:"path"`
	Rows int    `json:"rows"`
	Done bool   `json:"done"`
	// RefsReady means refs just arrived during a first load; re-fetch rows for labels.
	RefsReady bool `json:"refsReady"`
	// Paused means the reader stopped ahead of the viewport.
	Paused bool `json:"paused"`
	// Estimate is about how many rows the whole history has, or 0 until known.
	Estimate int `json:"estimate"`
	// Reset means a reloaded graph replaced the displayed one; cached rows are stale.
	Reset bool   `json:"reset"`
	Error string `json:"error"`
}

// GraphRows returns up to count rows of the commit graph starting at start. Asking for rows
// past those loaded makes a paused reader read on; they arrive with later progress events.
func (a *App) GraphRows(path string, start, count int) (GraphPage, error) {
	m, err := a.sessions()
	if err != nil {
		return GraphPage{}, err
	}
	r := m.Get(path)
	if r == nil {
		return GraphPage{}, userError("That repository isn't open.")
	}
	count = min(count, maxGraphPage)
	r.DemandRows(start + count)
	g, loading := r.Graph()
	if g == nil {
		return GraphPage{Rows: []graph.Row{}, Refs: map[string][]git.Ref{}, Loading: true}, nil
	}
	paused, estimate := r.GraphStatus()
	rows := g.Rows(start, count)
	idx := r.Refs()
	refs := map[string][]git.Ref{}
	for _, row := range rows {
		if rs := idx.ByCommit[row.ID]; len(rs) > 0 {
			refs[row.ID] = rs
		}
	}
	return GraphPage{Rows: rows, Refs: refs, Total: g.Len(), Loading: loading, Paused: paused, Estimate: estimate}, nil
}

// Refs lists the branches, remote branches, tags and stashes shown in the graph.
func (a *App) Refs(path string) ([]git.Ref, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	refs := r.Refs().All
	if refs == nil {
		refs = []git.Ref{}
	}
	return refs, nil
}

// CommitRow returns the graph row of a commit, reading on through the history until it
// loads, or -1 if the commit isn't in the history. Progress events report the reading.
func (a *App) CommitRow(path, id string) (int, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return -1, err
	}
	row, ok, err := r.FindRow(context.Background(), id)
	if err != nil || !ok {
		return -1, err
	}
	return row, nil
}

// openRepo returns the open repository at path.
func (a *App) openRepo(path string) (*session.Repo, error) {
	m, err := a.sessions()
	if err != nil {
		return nil, err
	}
	r := m.Get(path)
	if r == nil {
		return nil, userError("That repository isn't open.")
	}
	return r, nil
}

func (a *App) emitGraphProgress(path string, p session.GraphProgress) {
	ev := GraphProgress{Path: path, Rows: p.Rows, Done: p.Done, RefsReady: p.RefsReady, Paused: p.Paused, Estimate: p.Estimate, Reset: p.Reset}
	if p.Err != nil {
		ev.Error = p.Err.Error()
	}
	a.emit("graph:progress", ev)
}

// maxSearchRows caps how many match positions are returned; the total is still exact.
const maxSearchRows = 10000

// SearchGraph finds commits whose subject or author contains query, or whose ID starts
// with it. A query like "Co-authored-by: ada" or "Closes: #12" searches trailers instead,
// and "has:note" finds the commits with a git note.
// It searches the whole history, so it first reads the rest of it.
func (a *App) SearchGraph(path, query string) (graph.SearchResult, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return graph.SearchResult{}, err
	}
	g, err := r.FullGraph(context.Background())
	if err != nil {
		return graph.SearchResult{}, err
	}
	if g == nil {
		return graph.SearchResult{Rows: []int{}}, nil
	}
	if isNotesQuery(query) {
		ids, err := r.Git.NotedCommits(context.Background())
		if err != nil {
			return graph.SearchResult{}, err
		}
		return rowsOf(g, ids, maxSearchRows), nil
	}
	if q, ok := git.ParseTrailerQuery(query); ok {
		ctx, done := a.searches.start(path)
		defer done()
		ids, err := r.Git.CommitsWithTrailer(ctx, q)
		if errors.Is(err, context.Canceled) {
			return graph.SearchResult{Rows: []int{}}, nil // superseded by a newer search
		}
		if err != nil {
			return graph.SearchResult{}, err
		}
		return rowsOf(g, ids, maxSearchRows), nil
	}
	return g.Search(query, maxSearchRows), nil
}

// searches cancels a repository's previous trailer search when a new one starts: the search
// box searches on each keystroke, and a trailer search reads every commit message.
type searches struct {
	mu      sync.Mutex
	running map[string]*context.CancelFunc
}

// start returns the context for a new search on path, cancelling the one before it, and the
// function to call when the search ends.
func (s *searches) start(path string) (context.Context, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running == nil {
		s.running = map[string]*context.CancelFunc{}
	}
	if prev := s.running[path]; prev != nil {
		(*prev)()
	}
	ctx, cancel := context.WithCancel(context.Background())
	mine := &cancel
	s.running[path] = mine
	return ctx, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		cancel()
		if s.running[path] == mine { // a newer search may have replaced it
			delete(s.running, path)
		}
	}
}

// rowsOf returns the graph rows of the commits ids, in graph order, at most limit of them.
func rowsOf(g *graph.Graph, ids []string, limit int) graph.SearchResult {
	rows := make([]int, 0, len(ids))
	for _, id := range ids {
		if row, ok := g.RowOf(id); ok {
			rows = append(rows, row)
		}
	}
	slices.Sort(rows)
	res := graph.SearchResult{Rows: rows, Total: len(rows)}
	if len(rows) > limit {
		res.Rows = rows[:limit]
	}
	return res
}

// maxLaneNames caps how many lanes one LaneNames call names: a view shows a few dozen.
const maxLaneNames = 400

// LaneName says who a lane of the graph belongs to.
type LaneName struct {
	// Color is the lane's colour number, as rows and their lines carry it: it names the lane.
	Color int `json:"color"`
	// Row and Commit are the commit that started the lane; Row is -1 when that isn't known
	// (a Trunk whose tip isn't read yet, or a colour no lane has).
	Row    int    `json:"row"`
	Commit string `json:"commit"`
	// Refs are the branches and tags on that commit, when it's the lane's own tip. A lane a
	// merge opened has none: the branch it was is gone, or was never here.
	Refs []git.Ref `json:"refs"`
}

// LaneNames says which commit started each of the given lanes of the graph at path, and the
// refs on it, for a view that names its lanes. Lanes are given by colour number.
func (a *App) LaneNames(path string, colors []int) ([]LaneName, error) {
	out := []LaneName{}
	r, err := a.openRepo(path)
	if err != nil {
		return out, err
	}
	g, _ := r.Graph()
	if g == nil {
		return out, nil
	}
	idx := r.Refs()
	for _, c := range colors[:min(len(colors), maxLaneNames)] {
		name := LaneName{Color: c, Row: -1, Refs: []git.Ref{}}
		if row, tip, ok := g.LaneStart(c); ok {
			name.Row, name.Commit = row, g.ID(row)
			if tip {
				name.Refs = append(name.Refs, idx.ByCommit[name.Commit]...)
			}
		}
		out = append(out, name)
	}
	return out, nil
}
