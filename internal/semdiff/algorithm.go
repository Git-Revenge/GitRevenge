package semdiff

import (
	"context"

	"znkr.io/diff"
	"znkr.io/diff/textdiff"
)

// lineBlocks returns the places where a and b differ, by algorithm, with each run of changed
// lines slid to where git would put it (see slideChanges).
func lineBlocks(ctx context.Context, a, b text, algorithm string) ([]Block, error) {
	var out []Block
	if algorithm != Patience && algorithm != Histogram {
		out = myers(a, b, 0, a.n(), 0, b.n(), algorithm == Minimal)
	} else {
		m := newMatcher(a, b)
		var err error
		finish := func(g gap) {
			if err != nil {
				return
			}
			if err = ctx.Err(); err != nil {
				return
			}
			switch {
			case g.a0 == g.a1 && g.b0 == g.b1:
			case g.a0 == g.a1 || g.b0 == g.b1 || !g.common:
				out = append(out, Block{OldStart: g.a0, OldEnd: g.a1, NewStart: g.b0, NewEnd: g.b1})
			default:
				out = append(out, myers(a, b, g.a0, g.a1, g.b0, g.b1, false)...)
			}
		}
		if algorithm == Patience {
			m.patience(0, a.n(), 0, b.n(), finish)
		} else {
			m.histogram(0, a.n(), 0, b.n(), finish)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return compact(a, b, joinAdjacent(out)), nil
}

// myers compares lines [a0, a1) of a with [b0, b1) of b by znkr.io/diff's Myers. minimal
// turns off the shortcuts that keep a diff with many changes fast. Sliding the changes is left
// to lineBlocks, which does it for every algorithm.
func myers(a, b text, a0, a1, b0, b1 int, minimal bool) []Block {
	opts := []textdiff.Option{diff.Context(0)}
	if minimal {
		opts = append(opts, diff.Minimal())
	}
	hunks := textdiff.Hunks(a.span(a0, a1), b.span(b0, b1), opts...)
	out := make([]Block, 0, len(hunks))
	for _, h := range hunks {
		out = append(out, Block{OldStart: a0 + h.LineNoX, OldEnd: a0 + h.EndLineNoX, NewStart: b0 + h.LineNoY, NewEnd: b0 + h.EndLineNoY})
	}
	return out
}

// joinAdjacent merges blocks with no unchanged line between them.
func joinAdjacent(blocks []Block) []Block {
	out := blocks[:0]
	for _, blk := range blocks {
		if n := len(out); n > 0 && out[n-1].OldEnd == blk.OldStart && out[n-1].NewEnd == blk.NewStart {
			out[n-1].OldEnd, out[n-1].NewEnd = blk.OldEnd, blk.NewEnd
			continue
		}
		out = append(out, blk)
	}
	return out
}

// gap is a stretch patience or histogram couldn't match, for Myers to finish: lines
// [a0, a1) of a against [b0, b1) of b. common is false when the two have no line in common,
// so the whole stretch is one change.
type gap struct {
	a0, a1, b0, b1 int
	common         bool
}

// matcher holds both files' lines as numbers, equal lines getting equal numbers, for the
// anchored algorithms.
type matcher struct {
	a, b []int32
}

func newMatcher(a, b text) *matcher {
	ids := map[string]int32{}
	number := func(t text) []int32 {
		out := make([]int32, t.n())
		for i := range out {
			l := t.line(i)
			id, ok := ids[l]
			if !ok {
				id = int32(len(ids))
				ids[l] = id
			}
			out[i] = id
		}
		return out
	}
	return &matcher{a: number(a), b: number(b)}
}

// patience compares lines [a0, a1) and [b0, b1) as git's --patience does. The lines that
// appear exactly once in each are matched in their longest common order. Around each of
// those anchors the matching lines on either side are taken in too, and the stretches left
// between anchors are compared the same way. A stretch with lines in common but none unique
// goes to finish for Myers.
func (m *matcher) patience(a0, a1, b0, b1 int, finish func(gap)) {
	if a0 == a1 || b0 == b1 {
		finish(gap{a0, a1, b0, b1, false})
		return
	}
	type seen struct{ inA, inB, atB int }
	counts := map[int32]*seen{}
	for i := a0; i < a1; i++ {
		s := counts[m.a[i]]
		if s == nil {
			s = &seen{}
			counts[m.a[i]] = s
		}
		s.inA++
	}
	common := false
	for j := b0; j < b1; j++ {
		if s := counts[m.b[j]]; s != nil {
			s.inB++
			s.atB = j
			common = true
		}
	}
	if !common {
		finish(gap{a0, a1, b0, b1, false})
		return
	}
	// The unique lines in a's order, with where each is in b.
	var pairs [][2]int
	for i := a0; i < a1; i++ {
		if s := counts[m.a[i]]; s.inA == 1 && s.inB == 1 {
			pairs = append(pairs, [2]int{i, s.atB})
		}
	}
	anchors := longestIncreasing(pairs)
	if len(anchors) == 0 {
		finish(gap{a0, a1, b0, b1, true})
		return
	}
	i, j := a0, b0
	for k := 0; ; {
		// The stretch runs to the next anchor, less the lines before it that match too.
		ni, nj := a1, b1
		if k < len(anchors) {
			ni, nj = anchors[k][0], anchors[k][1]
			for ni > i && nj > j && m.a[ni-1] == m.b[nj-1] {
				ni--
				nj--
			}
		}
		// And less the lines after the last anchor that match.
		for i < ni && j < nj && m.a[i] == m.b[j] {
			i++
			j++
		}
		if ni > i || nj > j {
			m.patience(i, ni, j, nj, finish)
		}
		if k == len(anchors) {
			return
		}
		// Anchors next to each other are one run.
		for k+1 < len(anchors) && anchors[k+1][0] == anchors[k][0]+1 && anchors[k+1][1] == anchors[k][1]+1 {
			k++
		}
		i, j = anchors[k][0]+1, anchors[k][1]+1
		k++
	}
}

// longestIncreasing returns the longest run of pairs whose second halves increase, keeping
// their order: patience sorting.
func longestIncreasing(pairs [][2]int) [][2]int {
	if len(pairs) == 0 {
		return nil
	}
	// tops[k] is the index of the pair ending the best run of length k+1 found so far.
	var tops []int
	back := make([]int, len(pairs))
	for i, p := range pairs {
		lo, hi := 0, len(tops)
		for lo < hi {
			mid := (lo + hi) / 2
			if pairs[tops[mid]][1] < p[1] {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		back[i] = -1
		if lo > 0 {
			back[i] = tops[lo-1]
		}
		if lo == len(tops) {
			tops = append(tops, i)
		} else {
			tops[lo] = i
		}
	}
	out := make([][2]int, len(tops))
	for k, i := len(tops)-1, tops[len(tops)-1]; k >= 0; k, i = k-1, back[i] {
		out[k] = pairs[i]
	}
	return out
}

// maxChain is how often a line may appear in a stretch and still anchor a histogram match,
// as in git and JGit.
const maxChain = 64

// histogram compares lines [a0, a1) and [b0, b1) as git's --histogram does. It finds the
// longest common run built around the line that appears least often in a, splits there, and
// compares each side the same way. A stretch where every common line is too common goes to
// finish for Myers.
func (m *matcher) histogram(a0, a1, b0, b1 int, finish func(gap)) {
	if a0 == a1 || b0 == b1 {
		finish(gap{a0, a1, b0, b1, false})
		return
	}
	where := map[int32][]int{}
	for i := a0; i < a1; i++ {
		where[m.a[i]] = append(where[m.a[i]], i)
	}
	count := func(i int) int { return len(where[m.a[i]]) }

	found, common := false, false
	var as, ae, bs, be int
	best := maxChain + 1
	for j := b0; j < b1; {
		occurs := where[m.b[j]]
		if len(occurs) == 0 {
			j++
			continue
		}
		common = true
		if len(occurs) > best {
			j++
			continue
		}
		next := j + 1
		for k := 0; k < len(occurs); {
			i := occurs[k]
			s, t := i, j
			low := len(occurs)
			for s > a0 && t > b0 && m.a[s-1] == m.b[t-1] {
				s--
				t--
				if low > 1 {
					low = min(low, count(s))
				}
			}
			e, f := i+1, j+1
			for e < a1 && f < b1 && m.a[e] == m.b[f] {
				if low > 1 {
					low = min(low, count(e))
				}
				e++
				f++
			}
			next = max(next, f)
			if !found || e-s > ae-as || low < best {
				found = true
				as, ae, bs, be = s, e, t, f
				best = low
			}
			// The next place this line is in a, past the run just found.
			k++
			for k < len(occurs) && occurs[k] < e {
				k++
			}
		}
		j = next
	}
	if !found {
		finish(gap{a0, a1, b0, b1, common})
		return
	}
	m.histogram(a0, as, b0, bs, finish)
	m.histogram(ae, a1, be, b1, finish)
}
