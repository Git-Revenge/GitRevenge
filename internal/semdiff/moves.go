package semdiff

import (
	"context"
	"slices"
	"strings"
	"unicode"
)

// minMoveWeight is how many letters and digits a run of lines needs to count as moved. A
// closing brace, a blank line or a lone `return nil` deleted in one place and added in another
// is a coincidence, not a move; a function or an import path is a move.
const minMoveWeight = 20

// maxMoveCandidates bounds how many places a line is looked for, so a file full of the same
// line can't make finding moves quadratic.
const maxMoveCandidates = 64

// findMoves pairs runs of deleted lines with runs of added lines that read the same apart from
// indentation, longest first from each deleted line.
func findMoves(ctx context.Context, a, b text, blocks []Block) ([]Move, error) {
	deleted := make([]bool, a.n())
	added := make([]bool, b.n())
	addedIn := make([]int, b.n()) // the block each added line is in
	addedAt := map[string][]int{}
	for bi, blk := range blocks {
		for i := blk.OldStart; i < blk.OldEnd; i++ {
			deleted[i] = true
		}
		for j := blk.NewStart; j < blk.NewEnd; j++ {
			added[j] = true
			addedIn[j] = bi
			if k := key(b, j); k != "" {
				addedAt[k] = append(addedAt[k], j)
			}
		}
	}
	var moves []Move
	for bi, blk := range blocks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for i := blk.OldStart; i < blk.OldEnd; {
			k := key(a, i)
			if !deleted[i] || k == "" {
				i++
				continue
			}
			bestJ, bestLen := -1, 0
			candidates := addedAt[k]
			for _, j := range candidates[:min(len(candidates), maxMoveCandidates)] {
				// Lines taken out and put back where they were were edited, or indented,
				// not moved.
				if addedIn[j] == bi && i-blk.OldStart == j-blk.NewStart {
					continue
				}
				n := 0
				for i+n < a.n() && j+n < b.n() && deleted[i+n] && added[j+n] && key(a, i+n) == key(b, j+n) {
					n++
				}
				if n > bestLen {
					bestJ, bestLen = j, n
				}
			}
			// A run ends on a line with something on it.
			for bestLen > 0 && key(a, i+bestLen-1) == "" {
				bestLen--
			}
			if bestLen == 0 || weight(a, i, i+bestLen) < minMoveWeight {
				i++
				continue
			}
			mv := Move{OldStart: i, OldEnd: i + bestLen, NewStart: bestJ, NewEnd: bestJ + bestLen}
			for n := range bestLen {
				deleted[i+n], added[bestJ+n] = false, false
				if a.body(i+n) != b.body(bestJ+n) {
					mv.Reindented = true
				}
			}
			moves = append(moves, mv)
			i += bestLen
		}
	}
	slices.SortFunc(moves, func(x, y Move) int { return x.NewStart - y.NewStart })
	if moves == nil {
		moves = []Move{}
	}
	return moves, nil
}

// key is line i without its indentation and ending: what a moved line has to keep.
func key(t text, i int) string { return strings.TrimSpace(t.line(i)) }

// weight counts the letters and digits in lines [from, to).
func weight(t text, from, to int) int {
	n := 0
	for _, r := range t.span(from, to) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}

// movedLines says which lines on each side belong to a move.
type movedLines struct {
	before, after map[int]bool
}

func movedSet(moves []Move) movedLines {
	m := movedLines{before: map[int]bool{}, after: map[int]bool{}}
	for _, mv := range moves {
		for i := mv.OldStart; i < mv.OldEnd; i++ {
			m.before[i] = true
		}
		for j := mv.NewStart; j < mv.NewEnd; j++ {
			m.after[j] = true
		}
	}
	return m
}
