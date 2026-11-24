package semdiff

import (
	"context"
	"slices"
	"sort"
	"strings"

	"znkr.io/diff"
)

// Block kinds, in code mode.
const (
	// KindChange is a change to the code. It's the kind of every block in line mode.
	KindChange = ""
	// KindFormat changes only how the code is written: its whitespace and line breaks, how a
	// literal is written, or optional punctuation. Its tokens mean the same on both sides.
	KindFormat = "format"
	// KindRename changes only names, each the same way wherever it changed.
	KindRename = "rename"
)

// Rename is a name changed the same way in more than one place.
type Rename struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"` // the places it changed
}

// maxBlockTokens bounds the tokens a block is compared by. A bigger block is marked by
// words, as the line diff marks it.
const maxBlockTokens = 40_000

// minRenamePlaces is how many places a name has to change in, the same way, to count as a
// rename rather than a change.
const minRenamePlaces = 2

// version is one side, read as code: its text, its tokens, and the lines each token is on.
type version struct {
	t     text
	toks  []tok
	first []int // the line each token starts on
	last  []int // the line each token ends on
	moved map[int]bool
}

func newVersion(t text, toks []tok, moved map[int]bool) *version {
	v := &version{t: t, toks: toks, first: make([]int, len(toks)), last: make([]int, len(toks)), moved: moved}
	for i, k := range toks {
		v.first[i], _ = lineCol(t, k.from)
		v.last[i], _ = lineCol(t, max(k.from, k.to-1))
		// Whitespace in markup counts only by its presence, so it belongs to the line it
		// starts on, and a line break inside it doesn't hold two lines together.
		if k.class == clsSpace {
			v.last[i] = v.first[i]
		}
	}
	return v
}

// on returns the tokens on lines [from, to), leaving out tokens wholly on moved lines.
func (v *version) on(from, to int) []int {
	i := sort.Search(len(v.toks), func(i int) bool { return v.last[i] >= from })
	var out []int
	for ; i < len(v.toks) && v.first[i] < to; i++ {
		if v.moved[v.first[i]] && v.first[i] == v.last[i] {
			continue
		}
		out = append(out, i)
	}
	return out
}

func (v *version) text(i int) string { return v.t.s[v.toks[i].from:v.toks[i].to] }

// segment is part of a block, compared token by token: old lines [o0, o1) became new lines
// [n0, n1). ot and nt are its tokens and chO and chN say which of them changed.
type segment struct {
	block          int
	o0, o1, n0, n1 int
	ot, nt         []int
	chO, chN       []bool
	whole          bool // too big to compare by tokens; marked by words instead
	kind           string
	note           string
	old, new       []Span
}

// refine reads the line diff's blocks as code: each block is compared token by token and
// split where its parts differ in kind, so formatting and renames stand apart from changes.
// Lines left identical by the split go back to being unchanged.
func refine(ctx context.Context, a, b *version, blocks []Block) ([]Block, []Rename, error) {
	var segs []*segment
	for k, blk := range blocks {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		segs = append(segs, splitBlock(a, b, k, blk)...)
	}
	renames, confirmed := findRenames(a, b, segs)
	for _, s := range segs {
		classify(a, b, s, confirmed)
	}
	out := make([]Block, 0, len(segs))
	for i := 0; i < len(segs); i++ {
		s := segs[i]
		blk := Block{OldStart: s.o0, OldEnd: s.o1, NewStart: s.n0, NewEnd: s.n1, Kind: s.kind, Note: s.note, Old: s.old, New: s.new}
		// Neighbours of one kind from one block stay one block, as the line diff had them.
		for i+1 < len(segs) {
			n := segs[i+1]
			if n.block != s.block || n.kind != s.kind || n.o0 != blk.OldEnd || n.n0 != blk.NewEnd {
				break
			}
			blk.OldEnd, blk.NewEnd = n.o1, n.n1
			blk.Old = append(blk.Old, n.old...)
			blk.New = append(blk.New, n.new...)
			if blk.Note != n.note {
				blk.Note = mergeNotes(blk.Note, n.note)
			}
			i++
		}
		if blk.Kind == KindChange {
			blk.Note = ""
		}
		out = append(out, blk)
	}
	return out, renames, nil
}

// splitBlock compares a block's tokens and cuts it into segments at lines where the comparison
// lines up on both sides. A block with moved lines stays whole.
func splitBlock(a, b *version, k int, blk Block) []*segment {
	ot := a.on(blk.OldStart, blk.OldEnd)
	nt := b.on(blk.NewStart, blk.NewEnd)
	whole := &segment{block: k, o0: blk.OldStart, o1: blk.OldEnd, n0: blk.NewStart, n1: blk.NewEnd, ot: ot, nt: nt}
	if len(ot)+len(nt) > maxBlockTokens {
		whole.whole = true
		return []*segment{whole}
	}
	whole.chO, whole.chN = compare(a, b, ot, nt)

	// Cut wherever the comparison is between lines on both sides: every token before the cut
	// is matched or changed against tokens before it on the other side, and no line holds
	// tokens from both sides of it. Blank lines go with what follows them.
	type cut struct{ x, y, i, j int }
	var cuts []cut
	at := func(v *version, idx []int, i, start, end int) (int, bool) {
		switch {
		case i == 0:
			return start, true
		case i == len(idx):
			return min(max(v.last[idx[i-1]]+1, start), end), true
		}
		x := v.last[idx[i-1]] + 1
		return x, v.first[idx[i]] >= x && x >= start && x <= end
	}
	try := func(i, j int) {
		if (i == 0 && j == 0) || (i == len(ot) && j == len(nt)) {
			return
		}
		x, okx := at(a, ot, i, blk.OldStart, blk.OldEnd)
		y, oky := at(b, nt, j, blk.NewStart, blk.NewEnd)
		if !okx || !oky {
			return
		}
		if n := len(cuts); n > 0 && cuts[n-1].x == x && cuts[n-1].y == y {
			return
		}
		if (x == blk.OldStart && y == blk.NewStart) || (x == blk.OldEnd && y == blk.NewEnd) {
			return
		}
		cuts = append(cuts, cut{x, y, i, j})
	}
	// Never inside a run of changes: a replacement cut between its old and new tokens would
	// read as a removal and an addition.
	i, j := 0, 0
	for i < len(ot) || j < len(nt) {
		try(i, j)
		run := false
		for i < len(ot) && whole.chO[i] {
			i++
			run = true
		}
		for j < len(nt) && whole.chN[j] {
			j++
			run = true
		}
		if !run {
			i++
			j++
		}
	}
	var segs []*segment
	prev := cut{blk.OldStart, blk.NewStart, 0, 0}
	for _, c := range append(cuts, cut{blk.OldEnd, blk.NewEnd, len(ot), len(nt)}) {
		s := &segment{block: k, o0: prev.x, o1: c.x, n0: prev.y, n1: c.y,
			ot: ot[prev.i:c.i], nt: nt[prev.j:c.j], chO: whole.chO[prev.i:c.i], chN: whole.chN[prev.j:c.j]}
		prev = c
		if trim(a, b, s) {
			segs = append(segs, s)
		}
	}
	return segs
}

// trim drops a segment's first and last lines while they're the same on both sides, and the
// tokens on them. Moved lines stay, since their moves point at them. It returns false when
// nothing is left.
func trim(a, b *version, s *segment) bool {
	same := func(i, j int) bool { return !a.moved[i] && !b.moved[j] && a.t.line(i) == b.t.line(j) }
	for s.o0 < s.o1 && s.n0 < s.n1 && same(s.o0, s.n0) {
		s.o0++
		s.n0++
	}
	for s.o0 < s.o1 && s.n0 < s.n1 && same(s.o1-1, s.n1-1) {
		s.o1--
		s.n1--
	}
	if s.o0 == s.o1 && s.n0 == s.n1 {
		return false
	}
	keep := func(v *version, idx []int, ch []bool, from, to int) ([]int, []bool) {
		var oi []int
		var oc []bool
		for k, t := range idx {
			if v.last[t] >= from && v.first[t] < to {
				oi = append(oi, t)
				oc = append(oc, ch[k])
			}
		}
		return oi, oc
	}
	s.ot, s.chO = keep(a, s.ot, s.chO, s.o0, s.o1)
	s.nt, s.chN = keep(b, s.nt, s.chN, s.n0, s.n1)
	return true
}

// compare diffs two runs of tokens by key and says which of each changed, with each run of
// changes slid to where it starts and ends with syntax.
func compare(a, b *version, ot, nt []int) (chO, chN []bool) {
	ids := map[string]int{}
	intern := func(v *version, idx []int) []int {
		out := make([]int, len(idx))
		for k, t := range idx {
			key := v.toks[t].key
			id, ok := ids[key]
			if !ok {
				id = len(ids)
				ids[key] = id
			}
			out[k] = id
		}
		return out
	}
	ko, kn := intern(a, ot), intern(b, nt)
	chO, chN = make([]bool, len(ot)), make([]bool, len(nt))
	// The tokens that mean something first, so optional punctuation can't pull them out of
	// line: in if ((x)), the if's own parenthesis matches the if's.
	meaning := func(v *version, idx []int, opt bool) []int {
		var out []int
		for p, t := range idx {
			if v.toks[t].ignorable == opt {
				out = append(out, p)
			}
		}
		return out
	}
	keys := func(k, pos []int) []int {
		out := make([]int, len(pos))
		for x, p := range pos {
			out[x] = k[p]
		}
		return out
	}
	mo, mn := meaning(a, ot, false), meaning(b, nt, false)
	anchors := [][2]int{}
	for _, e := range diff.Edits(keys(ko, mo), keys(kn, mn)) {
		switch e.Op {
		case diff.Delete:
			chO[mo[e.PosX]] = true
		case diff.Insert:
			chN[mn[e.PosY]] = true
		default:
			anchors = append(anchors, [2]int{mo[e.PosX], mn[e.PosY]})
		}
	}
	// Then the optional punctuation between each pair of matched tokens, against the other
	// side's.
	oo, on := meaning(a, ot, true), meaning(b, nt, true)
	xo, xn := 0, 0 // the next optional token on each side
	for _, an := range append(anchors, [2]int{len(ot), len(nt)}) {
		var io, in []int
		for ; xo < len(oo) && oo[xo] < an[0]; xo++ {
			io = append(io, oo[xo])
		}
		for ; xn < len(on) && on[xn] < an[1]; xn++ {
			in = append(in, on[xn])
		}
		if len(io)+len(in) > 0 {
			for _, e := range diff.Edits(keys(ko, io), keys(kn, in)) {
				switch e.Op {
				case diff.Delete:
					chO[io[e.PosX]] = true
				case diff.Insert:
					chN[in[e.PosY]] = true
				}
			}
		}
	}
	slide(a, ot, ko, chO)
	slide(b, nt, kn, chN)
	return chO, chN
}

// slide moves each run of changed tokens, where the tokens around it allow, to where it
// starts at the start of a syntax node and ends at the end of one: an inserted argument is
// ", c" rather than "b, " when either reads the same.
func slide(v *version, idx, keys []int, ch []bool) {
	score := func(s, e int) int {
		n := 0
		if v.toks[idx[s]].open > 0 {
			n++
		}
		if v.toks[idx[e-1]].close > 0 {
			n++
		}
		return n
	}
	for s := 0; s < len(ch); {
		if !ch[s] {
			s++
			continue
		}
		e := s
		for e < len(ch) && ch[e] {
			e++
		}
		// The highest place the run can go, then each place down from it.
		top, bottom := s, e
		// A run slides over tokens of its own sort only: optional punctuation stays optional.
		same := func(x, y int) bool { return keys[x] == keys[y] && v.toks[idx[x]].ignorable == v.toks[idx[y]].ignorable }
		for top > 0 && !ch[top-1] && same(top-1, bottom-1) {
			top--
			bottom--
		}
		best, bestScore := top, score(top, bottom)
		for p, q := top, bottom; q < len(ch) && !ch[q] && same(p, q); {
			p++
			q++
			if sc := score(p, q); sc > bestScore || (sc == bestScore && p <= s) {
				best, bestScore = p, sc
			}
		}
		n := e - s
		for k := s; k < e; k++ {
			ch[k] = false
		}
		for k := best; k < best+n; k++ {
			ch[k] = true
		}
		s = max(e, best+n)
	}
}

// change is a run of changed tokens: the old ones replaced by the new ones.
type change struct {
	old, new []int
}

// walk goes through a segment's tokens, calling same for each pair of unchanged tokens and changed
// for each run of changes.
func (s *segment) walk(same func(i, j int), changed func(c change)) {
	i, j := 0, 0
	for i < len(s.ot) || j < len(s.nt) {
		var c change
		for i < len(s.ot) && s.chO[i] {
			c.old = append(c.old, s.ot[i])
			i++
		}
		for j < len(s.nt) && s.chN[j] {
			c.new = append(c.new, s.nt[j])
			j++
		}
		if len(c.old) > 0 || len(c.new) > 0 {
			changed(c)
			continue
		}
		if i < len(s.ot) && j < len(s.nt) {
			same(s.ot[i], s.nt[j])
		}
		i++
		j++
	}
}

// meaningful is the tokens of a run that aren't optional punctuation.
func meaningful(v *version, idx []int) []int {
	var out []int
	for _, t := range idx {
		if !v.toks[t].ignorable {
			out = append(out, t)
		}
	}
	return out
}

// substitution says whether a run swaps one name for another, and which.
func substitution(a, b *version, c change) (from, to string, ok bool) {
	o, n := meaningful(a, c.old), meaningful(b, c.new)
	if len(o) != 1 || len(n) != 1 || a.toks[o[0]].class != clsIdent || b.toks[n[0]].class != clsIdent {
		return "", "", false
	}
	return a.text(o[0]), b.text(n[0]), true
}

// findRenames finds names changed the same way in at least minRenamePlaces places, with no
// other change to either name, where the old name is gone and the new one is new.
func findRenames(a, b *version, segs []*segment) ([]Rename, map[[2]string]int) {
	count := map[[2]string]int{}
	from := map[string]map[string]bool{}
	to := map[string]map[string]bool{}
	var order [][2]string
	for _, s := range segs {
		if s.whole {
			continue
		}
		s.walk(func(int, int) {}, func(c change) {
			f, t, ok := substitution(a, b, c)
			if !ok {
				return
			}
			k := [2]string{f, t}
			if count[k] == 0 {
				order = append(order, k)
			}
			count[k]++
			if from[f] == nil {
				from[f] = map[string]bool{}
			}
			if to[t] == nil {
				to[t] = map[string]bool{}
			}
			from[f][t] = true
			to[t][f] = true
		})
	}
	// A rename leaves no trace of the old name, and the new one wasn't there before: a type
	// swapped for another in two places, while the first is still used elsewhere, is a change.
	names := func(v *version) map[string]bool {
		out := map[string]bool{}
		for i, t := range v.toks {
			if t.class == clsIdent {
				out[v.text(i)] = true
			}
		}
		return out
	}
	oldNames, newNames := names(a), names(b)
	renames := []Rename{}
	confirmed := map[[2]string]int{} // each rename's place in renames, from 1
	for _, k := range order {
		if count[k] >= minRenamePlaces && len(from[k[0]]) == 1 && len(to[k[1]]) == 1 && !newNames[k[0]] && !oldNames[k[1]] {
			renames = append(renames, Rename{From: k[0], To: k[1], Count: count[k]})
			confirmed[k] = len(renames)
		}
	}
	return renames, confirmed
}

// classify says what a segment is and marks what changed in it.
func classify(a, b *version, s *segment, renames map[[2]string]int) {
	if s.whole {
		blk := Block{OldStart: s.o0, OldEnd: s.o1, NewStart: s.n0, NewEnd: s.n1}
		markWords(a.t, b.t, &blk, movedLines{before: a.moved, after: b.moved})
		s.old, s.new = blk.Old, blk.New
		return
	}
	var marks marker
	total := 0
	for _, t := range s.ot {
		total += len(a.text(t))
	}
	for _, t := range s.nt {
		total += len(b.text(t))
	}
	kept := total
	changes, renamed := 0, 0
	var renamedFrom string
	values, punct := false, map[string]bool{}
	s.walk(func(i, j int) {
		ta, tb := a.toks[i], b.toks[j]
		if ta.ignorable || ta.class == clsSpace || a.text(i) == b.text(j) {
			return
		}
		// The same token written differently: 0x30 and 48.
		marks.add(&marks.old, ta.from, ta.to, KindFormat)
		marks.add(&marks.new, tb.from, tb.to, KindFormat)
		if ta.class == clsNumber || ta.class == clsString {
			values = true
		}
	}, func(c change) {
		o, n := meaningful(a, c.old), meaningful(b, c.new)
		if len(o) == 0 && len(n) == 0 {
			// Only optional punctuation came or went: formatting.
			for _, t := range c.old {
				marks.add(&marks.old, a.toks[t].from, a.toks[t].to, KindFormat)
				punct[a.toks[t].key] = true
			}
			for _, t := range c.new {
				marks.add(&marks.new, b.toks[t].from, b.toks[t].to, KindFormat)
				punct[b.toks[t].key] = true
			}
			return
		}
		if f, t, ok := substitution(a, b, c); ok && renames[[2]string{f, t}] > 0 {
			renamed++
			if renamedFrom == "" {
				renamedFrom = f
			}
			g := renames[[2]string{f, t}]
			marks.old = append(marks.old, mark{a.toks[o[0]].from, a.toks[o[0]].to, KindRename, g})
			marks.new = append(marks.new, mark{b.toks[n[0]].from, b.toks[n[0]].to, KindRename, g})
			return
		}
		changes++
		// Punctuation inside a change is part of it.
		o, n = c.old, c.new
		for _, t := range o {
			kept -= len(a.text(t))
		}
		for _, t := range n {
			kept -= len(b.text(t))
		}
		if len(o) == 1 && len(n) == 1 && a.toks[o[0]].class == b.toks[n[0]].class && wordy(a.toks[o[0]].class) {
			ta, tb := a.toks[o[0]], b.toks[n[0]]
			x, y := a.t.s[ta.from:ta.to], b.t.s[tb.from:tb.to]
			// A string's words are compared inside its quotes; quotes changed apart from
			// them are formatting.
			xo, xc, yo, yc := 0, len(x), 0, len(y)
			if ta.class == clsString {
				xo, xc = quotes(x)
				yo, yc = quotes(y)
			}
			if wa, wb, ok := wordChanges(x[xo:xc], y[yo:yc]); ok {
				for _, w := range wa {
					marks.add(&marks.old, ta.from+xo+w[0], ta.from+xo+w[1], KindChange)
				}
				for _, w := range wb {
					marks.add(&marks.new, tb.from+yo+w[0], tb.from+yo+w[1], KindChange)
				}
				if x[:xo] != y[:yo] {
					marks.add(&marks.old, ta.from, ta.from+xo, KindFormat)
					marks.add(&marks.new, tb.from, tb.from+yo, KindFormat)
				}
				if x[xc:] != y[yc:] {
					marks.add(&marks.old, ta.from+xc, ta.to, KindFormat)
					marks.add(&marks.new, tb.from+yc, tb.to, KindFormat)
				}
				return
			}
		}
		for _, t := range o {
			marks.add(&marks.old, a.toks[t].from, a.toks[t].to, KindChange)
		}
		for _, t := range n {
			marks.add(&marks.new, b.toks[t].from, b.toks[t].to, KindChange)
		}
	})
	// What the segment is follows from its tokens, not from how the diff lined them up: equal
	// tokens in the same syntax are formatting, whatever a tie in the diff marked.
	switch {
	case s.moves(a, b) || s.reshaped(a, b, renames):
		s.kind = KindChange
	case renamed > 0:
		s.kind, s.note = KindRename, "renamed from "+renamedFrom
	default:
		s.kind, s.note = KindFormat, formatNote(a, b, s, values, punct)
	}
	if s.kind != KindChange && changes > 0 {
		for _, list := range []*[]mark{&marks.old, &marks.new} {
			for k := range *list {
				if (*list)[k].kind == KindChange {
					(*list)[k].kind = KindFormat
				}
			}
		}
	}
	// Lines only added or removed, or rewritten, are marked by their colour alone; marking
	// every token of them says nothing more.
	if s.o0 == s.o1 || s.n0 == s.n1 || total == 0 || float64(kept)/float64(total) < minAlike {
		marks.old = slices.DeleteFunc(marks.old, func(m mark) bool { return m.kind == KindChange })
		marks.new = slices.DeleteFunc(marks.new, func(m mark) bool { return m.kind == KindChange })
	}
	s.old = marks.spans(a.t, s.o0, s.o1, marks.old)
	s.new = marks.spans(b.t, s.n0, s.n1, marks.new)
}

// reshaped says whether a segment's unchanged tokens sit in different syntax on the two
// sides, as when Python code is indented into a block, or a CSS selector loses the space
// that made it a descendant. Tokens read from text the tree left out, or inside a parse
// error, count only as written. A name a rename changed counts as the same token.
func (s *segment) reshaped(a, b *version, renames map[[2]string]int) bool {
	o, n := meaningful(a, s.ot), meaningful(b, s.nt)
	if len(o) != len(n) {
		return true
	}
	for k := range o {
		ta, tb := a.toks[o[k]], b.toks[n[k]]
		if ta.path != tb.path {
			return true
		}
		if ta.key != tb.key && (ta.class != clsIdent || renames[[2]string{a.text(o[k]), b.text(n[k])}] == 0) {
			return true
		}
		if (ta.inError || tb.inError) && a.text(o[k]) != b.text(n[k]) {
			return true
		}
	}
	return false
}

// moves says whether any of a segment's lines moved. Such a segment is a change: its moves
// show what happened to those lines.
func (s *segment) moves(a, b *version) bool {
	for i := s.o0; i < s.o1; i++ {
		if a.moved[i] {
			return true
		}
	}
	for j := s.n0; j < s.n1; j++ {
		if b.moved[j] {
			return true
		}
	}
	return false
}

// formatNote says what kind of formatting a segment changed.
func formatNote(a, b *version, s *segment, values bool, punct map[string]bool) string {
	switch {
	case values:
		return "same value"
	case punct[","]:
		return "trailing comma"
	case punct["("] || punct[")"]:
		return "parentheses"
	case punct[";"]:
		return "semicolons"
	case len(s.ot) == 0 && len(s.nt) == 0:
		return "blank lines"
	}
	ol, nl := a.t.span(s.o0, s.o1), b.t.span(s.n0, s.n1)
	if strings.ReplaceAll(ol, "\r\n", "\n") == strings.ReplaceAll(nl, "\r\n", "\n") {
		return "line endings"
	}
	if s.o1-s.o0 != s.n1-s.n0 {
		return "line breaks"
	}
	o, n := meaningful(a, s.ot), meaningful(b, s.nt)
	for k := range min(len(o), len(n)) {
		if a.toks[o[k]].class != clsSpace && a.text(o[k]) != b.text(n[k]) {
			return "same text"
		}
	}
	return "spacing"
}

// mergeNotes joins the notes of two formatting segments that became one block.
func mergeNotes(x, y string) string {
	if x == "" {
		return y
	}
	if y == "" || x == y {
		return x
	}
	return "formatting"
}

func wordy(c class) bool { return c == clsString || c == clsComment || c == clsText }

// quotes finds a string literal's text: after its prefix and opening quotes, and before its
// closing quotes and suffix, as byte offsets. It returns the whole literal when it can't tell.
func quotes(s string) (start, end int) {
	i := 0
	for i < len(s) && (isWordByte(s[i]) && s[i] < 0x80 || s[i] == '@' || s[i] == '$' || s[i] == '#') {
		i++
	}
	if i >= len(s) || (s[i] != '"' && s[i] != '\'' && s[i] != '`') {
		return 0, len(s)
	}
	q := s[i]
	j := i
	for j < len(s) && j-i < 3 && s[j] == q {
		j++
	}
	k := len(s)
	for k > j && (isWordByte(s[k-1]) && s[k-1] < 0x80 || s[k-1] == '#') {
		k--
	}
	for n := 0; n < j-i && k > j && s[k-1] == q; n++ {
		k--
	}
	if k < j {
		return 0, len(s)
	}
	return j, k
}

// wordChanges marks the words that changed between two versions of a string, comment or text
// token, as byte ranges in each, when enough of it stayed the same to make that worth it.
func wordChanges(x, y string) (wx, wy [][2]int, ok bool) {
	tx, ty := words(x), words(y)
	sx := make([]string, len(tx))
	for i, t := range tx {
		sx[i] = x[t[0]:t[1]]
	}
	sy := make([]string, len(ty))
	for i, t := range ty {
		sy[i] = y[t[0]:t[1]]
	}
	kept, total := 0, 0
	var cx, cy []bool
	cx, cy = make([]bool, len(tx)), make([]bool, len(ty))
	for _, e := range diff.Edits(sx, sy) {
		switch e.Op {
		case diff.Match:
			if strings.TrimSpace(e.X) != "" {
				kept += 2 * len(e.X)
			}
		case diff.Delete:
			cx[e.PosX] = true
		case diff.Insert:
			cy[e.PosY] = true
		}
	}
	for _, s := range sx {
		if strings.TrimSpace(s) != "" {
			total += len(s)
		}
	}
	for _, s := range sy {
		if strings.TrimSpace(s) != "" {
			total += len(s)
		}
	}
	if total == 0 || float64(kept)/float64(total) < minAlike {
		return nil, nil, false
	}
	pick := func(ts [][2]int, ch []bool) [][2]int {
		var out [][2]int
		for i, t := range ts {
			if ch[i] {
				out = append(out, t)
			}
		}
		return out
	}
	return pick(tx, cx), pick(ty, cy), true
}

// words splits s into words, runs of whitespace, and single other characters, as byte
// ranges.
func words(s string) [][2]int {
	var out [][2]int
	for i := 0; i < len(s); {
		j := i + 1
		switch {
		case isSpace(s[i]):
			for j < len(s) && isSpace(s[j]) {
				j++
			}
		case isWordByte(s[i]):
			for j < len(s) && isWordByte(s[j]) {
				j++
			}
		default:
			for j < len(s) && s[j] >= 0x80 && s[j] < 0xc0 {
				j++ // the rest of a UTF-8 character
			}
		}
		out = append(out, [2]int{i, j})
		i = j
	}
	return out
}

// isWordByte is a letter, digit, underscore, or any byte of a non-ASCII character.
func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// marker gathers marked byte ranges for one segment.
type marker struct {
	old, new []mark
}

type mark struct {
	from, to int
	kind     string
	rename   int // which rename, from 1, for a rename mark
}

func (m *marker) add(list *[]mark, from, to int, kind string) {
	if to > from {
		*list = append(*list, mark{from, to, kind, 0})
	}
}

// spans turns marked byte ranges into spans by line, within lines [from, to), joining marks of
// one kind with only whitespace between them.
func (m *marker) spans(t text, from, to int, marks []mark) []Span {
	if len(marks) == 0 {
		return nil
	}
	slices.SortFunc(marks, func(x, y mark) int { return x.from - y.from })
	var out []Span
	type run struct {
		line, from, to int // bytes within the line
		kind           string
		rename         int
	}
	var runs []run
	for _, mk := range marks {
		// Split at line ends; a mark never covers a line's ending.
		for p := mk.from; p < mk.to; {
			line, _ := lineCol(t, p)
			lineStart := t.start[line]
			body := t.body(line)
			end := min(mk.to, lineStart+len(body))
			if end > p && line >= from && line < to {
				r := run{line: line, from: p - lineStart, to: end - lineStart, kind: mk.kind, rename: mk.rename}
				if n := len(runs); n > 0 && runs[n-1].line == r.line && runs[n-1].kind == r.kind && runs[n-1].rename == r.rename &&
					strings.TrimSpace(body[runs[n-1].to:max(r.from, runs[n-1].to)]) == "" && r.from >= runs[n-1].from {
					runs[n-1].to = max(runs[n-1].to, r.to)
				} else {
					runs = append(runs, r)
				}
			}
			if line+1 >= len(t.start)-1 {
				break
			}
			p = max(t.start[line+1], end+1)
		}
	}
	for _, r := range runs {
		body := t.body(r.line)
		out = append(out, Span{Line: r.line, From: utf16Len(body[:r.from]), To: utf16Len(body[:r.to]), Kind: r.kind, Rename: r.rename})
	}
	return out
}
