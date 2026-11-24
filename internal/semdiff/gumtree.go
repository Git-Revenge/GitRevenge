package semdiff

import (
	"context"
	"hash/maphash"
	"slices"
	"sort"
)

// Matching the two versions' syntax trees (#947), after GumTree (Falleri et al., ASE 2014)
// with the fast recovery its authors called "GumTree simple" (ICSE 2024): identical subtrees
// are matched from the top down, containers from the bottom up by how many of their
// descendants match, and under each matched pair the children left over are matched by
// identity, then by shape, then by being the only one of their type. There is no optimal
// edit script and so no max_size (research note, phase 4).
//
// The token diff stays the diff. Matching only finds what it can't: a token it left changed
// on both sides that the trees match to the same token elsewhere moved, as arguments do when
// reordered; and lines it left matched whose syntax changed around them moved into new
// structure, as statements do when wrapped in an if.

const (
	// minTopHeight is the smallest subtree matched from the top down: a lone token matches
	// too many others to say anything.
	minTopHeight = 2
	// minDice is how alike two containers' descendants have to be to match bottom up.
	minDice = 0.5
	// maxTreeNodes bounds the nodes of each tree; a bigger file isn't matched.
	maxTreeNodes = 120_000
	// maxMatchSteps bounds the work matching may do in all, so it can't go quadratic.
	maxMatchSteps = 30_000_000
	// maxAmbiguous bounds the pairs of identical subtrees, each found more than once, that
	// are weighed against each other.
	maxAmbiguous = 20_000
)

var treeSeed = maphash.MakeSeed()

// gnode is a node of a tree being matched: a named syntax node, or a token as a leaf.
type gnode struct {
	name   uint64 // a named node's type, or for a leaf its token's class
	label  string // a leaf's key
	parent int32
	kids   []int32
	hash   uint64 // type, label and kids: equal hashes are identical subtrees
	shape  uint64 // type and kids' shapes, labels left out
	height int32
	size   int32 // nodes in the subtree, itself included
	pre    int32 // where it comes in pre-order
	tok    int32 // a leaf's token, else -1
	typ    string
}

// gtree is a tree being matched. Node 0 is the root, standing for the whole file.
type gtree struct {
	n   []gnode
	pre []int32 // nodes by pre-order position
}

// buildTree makes a version's tree from its tokens and recorded nodes. Whitespace and
// optional punctuation aren't leaves. With want set, only the top-level nodes overlapping
// those byte ranges are kept, with everything in them. It returns nil for a tree too big to
// match.
func buildTree(v *version, nodes []nodeRec, want [][2]int) *gtree {
	if want != nil {
		nodes, v = keepWanted(v, nodes, want)
	}
	type item struct {
		from, to int
		node     int // into nodes, or -1 for a token
		tok      int
		order    int // recording order: for equal ranges a parent comes after its child
	}
	var items []item
	for i, nr := range nodes {
		items = append(items, item{from: nr.from, to: nr.to, node: i, tok: -1, order: i})
	}
	for i, t := range v.toks {
		if t.class == clsSpace || t.ignorable {
			continue
		}
		items = append(items, item{from: t.from, to: t.to, node: -1, tok: i})
	}
	if len(items)+1 > maxTreeNodes {
		return nil
	}
	// Outer before inner: by start, then longest, then a named node before a token, then the
	// parent (recorded later) before its child.
	sort.SliceStable(items, func(x, y int) bool {
		a, b := items[x], items[y]
		if a.from != b.from {
			return a.from < b.from
		}
		if a.to != b.to {
			return a.to > b.to
		}
		if (a.node < 0) != (b.node < 0) {
			return a.node >= 0
		}
		return a.order > b.order
	})
	t := &gtree{n: make([]gnode, 0, len(items)+1)}
	t.n = append(t.n, gnode{parent: -1, tok: -1})
	type open struct {
		idx  int32
		from int
		to   int
	}
	stack := []open{{0, -1, int(^uint(0) >> 1)}}
	for _, it := range items {
		for len(stack) > 1 {
			top := stack[len(stack)-1]
			if top.from <= it.from && it.to <= top.to {
				break
			}
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1].idx
		g := gnode{parent: parent, tok: -1}
		if it.node >= 0 {
			nr := nodes[it.node]
			g.name, g.typ = nr.name, nr.typ
		} else {
			tk := v.toks[it.tok]
			g.name, g.label, g.tok = uint64(tk.class)+1, tk.key, int32(it.tok)
		}
		idx := int32(len(t.n))
		t.n = append(t.n, g)
		t.n[parent].kids = append(t.n[parent].kids, idx)
		if it.node >= 0 {
			stack = append(stack, open{idx, it.from, it.to})
		}
	}
	// Hashes, heights and sizes from the leaves up; pre-order positions from the root down.
	var post func(i int32)
	post = func(i int32) {
		g := &t.n[i]
		h := maphash.String(treeSeed, g.label) ^ g.name*0x9e3779b97f4a7c15
		s := g.name * 0xc2b2ae3d27d4eb4f
		g.height, g.size = 1, 1
		for _, k := range g.kids {
			post(k)
			kg := &t.n[k]
			h = mixHash(h, kg.hash)
			s = mixHash(s, kg.shape)
			g.height = max(g.height, kg.height+1)
			g.size += kg.size
		}
		g.hash, g.shape = h, s
	}
	post(0)
	t.pre = make([]int32, 0, len(t.n))
	var walk func(i int32)
	walk = func(i int32) {
		t.n[i].pre = int32(len(t.pre))
		t.pre = append(t.pre, i)
		for _, k := range t.n[i].kids {
			walk(k)
		}
	}
	walk(0)
	return t
}

func mixHash(h, x uint64) uint64 {
	h ^= x + 0x9e3779b97f4a7c15 + (h << 6) + (h >> 2)
	return h * 0xff51afd7ed558ccd
}

// within reports whether node d is in the subtree of node top.
func (t *gtree) within(d, top int32) bool {
	p, q := t.n[top].pre, t.n[d].pre
	return q >= p && q < p+t.n[top].size
}

// matching is a matching of two trees under way.
type matching struct {
	ctx    context.Context
	a, b   *gtree
	ma, mb []int32 // each node's partner, or -1
	steps  int
	asked  int  // calls to tooLong, so ctx is checked every so often
	ended  bool // ctx ended
}

// tooLong reports whether matching has used its budget, or ctx has ended.
func (m *matching) tooLong() bool {
	m.asked++
	if !m.ended && m.asked%256 == 0 && m.ctx.Err() != nil {
		m.ended = true
	}
	return m.ended || m.steps > maxMatchSteps
}

// matchTrees matches a and b. It returns each token's partner on the other side, or -1, and
// false when the budget ran out first, when nothing should be made of it.
func matchTrees(ctx context.Context, a, b *gtree, na, nb int) (pa, pb []int32, ok bool) {
	m := &matching{ctx: ctx, a: a, b: b, ma: filled32(len(a.n)), mb: filled32(len(b.n))}
	m.topDown()
	if m.tooLong() {
		return nil, nil, false
	}
	m.bottomUp()
	if m.tooLong() {
		return nil, nil, false
	}
	pa, pb = filled32(na), filled32(nb)
	for i, g := range a.n {
		if g.tok >= 0 && m.ma[i] >= 0 {
			if o := b.n[m.ma[i]]; o.tok >= 0 {
				pa[g.tok], pb[o.tok] = o.tok, g.tok
			}
		}
	}
	return pa, pb, true
}

func filled32(n int) []int32 {
	s := make([]int32, n)
	for i := range s {
		s[i] = -1
	}
	return s
}

// link matches two nodes.
func (m *matching) link(x, y int32) {
	m.ma[x], m.mb[y] = y, x
}

// linkSubtrees matches two identical subtrees node by node.
func (m *matching) linkSubtrees(x, y int32) {
	m.link(x, y)
	kx, ky := m.a.n[x].kids, m.b.n[y].kids
	m.steps += len(kx)
	for i := range min(len(kx), len(ky)) {
		m.linkSubtrees(kx[i], ky[i])
	}
}

// topDown matches the biggest identical subtrees first: a subtree found once on each side
// is matched; one found more often waits, and is weighed by how alike its parents are.
func (m *matching) topDown() {
	openA, openB := map[int32][]int32{}, map[int32][]int32{}
	push := func(t *gtree, open map[int32][]int32, i int32) { open[t.n[i].height] = append(open[t.n[i].height], i) }
	tallest := func(open map[int32][]int32) int32 {
		h := int32(0)
		for k, v := range open {
			if len(v) > 0 && k > h {
				h = k
			}
		}
		return h
	}
	push(m.a, openA, 0)
	push(m.b, openB, 0)
	var ambiguous [][2]int32
	for !m.tooLong() {
		ha, hb := tallest(openA), tallest(openB)
		if min(ha, hb) < minTopHeight {
			break
		}
		if ha != hb {
			t, open, h := m.a, openA, ha
			if hb > ha {
				t, open, h = m.b, openB, hb
			}
			for _, i := range open[h] {
				for _, k := range t.n[i].kids {
					push(t, open, k)
				}
			}
			delete(open, h)
			continue
		}
		h := ha
		xs, ys := openA[h], openB[h]
		delete(openA, h)
		delete(openB, h)
		m.steps += len(xs) + len(ys)
		byHash := map[uint64][]int32{}
		for _, y := range ys {
			byHash[m.b.n[y].hash] = append(byHash[m.b.n[y].hash], y)
		}
		countA := map[uint64]int{}
		for _, x := range xs {
			countA[m.a.n[x].hash]++
		}
		waiting := map[int32]bool{}
		for _, x := range xs {
			hx := m.a.n[x].hash
			cands := byHash[hx]
			switch {
			case len(cands) == 0:
			case countA[hx] == 1 && len(cands) == 1:
				m.linkSubtrees(x, cands[0])
			default:
				waiting[x] = true
				for _, y := range cands {
					waiting[-1-y] = true
					if len(ambiguous) < maxAmbiguous {
						ambiguous = append(ambiguous, [2]int32{x, y})
					}
				}
			}
		}
		for _, x := range xs {
			if m.ma[x] < 0 && !waiting[x] {
				for _, k := range m.a.n[x].kids {
					push(m.a, openA, k)
				}
			}
		}
		for _, y := range ys {
			if m.mb[y] < 0 && !waiting[-1-y] {
				for _, k := range m.b.n[y].kids {
					push(m.b, openB, k)
				}
			}
		}
	}
	// The repeated ones go to the pairs whose parents are most alike, then nearest in order.
	type scored struct {
		x, y int32
		dice float64
		near int32
	}
	var s []scored
	for _, p := range ambiguous {
		px, py := m.a.n[p[0]].parent, m.b.n[p[1]].parent
		d := 0.0
		if px >= 0 && py >= 0 {
			d = m.dice(px, py)
		}
		near := m.a.n[p[0]].pre - m.b.n[p[1]].pre
		s = append(s, scored{p[0], p[1], d, max(near, -near)})
		if m.tooLong() {
			return
		}
	}
	slices.SortStableFunc(s, func(x, y scored) int {
		switch {
		case x.dice > y.dice:
			return -1
		case x.dice < y.dice:
			return 1
		}
		return int(x.near - y.near)
	})
	for _, p := range s {
		if m.ma[p.x] < 0 && m.mb[p.y] < 0 {
			m.linkSubtrees(p.x, p.y)
		}
	}
}

// dice is how alike two subtrees are: twice their matched descendants over their sizes.
func (m *matching) dice(x, y int32) float64 {
	gx, gy := m.a.n[x], m.b.n[y]
	common := 0
	for p := gx.pre + 1; p < gx.pre+gx.size; p++ {
		d := m.a.pre[p]
		if q := m.ma[d]; q >= 0 && m.b.within(q, y) && q != y {
			common++
		}
	}
	m.steps += int(gx.size)
	if gx.size+gy.size <= 2 {
		return 0
	}
	return 2 * float64(common) / float64(gx.size+gy.size-2)
}

// bottomUp matches containers, deepest first, to the container on the other side holding
// most of their matched descendants, then recovers what's under each pair.
func (m *matching) bottomUp() {
	for p := len(m.a.pre) - 1; p >= 1 && !m.tooLong(); p-- {
		x := m.a.pre[p]
		gx := m.a.n[x]
		if m.ma[x] >= 0 || len(gx.kids) == 0 {
			continue
		}
		// Candidates: unmatched containers of its type above its descendants' partners.
		seen := map[int32]bool{}
		var cands []int32
		looked := 0
		for q := gx.pre + 1; q < gx.pre+gx.size && looked < 1000 && len(cands) < 20; q++ {
			partner := m.ma[m.a.pre[q]]
			if partner < 0 {
				continue
			}
			looked++
			for c := m.b.n[partner].parent; c > 0; c = m.b.n[c].parent {
				m.steps++
				if seen[c] {
					break
				}
				seen[c] = true
				if m.mb[c] < 0 && m.b.n[c].name == gx.name {
					cands = append(cands, c)
				}
			}
		}
		best, bestDice := int32(-1), minDice
		for _, c := range cands {
			if d := m.dice(x, c); d >= bestDice {
				best, bestDice = c, d
			}
		}
		if best >= 0 {
			m.link(x, best)
			m.recover(x, best)
		}
	}
	if m.ma[0] < 0 {
		m.link(0, 0)
	}
	m.recover(0, 0)
}

// recover matches what's left under two matched nodes: children identical to one child on
// the other side, then those the same shape as one, then the only child of a type on each
// side, which is recovered in turn.
func (m *matching) recover(x, y int32) {
	if m.tooLong() {
		return
	}
	left := func(t *gtree, mine []int32, i int32) []int32 {
		var out []int32
		for _, k := range t.n[i].kids {
			if mine[k] < 0 {
				out = append(out, k)
			}
		}
		return out
	}
	ua, ub := left(m.a, m.ma, x), left(m.b, m.mb, y)
	if len(ua) == 0 || len(ub) == 0 {
		return
	}
	m.steps += len(ua) + len(ub)
	unique := func(key func(t *gtree, i int32) uint64, then func(x, y int32)) {
		ca, cb := map[uint64]int{}, map[uint64]int32{}
		nb := map[uint64]int{}
		for _, i := range ua {
			if m.ma[i] < 0 {
				ca[key(m.a, i)]++
			}
		}
		for _, j := range ub {
			if m.mb[j] < 0 {
				k := key(m.b, j)
				nb[k]++
				cb[k] = j
			}
		}
		for _, i := range ua {
			if m.ma[i] >= 0 {
				continue
			}
			k := key(m.a, i)
			if ca[k] == 1 && nb[k] == 1 && m.mb[cb[k]] < 0 {
				then(i, cb[k])
			}
		}
	}
	unique(func(t *gtree, i int32) uint64 { return t.n[i].hash }, m.linkSubtrees)
	unique(func(t *gtree, i int32) uint64 { return t.n[i].shape }, func(i, j int32) {
		m.link(i, j)
		m.recover(i, j)
	})
	unique(func(t *gtree, i int32) uint64 { return t.n[i].name }, func(i, j int32) {
		m.link(i, j)
		m.recover(i, j)
	})
}

// keepWanted narrows a version's nodes and tokens to the declarations overlapping want: the
// children of the file's root node that overlap a wanted byte range, with everything in
// them. The roots themselves stay, and tokens outside every declaration stay where they
// overlap want. Tokens left out become whitespace, so every token keeps its index.
func keepWanted(v *version, nodes []nodeRec, want [][2]int) ([]nodeRec, *version) {
	overlaps := func(from, to int) bool {
		for _, w := range want {
			if from < w[1] && to > w[0] {
				return true
			}
		}
		return false
	}
	order := make([]int, len(nodes))
	for i := range order {
		order[i] = i
	}
	// Outer before inner; for equal ranges the parent, recorded later, first.
	slices.SortFunc(order, func(x, y int) int {
		switch {
		case nodes[x].from != nodes[y].from:
			return nodes[x].from - nodes[y].from
		case nodes[x].to != nodes[y].to:
			return nodes[y].to - nodes[x].to
		}
		return y - x
	})
	keep := make([]bool, len(nodes))
	var decls [][2]int // the declarations: depth one
	type open struct{ to, depth int }
	var stack []open
	under := -1 // the end of the kept declaration being passed through, or -1
	for _, i := range order {
		n := nodes[i]
		// Close the nodes this one doesn't sit inside.
		for len(stack) > 0 && (n.from >= stack[len(stack)-1].to || n.to > stack[len(stack)-1].to) {
			stack = stack[:len(stack)-1]
		}
		depth := len(stack)
		switch depth {
		case 0:
			keep[i] = true
		case 1:
			decls = append(decls, [2]int{n.from, n.to})
			if overlaps(n.from, n.to) {
				keep[i], under = true, n.to
			} else {
				under = -1
			}
		default:
			keep[i] = under >= n.to
		}
		stack = append(stack, open{n.to, depth})
	}
	var outNodes []nodeRec
	for i, n := range nodes {
		if keep[i] {
			outNodes = append(outNodes, n)
		}
	}
	// decls are in order and don't overlap.
	declAt := func(from, to int) (int, bool) {
		k := sort.Search(len(decls), func(k int) bool { return decls[k][1] >= to })
		return k, k < len(decls) && decls[k][0] <= from
	}
	sub := *v
	sub.toks = make([]tok, len(v.toks))
	for i, t := range v.toks {
		k, in := declAt(t.from, t.to)
		if (in && overlaps(decls[k][0], decls[k][1])) || (!in && overlaps(t.from, t.to)) {
			sub.toks[i] = t
		} else {
			sub.toks[i] = tok{class: clsSpace, from: t.from, to: t.to}
		}
	}
	return outNodes, &sub
}
