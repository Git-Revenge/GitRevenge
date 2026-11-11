package app

import (
	"context"
	"slices"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
)

// RulerMarks is what the graph's ruler marks: where things are in the part of the history
// that's loaded. Every list is of rows, in order, each row once.
type RulerMarks struct {
	// Loaded is how many rows the marks cover.
	Loaded int `json:"loaded"`
	// Months are the first rows of each month, with their times.
	Months []graph.MonthMark `json:"months"`
	// Tips are the rows of branches, local and remote.
	Tips []int `json:"tips"`
	// Tags are the rows of tags.
	Tags []int `json:"tags"`
	// Notes are the rows of commits with a git note.
	Notes []int `json:"notes"`
}

// RulerMarks says where the branches, tags, noted commits and month boundaries are among the
// rows loaded so far in the repository at path. It never reads more of the history: a ref or
// a note on a commit that isn't loaded yet is simply not marked until it is.
func (a *App) RulerMarks(path string) (RulerMarks, error) {
	none := RulerMarks{Months: []graph.MonthMark{}, Tips: []int{}, Tags: []int{}, Notes: []int{}}
	m, err := a.sessions()
	if err != nil {
		return none, err
	}
	r := m.Get(path)
	if r == nil {
		return none, userError("That repository isn't open.")
	}
	g, _ := r.Graph()
	if g == nil {
		return none, nil
	}
	out := none
	out.Loaded = g.Len()
	out.Months = g.MonthMarks()
	rowsOf := func(ids []string) []int {
		rows := make([]int, 0, len(ids))
		for _, id := range ids {
			if row, ok := g.RowOf(id); ok {
				rows = append(rows, row)
			}
		}
		slices.Sort(rows)
		return slices.Compact(rows)
	}
	var tips, tags []string
	for _, ref := range r.Refs().All {
		switch ref.Kind {
		case git.LocalBranch, git.RemoteBranch:
			tips = append(tips, ref.Commit)
		case git.Tag:
			tags = append(tags, ref.Commit)
		}
	}
	out.Tips, out.Tags = rowsOf(tips), rowsOf(tags)
	// The marks are decoration: without the notes there are just no note ticks.
	if noted, err := r.Git.NotedCommits(context.Background()); err == nil {
		out.Notes = rowsOf(noted)
	}
	return out, nil
}
