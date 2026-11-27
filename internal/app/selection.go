package app

import (
	"cmp"
	"context"
	"slices"
	"strconv"

	"github.com/git-revenge/gitrevenge/internal/graph"
)

// Several commits selected in the graph (#1018).

// maxSelectionListed is how many of a selection's commits are listed, oldest first: the
// panel beside the graph lists these and counts the rest.
const maxSelectionListed = 100

// maxSelectionFiles is how many of a selection's commits are read for the files they change.
const maxSelectionFiles = 500

// maxSelectionFileList is how many of those files are listed, most changed first.
const maxSelectionFileList = 500

// SelectionPick is a run of selected graph rows, Lo to Hi and both included, with the commits
// the graph showed at its ends. An end the graph hadn't loaded is "".
type SelectionPick struct {
	Lo   int    `json:"lo"`
	Hi   int    `json:"hi"`
	LoID string `json:"loId"`
	HiID string `json:"hiId"`
}

// Selection is what the graph's selection holds.
type Selection struct {
	// IDs is every selected commit, oldest first: the order history has them in.
	IDs []string `json:"ids"`
	// Commits lists the oldest maxSelectionListed of them, oldest first.
	Commits []graph.Commit `json:"commits"`
	// Authors is how many people wrote them; Author names them when it's one.
	Authors int    `json:"authors"`
	Author  string `json:"author,omitempty"`
	// First and Last are the earliest and latest author times among them.
	First int64 `json:"first"`
	Last  int64 `json:"last"`
	// Merges is how many of them are merges.
	Merges int `json:"merges"`
	// Head says the checked-out commit is one of them.
	Head bool `json:"head"`
	// Stale says the graph changed under the picks: its rows aren't the ones picked. The graph
	// finds its commits again once its reload arrives.
	Stale bool `json:"stale"`
}

// SelectedCommits reads the commits of the graph rows picked.
func (a *App) SelectedCommits(path string, picks []SelectionPick) (Selection, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return Selection{}, err
	}
	g, _ := r.Graph()
	if g == nil {
		return Selection{Stale: true, IDs: []string{}, Commits: []graph.Commit{}}, nil
	}
	n := g.Len()
	for _, p := range picks {
		if p.Lo < 0 || p.Hi < p.Lo || p.Hi >= n || (p.LoID != "" && g.ID(p.Lo) != p.LoID) || (p.HiID != "" && g.ID(p.Hi) != p.HiID) {
			return Selection{Stale: true, IDs: []string{}, Commits: []graph.Commit{}}, nil
		}
	}
	// Oldest first is the graph's order backwards: the lowest rows are the newest.
	picks = slices.Clone(picks)
	slices.SortFunc(picks, func(x, y SelectionPick) int { return cmp.Compare(y.Lo, x.Lo) })
	head := ""
	for _, ref := range r.Refs().All {
		if ref.Head {
			head = ref.Commit
		}
	}
	out := Selection{IDs: []string{}, Commits: []graph.Commit{}}
	authors := map[string]bool{}
	seen := map[int]bool{}
	for _, p := range picks {
		cs := g.Commits(p.Lo, p.Hi-p.Lo+1)
		for i := len(cs) - 1; i >= 0; i-- {
			c := cs[i]
			if seen[c.Index] {
				continue
			}
			seen[c.Index] = true
			out.IDs = append(out.IDs, c.ID)
			if len(out.Commits) < maxSelectionListed {
				out.Commits = append(out.Commits, c)
			}
			authors[c.Author] = true
			out.Author = c.Author
			if out.First == 0 || c.AuthorTime < out.First {
				out.First = c.AuthorTime
			}
			out.Last = max(out.Last, c.AuthorTime)
			if c.Parents > 1 {
				out.Merges++
			}
			out.Head = out.Head || c.ID == head
		}
	}
	out.Authors = len(authors)
	if out.Authors != 1 {
		out.Author = ""
	}
	return out, nil
}

// SelectionFile is a file some of a selection's commits change.
type SelectionFile struct {
	Path string `json:"path"`
	// Commits are the ones that change it, oldest first.
	Commits []string `json:"commits"`
}

// SelectionFiles is what a selection's commits change.
type SelectionFiles struct {
	// Files are the most changed first, then by path; at most maxSelectionFileList.
	Files []SelectionFile `json:"files"`
	// Total is how many files there are in all, listed or not.
	Total int `json:"total"`
	// Read is how many commits were read: the oldest maxSelectionFiles of those asked about.
	Read int `json:"read"`
}

// SelectionFiles reads which files the commits change, from the oldest of them on (ids come
// oldest first). It reads trees only, so a partial clone downloads nothing.
func (a *App) SelectionFiles(path string, ids []string) (SelectionFiles, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return SelectionFiles{}, err
	}
	ids = ids[:min(len(ids), maxSelectionFiles)]
	byCommit, err := r.Git.FilesByCommit(context.Background(), ids)
	if err != nil {
		return SelectionFiles{}, writeErr(err)
	}
	at := map[string]int{}
	var files []SelectionFile
	for _, id := range ids {
		for _, f := range byCommit[id] {
			i, ok := at[f]
			if !ok {
				i = len(files)
				at[f] = i
				files = append(files, SelectionFile{Path: f})
			}
			files[i].Commits = append(files[i].Commits, id)
		}
	}
	slices.SortStableFunc(files, func(x, y SelectionFile) int {
		return cmp.Or(cmp.Compare(len(y.Commits), len(x.Commits)), cmp.Compare(x.Path, y.Path))
	})
	out := SelectionFiles{Files: files[:min(len(files), maxSelectionFileList)], Total: len(files), Read: len(ids)}
	if out.Files == nil {
		out.Files = []SelectionFile{}
	}
	return out, nil
}

// commitsLabel names commits for the undo list: the one commit, or how many.
func commitsLabel(ids []string) string {
	if len(ids) == 1 {
		return shortRef(ids[0])
	}
	return strconv.Itoa(len(ids)) + " commits"
}

// CommitRows finds each commit's graph row, as CommitRow does one: after a reload, the
// selection follows its commits to their new rows. A commit with no row gets rowAbsent, or
// rowFilteredOut when a smart filter narrowed the graph.
func (a *App) CommitRows(path string, ids []string) ([]int, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	g, _ := r.Graph()
	out := make([]int, len(ids))
	for i, id := range ids {
		// Most are in the rows already read; only the others make the graph read on.
		if g != nil {
			if row, ok := g.RowOf(id); ok {
				out[i] = row
				continue
			}
		}
		row, err := a.CommitRow(path, id)
		if err != nil {
			return nil, err
		}
		out[i] = row
	}
	return out, nil
}
