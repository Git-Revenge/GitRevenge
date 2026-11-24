// Package semdiff compares two versions of a file for the diff views: which lines changed,
// what changed inside them, and which lines moved (ADR 0015). The views draw from its
// result, and staging rebuilds a file from the same blocks.
//
// It always runs a line diff. A file in a language it reads (lang.go) is then read as code:
// both versions are parsed, their tokens compared, and each block told apart into changes,
// formatting and renames. When reading fails or runs over budget, the line diff stands, and
// the result says why.
package semdiff

import (
	"context"
	"slices"
	"time"
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

// Engines say how a result compared the texts.
const (
	EngineLines = "lines"
	EngineCode  = "code"
)

// Options say how to compare.
type Options struct {
	// Algorithm is the line diff: one of Algorithms. "" means Myers, git's default.
	Algorithm string `json:"algorithm"`
	// Moves finds runs of lines deleted in one place and added unchanged in another.
	Moves bool `json:"moves"`
	// Language reads both versions as code in this language (LanguageOf names it). "" compares
	// lines only.
	Language string `json:"language"`
	// ReadTime is how long each version may take to parse: 0 means QuickRead, and less than
	// 0 means as long as it takes.
	ReadTime time.Duration `json:"readTime"`
}

// Result is a comparison of two texts.
type Result struct {
	// Blocks are the places that changed, in order. The lines between blocks are the same on
	// both sides.
	Blocks []Block `json:"blocks"`
	// Moves are runs of lines deleted in one place and added in another, ordered by where they
	// were added. Each one's lines lie inside blocks.
	Moves []Move `json:"moves"`
	// Renames are the names changed the same way in several places, when read as code.
	Renames []Rename `json:"renames"`
	// Algorithm is the line diff that ran. A long file compared by patience or histogram is
	// compared by Myers instead.
	Algorithm string `json:"algorithm"`
	// Engine is EngineCode when the versions were read as code, else EngineLines.
	Engine string `json:"engine"`
	// Language is the name of the language the file is in, when it's one read as code, even
	// if reading it failed.
	Language string `json:"language"`
	// Fallback says why a file in such a language got the line diff: one of the Fallback
	// constants, or "".
	Fallback string `json:"fallback"`
	// Unparsed is set when part of a version didn't parse. The rest was still read as code.
	Unparsed bool `json:"unparsed"`
	// Added and Removed count lines in blocks, moved ones included.
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
	// Kind is KindChange, or when read as code KindFormat or KindRename.
	Kind string `json:"kind,omitempty"`
	// Note says what a formatting or rename block changed, such as "same value".
	Note string `json:"note,omitempty"`
	// Old and New mark what changed inside the block's lines: words, or when read as code,
	// tokens. Neither is set when the lines were rewritten, or when either side is empty.
	Old []Span `json:"old,omitempty"`
	New []Span `json:"new,omitempty"`
}

// Span is changed text in one line. From and To count UTF-16 code units from the start of
// the line, which is how the views measure text.
type Span struct {
	Line int `json:"line"`
	From int `json:"from"`
	To   int `json:"to"`
	// Kind is KindChange, KindFormat (the same token written differently, or optional
	// punctuation) or KindRename.
	Kind string `json:"kind,omitempty"`
	// Rename is which of the Result's Renames a rename span is, counting from 1, so every
	// place one name changed can be shown together. 0 for other spans.
	Rename int `json:"rename,omitempty"`
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
	res := Result{Blocks: blocks, Moves: []Move{}, Renames: []Rename{}, Algorithm: algorithm, Engine: EngineLines}
	moved := movedLines{before: map[int]bool{}, after: map[int]bool{}}
	if opts.Moves {
		if res.Moves, err = findMoves(ctx, a, b, blocks); err != nil {
			return Result{}, err
		}
		moved = movedSet(res.Moves)
	}
	read := false
	if opts.Language != "" {
		if read, err = readAsCode(ctx, &res, a, b, opts, moved); err != nil {
			return Result{}, err
		}
	}
	if !read {
		for i := range res.Blocks {
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			markWords(a, b, &res.Blocks[i], moved)
		}
	}
	for _, blk := range res.Blocks {
		res.Removed += blk.OldEnd - blk.OldStart
		res.Added += blk.NewEnd - blk.NewStart
	}
	return res, nil
}

// readAsCode reads both versions in opts.Language and refines res's blocks from their tokens.
// It returns false, with res.Fallback set, when the line diff has to stand.
func readAsCode(ctx context.Context, res *Result, a, b text, opts Options, moved movedLines) (bool, error) {
	l := byID(opts.Language)
	if l == nil {
		return false, nil
	}
	res.Language = l.name
	if len(a.s) > maxCodeBytes || len(b.s) > maxCodeBytes {
		res.Fallback = FallbackSize
		return false, nil
	}
	timeout := opts.ReadTime
	if timeout == 0 {
		timeout = QuickRead
	}
	ra, rb, err := readCode(ctx, l.id, a.s, b.s, timeout)
	if err != nil {
		return false, err
	}
	for _, r := range []struct {
		reading
		size int
	}{{ra, len(a.s)}, {rb, len(b.s)}} {
		switch {
		case r.stop != "":
			res.Fallback = r.stop
		case r.size > 0 && float64(r.errBytes)/float64(r.size) > maxErrorShare:
			res.Fallback = FallbackErrors
		}
		if res.Fallback != "" {
			return false, nil
		}
	}
	blocks, renames, err := refine(ctx, newVersion(a, ra.toks, moved.before), newVersion(b, rb.toks, moved.after), res.Blocks)
	if err != nil {
		return false, err
	}
	res.Blocks, res.Renames = blocks, renames
	res.Engine = EngineCode
	res.Unparsed = ra.errBytes > 0 || rb.errBytes > 0 || ra.missing || rb.missing
	return true, nil
}

// maxAnchoredLines is the longest file patience and histogram compare. Both recurse once per
// anchor, so a huge file with few unique lines could take a long time; Myers bounds its own
// work.
const maxAnchoredLines = 100_000
