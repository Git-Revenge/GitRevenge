package semdiff

import (
	"context"
	"slices"
	"time"

	"znkr.io/diff"
)

// Moves that changed on the way (#947). After the exact moves are taken out, a run of deleted
// lines in one block that shares most of its lines with a run of added lines in another block
// is code moved and edited. The move carries its own small diff, so the views can open it
// against where it came from and show the edits as edits.

const (
	// minChangedLines is how many lines with something on them a run needs on each side.
	minChangedLines = 3
	// minChangedAlike is the share of the two runs' lines that have to come through as they
	// were: twice the lines kept over the lines on both sides.
	minChangedAlike = 0.5
	// maxChangedRuns bounds the runs compared on each side, so the pairing stays cheap.
	maxChangedRuns = 200
)

// lineRun is a run of lines [from, to) that are all deleted, or all added, in one block.
type lineRun struct{ block, from, to int }

// runsOf lists the runs of flagged lines in each block, without blank lines at their ends.
func runsOf(t text, blocks []Block, flagged []bool, old bool) []lineRun {
	var out []lineRun
	for bi, blk := range blocks {
		start, end := blk.NewStart, blk.NewEnd
		if old {
			start, end = blk.OldStart, blk.OldEnd
		}
		for i := start; i < end; {
			if !flagged[i] {
				i++
				continue
			}
			j := i
			for j < end && flagged[j] {
				j++
			}
			from, to := i, j
			for from < to && key(t, from) == "" {
				from++
			}
			for to > from && key(t, to-1) == "" {
				to--
			}
			if from < to {
				out = append(out, lineRun{bi, from, to})
			}
			i = j
		}
	}
	return out
}

// filled counts a run's lines with something on them.
func filled(t text, r lineRun) int {
	n := 0
	for i := r.from; i < r.to; i++ {
		if key(t, i) != "" {
			n++
		}
	}
	return n
}

// changedMoves pairs the runs of lines exact moves left deleted with runs left added in
// another block, keeping the pair whose lines come through best.
func changedMoves(ctx context.Context, a, b text, blocks []Block, deleted, added []bool) ([]Move, error) {
	olds, news := runsOf(a, blocks, deleted, true), runsOf(b, blocks, added, false)
	if len(olds) > maxChangedRuns || len(news) > maxChangedRuns {
		return nil, nil
	}
	keys := func(t text, r lineRun) []string {
		out := make([]string, 0, r.to-r.from)
		for i := r.from; i < r.to; i++ {
			out = append(out, key(t, i))
		}
		return out
	}
	used := make([]bool, len(news))
	var moves []Move
	for _, ro := range olds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		no := filled(a, ro)
		if no < minChangedLines || weight(a, ro.from, ro.to) < 2*minMoveWeight {
			continue
		}
		ko := keys(a, ro)
		best, bestKept, bestAlike := -1, 0, 0.0
		for k, rn := range news {
			// Lines changed where they were are an edit, not a move.
			if used[k] || rn.block == ro.block {
				continue
			}
			nn := filled(b, rn)
			if nn < minChangedLines {
				continue
			}
			kept := 0
			for _, e := range diff.Edits(ko, keys(b, rn)) {
				if e.Op == diff.Match && ko[e.PosX] != "" {
					kept++
				}
			}
			alike := 2 * float64(kept) / float64(no+nn)
			if kept >= 2 && alike >= minChangedAlike && (kept > bestKept || kept == bestKept && alike > bestAlike) {
				best, bestKept, bestAlike = k, kept, alike
			}
		}
		if best < 0 {
			continue
		}
		used[best] = true
		rn := news[best]
		moves = append(moves, Move{OldStart: ro.from, OldEnd: ro.to, NewStart: rn.from, NewEnd: rn.to, Changed: true})
	}
	return moves, nil
}

// compareMoves gives each changed move its own diff, its blocks counted from the move's first
// line on each side, read as code when the file is.
func compareMoves(ctx context.Context, a, b text, res *Result, opts Options) error {
	limit := moveReadTime(opts.ReadTime)
	for k := range res.Moves {
		mv := &res.Moves[k]
		if !mv.Changed {
			continue
		}
		sub, err := Diff(ctx, a.span(mv.OldStart, mv.OldEnd), b.span(mv.NewStart, mv.NewEnd), Options{Algorithm: opts.Algorithm, Language: opts.Language, ReadTime: limit})
		if err != nil {
			return err
		}
		mv.Edits = sub.Blocks
	}
	return nil
}

// moveReadTime is how long a move's own diff may take to read each version, given the
// caller's ReadTime: QuickRead, as a file can have many moves, unless the caller asked for
// no limit.
func moveReadTime(caller time.Duration) time.Duration {
	if caller < 0 {
		return caller
	}
	return QuickRead
}

// maxJoinGap is how many lines, on each side, two moves can be apart and still be one move
// that changed: an edited line or two inside moved code splits it into two exact moves.
const maxJoinGap = 3

// joinMoves joins moves that follow each other on both sides with a few unmatched lines
// between them into one move that changed on the way.
func joinMoves(moves []Move, deleted, added []bool) []Move {
	if len(moves) < 2 {
		return moves
	}
	slices.SortFunc(moves, func(x, y Move) int { return x.OldStart - y.OldStart })
	out := []Move{moves[0]}
	for _, mv := range moves[1:] {
		last := &out[len(out)-1]
		gapOld, gapNew := mv.OldStart-last.OldEnd, mv.NewStart-last.NewEnd
		if gapOld < 0 || gapNew < 0 || gapOld > maxJoinGap || gapNew > maxJoinGap || gapOld+gapNew == 0 {
			out = append(out, mv)
			continue
		}
		free := true
		for i := last.OldEnd; i < mv.OldStart; i++ {
			free = free && deleted[i]
		}
		for j := last.NewEnd; j < mv.NewStart; j++ {
			free = free && added[j]
		}
		if !free {
			out = append(out, mv)
			continue
		}
		for i := last.OldEnd; i < mv.OldStart; i++ {
			deleted[i] = false
		}
		for j := last.NewEnd; j < mv.NewStart; j++ {
			added[j] = false
		}
		last.OldEnd, last.NewEnd = mv.OldEnd, mv.NewEnd
		last.Reindented = last.Reindented || mv.Reindented
		last.Changed = true
	}
	return out
}
