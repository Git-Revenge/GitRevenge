// Package semdiff compares two versions of a file for the diff views: which lines changed,
// which words changed inside them, and which lines moved (ADR 0015). The views draw from its
// result, and staging rebuilds a file from the same blocks.
//
// This is the line diff. Reading code by its syntax comes later and keeps this as its
// fallback.
package semdiff

import (
	"context"
	"slices"
)

// Line diff algorithms, spelled as git's diff.algorithm spells them.
const (
	Myers     = "myers"
	Minimal   = "minimal"
	Patience  = "patience"
	Histogram = "histogram"
)

// Algorithms are the line diffs Options.Algorithm takes.
var Algorithms = []string{Myers, Minimal, Patience, Histogram}

// Algorithm returns the algorithm git's diff.algorithm names, or "" for one it doesn't know.
// git takes "default" for Myers.
func Algorithm(name string) string {
	if name == "default" {
		return Myers
	}
	if slices.Contains(Algorithms, name) {
		return name
	}
	return ""
}

// Options say how to compare.
type Options struct {
	// Algorithm is the line diff: one of Algorithms. "" means Myers, git's default.
	Algorithm string `json:"algorithm"`
	// Moves finds runs of lines deleted in one place and added unchanged in another.
	Moves bool `json:"moves"`
}

// Result is a comparison of two texts.
type Result struct {
	// Blocks are the places that changed, in order. The lines between blocks are the same on
	// both sides.
	Blocks []Block `json:"blocks"`
	// Moves are runs of lines deleted in one place and added in another, ordered by where they
	// were added. Each one's lines lie inside blocks.
	Moves []Move `json:"moves"`
	// Algorithm is the line diff that ran. A long file compared by patience or histogram is
	// compared by Myers instead.
	Algorithm string `json:"algorithm"`
	// Added and Removed count lines, moved ones included.
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

// Block replaces old lines [OldStart, OldEnd) with new lines [NewStart, NewEnd). Lines count
// from 0 and keep their line endings, as the views split them. One side may be empty: its
// start is then where the other side's lines are, or were.
type Block struct {
	OldStart int `json:"oldStart"`
	OldEnd   int `json:"oldEnd"`
	NewStart int `json:"newStart"`
	NewEnd   int `json:"newEnd"`
	// Old and New mark the changed words inside the block's lines, when its old and new
	// lines are alike enough to compare word by word. Neither is set when the lines were
	// rewritten, or when either side is empty.
	Old []Span `json:"old,omitempty"`
	New []Span `json:"new,omitempty"`
}

// Span is changed text in one line. From and To count UTF-16 code units from the start of
// the line, which is how the views measure text.
type Span struct {
	Line int `json:"line"`
	From int `json:"from"`
	To   int `json:"to"`
}

// Move is a run of lines deleted at [OldStart, OldEnd) and added at [NewStart, NewEnd), the
// same apart from indentation.
type Move struct {
	OldStart int `json:"oldStart"`
	OldEnd   int `json:"oldEnd"`
	NewStart int `json:"newStart"`
	NewEnd   int `json:"newEnd"`
	// Reindented is set when the lines' indentation changed on the way.
	Reindented bool `json:"reindented"`
}

// Diff compares before with after. It returns ctx's error if ctx ends first.
func Diff(ctx context.Context, before, after string, opts Options) (Result, error) {
	a, b := split(before), split(after)
	algorithm := opts.Algorithm
	if algorithm == "" {
		algorithm = Myers
	}
	if (algorithm == Patience || algorithm == Histogram) && max(a.n(), b.n()) > maxAnchoredLines {
		algorithm = Myers
	}
	blocks, err := lineBlocks(ctx, a, b, algorithm)
	if err != nil {
		return Result{}, err
	}
	if blocks == nil {
		blocks = []Block{} // the views read an empty list, not null
	}
	res := Result{Blocks: blocks, Moves: []Move{}, Algorithm: algorithm}
	for _, blk := range blocks {
		res.Removed += blk.OldEnd - blk.OldStart
		res.Added += blk.NewEnd - blk.NewStart
	}
	var moved movedLines
	if opts.Moves {
		if res.Moves, err = findMoves(ctx, a, b, blocks); err != nil {
			return Result{}, err
		}
		moved = movedSet(res.Moves)
	}
	for i := range res.Blocks {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		markWords(a, b, &res.Blocks[i], moved)
	}
	return res, nil
}

// maxAnchoredLines is the longest file patience and histogram compare. Both recurse once per
// anchor, so a huge file with few unique lines could take a long time; Myers bounds its own
// work.
const maxAnchoredLines = 100_000
