package semdiff

import (
	"context"
	"slices"
	"strings"
)

// What matching the trees adds to the token diff (#947): tokens that moved, and lines that
// moved into new structure.

// KindMove marks a token the token diff left changed on both sides that the trees match to
// the same token in another place: it moved, as arguments do when reordered.
const KindMove = "move"

// matchVersions matches the two versions' trees and records, on each, every token's partner
// and which tokens the token diff left changed. It does nothing for a version without a tree,
// or when matching runs out of budget.
func matchVersions(ctx context.Context, a, b *version, segs []*segment) {
	if a.nodes == nil || b.nodes == nil {
		return
	}
	a.changed, b.changed = make([]bool, len(a.toks)), make([]bool, len(b.toks))
	any := false
	for _, s := range segs {
		for k, t := range s.ot {
			if s.chO[k] {
				a.changed[t], any = true, true
			}
		}
		for k, t := range s.nt {
			if s.chN[k] {
				b.changed[t], any = true, true
			}
		}
	}
	if !any {
		return
	}
	ta, tb := buildTree(a, a.nodes), buildTree(b, b.nodes)
	if ta == nil || tb == nil {
		return
	}
	pa, pb, ok := matchTrees(ctx, ta, tb, len(a.toks), len(b.toks))
	if !ok {
		return
	}
	a.partner, b.partner, a.tree, b.tree = pa, pb, ta, tb
	b.leaf = filled32(len(b.toks))
	for i, g := range tb.n {
		if g.tok >= 0 {
			b.leaf[g.tok] = int32(i)
		}
	}
}

// movable reports whether a token can be said to move on its own: a name, a literal, a
// comment or a word, not punctuation.
func movable(c class) bool {
	return c == clsIdent || c == clsNumber || c == clsString || c == clsComment || c == clsText
}

// takeMoved marks the tokens of a run that moved within their segment, and returns the
// rest. near is the segment's tokens on the other side: a token matched further away is the
// same name or value used again, not moved. Punctuation that went along with a moved token,
// such as the comma between two reordered arguments, is the same token, so it's left
// unmarked.
func takeMoved(v, other *version, run, near []int, list *[]mark) []int {
	if v.partner == nil {
		return run
	}
	moved := func(t int) bool {
		p := v.partner[t]
		return p >= 0 && other.changed[p] && v.toks[t].key == other.toks[p].key && slices.Contains(near, int(p))
	}
	any := false
	for _, t := range run {
		if movable(v.toks[t].class) && moved(t) {
			any = true
		}
	}
	if !any {
		return run
	}
	var rest []int
	for _, t := range run {
		switch {
		case !moved(t):
			rest = append(rest, t)
		case movable(v.toks[t].class):
			*list = append(*list, mark{v.toks[t].from, v.toks[t].to, KindMove, 0})
		}
	}
	return rest
}

// joinNested joins moves into new structure that follow on from each other, as consecutive
// lines in separate segments do.
func joinNested(moves []Move) []Move {
	var out []Move
	for _, mv := range moves {
		if n := len(out); n > 0 && out[n-1].OldEnd == mv.OldStart && out[n-1].NewEnd == mv.NewStart && out[n-1].Into == mv.Into {
			out[n-1].OldEnd, out[n-1].NewEnd = mv.OldEnd, mv.NewEnd
			out[n-1].Reindented = out[n-1].Reindented || mv.Reindented
			continue
		}
		out = append(out, mv)
	}
	return out
}

// nestedMoves finds the lines of a segment whose tokens the token diff matched, but whose
// syntax changed around them, that the trees match too: code moved into new structure, such
// as statements wrapped in an if. Each line has to go to one line whole, and come from it
// whole. Each run of such lines is a move.
func nestedMoves(a, b *version, s *segment) []Move {
	if a.partner == nil {
		return nil
	}
	to, from := map[int]int{}, map[int]int{}
	bad, badNew, reshaped := map[int]bool{}, map[int]bool{}, map[int]bool{}
	s.walk(func(i, j int) {
		if a.toks[i].ignorable || a.toks[i].class == clsSpace {
			return
		}
		li, lj := a.first[i], b.first[j]
		if a.partner[i] != int32(j) || a.first[i] != a.last[i] || b.first[j] != b.last[j] {
			bad[li], badNew[lj] = true, true
			return
		}
		if prev, seen := to[li]; seen && prev != lj {
			bad[li] = true
		}
		if prev, seen := from[lj]; seen && prev != li {
			badNew[lj] = true
		}
		to[li], from[lj] = lj, li
		if a.toks[i].path != b.toks[j].path {
			reshaped[li] = true
		}
	}, func(c change) {
		for _, t := range c.old {
			bad[a.first[t]] = true
		}
		for _, t := range c.new {
			badNew[b.first[t]] = true
		}
	})
	var out []Move
	for li := s.o0; li < s.o1; li++ {
		lj, ok := to[li]
		if !ok || bad[li] || badNew[lj] || !reshaped[li] {
			continue
		}
		if n := len(out); n > 0 && out[n-1].OldEnd == li && out[n-1].NewEnd == lj {
			out[n-1].OldEnd, out[n-1].NewEnd = li+1, lj+1
		} else {
			out = append(out, Move{OldStart: li, OldEnd: li + 1, NewStart: lj, NewEnd: lj + 1})
		}
	}
	for k := range out {
		mv := &out[k]
		for n := 0; n < mv.OldEnd-mv.OldStart; n++ {
			if a.t.body(mv.OldStart+n) != b.t.body(mv.NewStart+n) {
				mv.Reindented = true
			}
		}
		mv.Into = structureAround(b, mv.NewStart, mv.NewEnd)
	}
	return out
}

// structureAround names the structure that lines [from, to) on the new side now sit in: the
// nearest container reaching past them, passing over plain blocks and lists, such as "if
// statement". It's "" when there's none to name.
func structureAround(b *version, from, to int) string {
	for t := range b.toks {
		if b.first[t] != from || b.leaf[t] < 0 {
			continue
		}
		for p := b.tree.n[b.leaf[t]].parent; p > 0; p = b.tree.n[p].parent {
			g := b.tree.n[p]
			if g.typ == "" || generic(g.typ) || !reaches(b, p, from, to) {
				continue
			}
			return strings.ReplaceAll(g.typ, "_", " ")
		}
		return ""
	}
	return ""
}

// reaches reports whether node p of the new side's tree holds tokens outside lines [from, to).
func reaches(b *version, p int32, from, to int) bool {
	g := b.tree.n[p]
	for q := g.pre; q < g.pre+g.size; q++ {
		if n := b.tree.n[b.tree.pre[q]]; n.tok >= 0 && (b.first[n.tok] < from || b.first[n.tok] >= to) {
			return true
		}
	}
	return false
}

// generic reports whether a node type only holds other code: a block, a list, a body.
func generic(typ string) bool {
	for _, w := range []string{"block", "list", "body", "suite", "compound", "program", "source_file", "module", "chunk"} {
		if strings.Contains(typ, w) {
			return true
		}
	}
	return false
}
