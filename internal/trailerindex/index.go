package trailerindex

import (
	"cmp"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Index is one repository's trailers, in memory and in its file. Asking it something never
// runs git. Reading the history into it is Read's job, and can be going on meanwhile:
// answers then cover what has been read so far, and Status says how far that is.
//
// It holds, for each commit with any trailers, the commit's id and its trailers as pairs
// of numbers into a table of strings. Every question is answered by one pass over those
// pairs, which for the Linux kernel's five million trailers takes a few milliseconds, so
// there are no further tables to build or keep right.
type Index struct {
	// path is the index's file, or "" for an index kept in memory only.
	path string
	// parser is the trailer parser's version the file must have been written with.
	parser uint16

	// reading lets one read of the history run at a time, and splits says when a long walk
	// is read in slices side by side (see read.go): the defaults, unless a test says otherwise.
	reading sync.Mutex
	splits  splitting

	mu sync.RWMutex
	// loaded is set once the file has been read.
	loaded bool
	idSize int
	// strs is every key and value, once each. lookup finds one's number; it's only held
	// while the history is being read, since it's as big as the strings themselves.
	strs   []string
	lookup map[string]uint32
	// ids holds the commits' ids end to end, idSize bytes each. Commit i's trailers are
	// pairs[first[i]:first[i+1]], a key's number then a value's.
	ids   []byte
	first []uint32
	pairs []uint32
	// complete are the tips of the passes that finished: everything reachable from them is
	// here. partial is a pass that was cut short, to carry on with.
	complete [][]byte
	partial  *pass
	status   Status
	// gen counts changes, and keys is what Keys last worked out, for gen keysGen.
	gen     uint64
	keys    []KeyStat
	keysGen uint64

	// refs counts who has the index open (see Open).
	refs int

	// order is the numbers of the commits held, in the order of their ids, as of the last
	// time it was asked for (see orderOf). orderMu guards it, apart from mu, since keeping it
	// up is work that mustn't hold up a read of the history.
	orderMu sync.Mutex
	order   []uint32
}

// pass is one read of the history: from its tips, stopping at not.
type pass struct {
	passBegin
	done int
}

// Status says how far reading the history has got.
type Status struct {
	// Reading is true while a read is under way, of Total commits, Read of them so far.
	// Total is 0 until it has been counted.
	Reading bool `json:"reading"`
	Read    int  `json:"read"`
	Total   int  `json:"total"`
	// Done is true once everything the repository's branches and tags reach has been read,
	// as of the last look. Until then answers cover only part of the history.
	Done bool `json:"done"`
	// Commits is how many commits with trailers are held.
	Commits int `json:"commits"`
}

// New returns an empty index kept in memory only, for tests and for when there's nowhere
// to keep a file.
func New() *Index {
	return &Index{parser: git.TrailerParserVersion, loaded: true, first: []uint32{0}}
}

// Status returns how far reading has got.
func (x *Index) Status() Status {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.status
}

// load reads the index's file, once. A file from another version of the parser or the
// format is removed: it's a cache, and the history is still there to read.
func (x *Index) load() {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.loaded {
		return
	}
	x.loaded = true
	x.first = []uint32{0}
	if x.path == "" {
		return
	}
	f, err := os.Open(x.path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return
	}
	head := make([]byte, headerSize)
	if _, err := f.ReadAt(head, 0); err != nil {
		head = nil
	}
	idSize, err := decodeHeader(head, x.parser)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(x.path)
		return
	}
	x.idSize = idSize
	x.lookup = map[string]uint32{}
	// A commit two copies of the app both read is in the file twice.
	seen := map[[32]byte]struct{}{}
	passes := map[uint64]*pass{}
	var order []uint64
	_ = eachFrame(f, st.Size(), func(kind byte, payload []byte) error {
		switch kind {
		case frameBegin:
			if b, ok := decodeBegin(payload, idSize); ok && passes[b.nonce] == nil {
				passes[b.nonce] = &pass{passBegin: b}
				order = append(order, b.nonce)
			}
		case frameBatch:
			b, ok := decodeBatch(payload, idSize)
			if !ok {
				return nil
			}
			x.addLocked(b, seen)
			if p := passes[b.nonce]; p != nil {
				p.done = max(p.done, b.done)
			}
		case frameComplete:
			nonce, ok := decodeComplete(payload)
			if p := passes[nonce]; ok && p != nil {
				x.complete = appendTips(x.complete, p.tips)
				delete(passes, nonce)
			}
		}
		return nil
	})
	// The latest pass that never finished is the one to carry on with.
	for _, nonce := range slices.Backward(order) {
		if p := passes[nonce]; p != nil {
			x.partial = p
			break
		}
	}
	x.status.Commits = x.commitsLocked()
}

// appendTips adds tips to a set of them, each once.
func appendTips(set, tips [][]byte) [][]byte {
	for _, t := range tips {
		if !slices.ContainsFunc(set, func(have []byte) bool { return string(have) == string(t) }) {
			set = append(set, t)
		}
	}
	return set
}

func (x *Index) commitsLocked() int { return len(x.first) - 1 }

// addLocked takes a batch's commits in. seen, when given, holds the ids already here, and
// a commit in it is left out.
func (x *Index) addLocked(b passBatch, seen map[[32]byte]struct{}) {
	if len(b.commits) == 0 {
		return
	}
	if x.lookup == nil {
		x.lookup = make(map[string]uint32, len(x.strs))
		for i, s := range x.strs {
			x.lookup[s] = uint32(i)
		}
	}
	// The batch's strings, as their numbers here. New ones are copied out together, so the
	// batch's own text, most of which is already known, can be let go.
	global := make([]uint32, len(b.strs))
	var fresh []int
	size := 0
	for i, s := range b.strs {
		if n, ok := x.lookup[s]; ok {
			global[i] = n
			continue
		}
		fresh = append(fresh, i)
		size += len(s)
	}
	if len(fresh) > 0 {
		var all strings.Builder
		all.Grow(size)
		for _, i := range fresh {
			all.WriteString(b.strs[i])
		}
		text, at := all.String(), 0
		for _, i := range fresh {
			s := text[at : at+len(b.strs[i])]
			at += len(s)
			// A string a batch lists twice gets one number.
			n, ok := x.lookup[s]
			if !ok {
				n = uint32(len(x.strs))
				x.strs = append(x.strs, s)
				x.lookup[s] = n
			}
			global[i] = n
		}
	}
	for _, c := range b.commits {
		if len(c.pairs) == 0 || len(c.id) != x.idSize {
			continue
		}
		if seen != nil {
			var key [32]byte
			copy(key[:], c.id)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
		}
		x.ids = append(x.ids, c.id...)
		for _, local := range c.pairs {
			x.pairs = append(x.pairs, global[local])
		}
		x.first = append(x.first, uint32(len(x.pairs)))
	}
	x.gen++
}

// seenLocked returns the ids of every commit held, for a read that may meet them again.
func (x *Index) seenLocked() map[[32]byte]struct{} {
	seen := make(map[[32]byte]struct{}, x.commitsLocked())
	for i := 0; i < x.commitsLocked(); i++ {
		var key [32]byte
		copy(key[:], x.ids[i*x.idSize:(i+1)*x.idSize])
		seen[key] = struct{}{}
	}
	return seen
}

// number returns a string's number, or false when no trailer has it.
func (x *Index) numberLocked(s string) (uint32, bool) {
	if x.lookup != nil {
		n, ok := x.lookup[s]
		return n, ok
	}
	for i, have := range x.strs {
		if have == s {
			return uint32(i), true
		}
	}
	return 0, false
}

// KeyStat is one trailer key, as written, and how much it's used.
type KeyStat struct {
	Key string `json:"key"`
	// Commits is how many commits have it, Lines how many trailers (a commit can have
	// several), and Values how many different values it has.
	Commits int `json:"commits"`
	Lines   int `json:"lines"`
	Values  int `json:"values"`
}

// Keys returns every key any commit read so far has, most commits first. Keys are as
// written: "Signed-off-by" and "signed-off-by" are two.
func (x *Index) Keys() []KeyStat {
	x.mu.RLock()
	if x.keys != nil && x.keysGen == x.gen {
		defer x.mu.RUnlock()
		return slices.Clone(x.keys)
	}
	gen := x.gen
	stats := map[uint32]*KeyStat{}
	values := map[uint64]struct{}{}
	var keys once
	for i := 0; i < x.commitsLocked(); i++ {
		mine := x.pairs[x.first[i]:x.first[i+1]]
		keys.reset()
		for j := 0; j < len(mine); j += 2 {
			key, value := mine[j], mine[j+1]
			st := stats[key]
			if st == nil {
				st = &KeyStat{Key: x.strs[key]}
				stats[key] = st
			}
			st.Lines++
			// A commit counts once for a key, however many trailers it has under it.
			if keys.first(uint64(key)) {
				st.Commits++
			}
			pair := uint64(key)<<32 | uint64(value)
			if _, ok := values[pair]; !ok {
				values[pair] = struct{}{}
				st.Values++
			}
		}
	}
	x.mu.RUnlock()
	out := make([]KeyStat, 0, len(stats))
	for _, st := range stats {
		out = append(out, *st)
	}
	slices.SortFunc(out, func(a, b KeyStat) int {
		return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(a.Key, b.Key))
	})
	x.mu.Lock()
	if x.gen == gen {
		x.keys, x.keysGen = out, gen
	}
	x.mu.Unlock()
	return slices.Clone(out)
}

// fewTrailers is how many different things once keeps in a list. Nearly every commit has
// fewer trailers than this, and looking back through so few is quicker than a set.
const fewTrailers = 32

// once says whether a thing is met for the first time within one commit's trailers: a
// commit counts once for a key or a value, however many times it says it. A commit's
// things are kept in a short list, and past fewTrailers in a set, since looking back
// through a long list would take time growing with the square of the commit's trailers,
// and a crafted commit can have hundreds of thousands (#893).
type once struct {
	list []uint64
	set  map[uint64]struct{}
}

// reset starts on the next commit.
func (o *once) reset() {
	o.list = o.list[:0]
	o.set = nil
}

// first reports whether v hasn't been met in this commit yet, and remembers it.
func (o *once) first(v uint64) bool {
	if o.set != nil {
		if _, ok := o.set[v]; ok {
			return false
		}
		o.set[v] = struct{}{}
		return true
	}
	if slices.Contains(o.list, v) {
		return false
	}
	o.list = append(o.list, v)
	if len(o.list) > fewTrailers {
		o.set = make(map[uint64]struct{}, 4*len(o.list))
		for _, have := range o.list {
			o.set[have] = struct{}{}
		}
	}
	return true
}

// ValueStat is one value of a key, and how many commits have it.
type ValueStat struct {
	Value   string `json:"value"`
	Commits int    `json:"commits"`
}

// Values returns the values of a key, as written, most commits first.
func (x *Index) Values(key string) []ValueStat {
	x.mu.RLock()
	defer x.mu.RUnlock()
	k, ok := x.numberLocked(key)
	if !ok {
		return []ValueStat{}
	}
	counts := map[uint32]int{}
	var values once
	for i := 0; i < x.commitsLocked(); i++ {
		mine := x.pairs[x.first[i]:x.first[i+1]]
		values.reset()
		for j := 0; j < len(mine); j += 2 {
			// A commit counts once for a value, however many times it says it.
			if mine[j] == k && values.first(uint64(mine[j+1])) {
				counts[mine[j+1]]++
			}
		}
	}
	out := make([]ValueStat, 0, len(counts))
	for v, n := range counts {
		out = append(out, ValueStat{Value: x.strs[v], Commits: n})
	}
	slices.SortFunc(out, func(a, b ValueStat) int {
		return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(a.Value, b.Value))
	})
	return out
}

// Commits returns the ids of the commits that have a trailer with this key and value, both
// as written. An empty value asks for every commit with the key.
func (x *Index) Commits(key, value string) []string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	out := []string{}
	k, ok := x.numberLocked(key)
	if !ok {
		return out
	}
	var v uint32
	if value != "" {
		if v, ok = x.numberLocked(value); !ok {
			return out
		}
	}
	for i := 0; i < x.commitsLocked(); i++ {
		mine := x.pairs[x.first[i]:x.first[i+1]]
		for j := 0; j < len(mine); j += 2 {
			if mine[j] == k && (value == "" || mine[j+1] == v) {
				out = append(out, hex.EncodeToString(x.ids[i*x.idSize:(i+1)*x.idSize]))
				break
			}
		}
	}
	return out
}

// Trailers returns the trailers of one commit, in the order it has them, and whether the
// commit is held: one that isn't may have none, or may not have been read yet.
func (x *Index) Trailers(id string) ([]git.Trailer, bool) {
	raw, err := hex.DecodeString(id)
	// What's held only grows and is never changed, so the look is at the index as it is now
	// without holding anyone up (as Reading does).
	x.mu.RLock()
	ids, first, pairs, strs, size := x.ids, x.first, x.pairs, x.strs, x.idSize
	x.mu.RUnlock()
	if err != nil {
		return nil, false
	}
	c, ok := find(ids, size, x.orderOf(ids, size, len(first)-1), raw)
	if !ok {
		return nil, false
	}
	mine := pairs[first[c]:first[c+1]]
	out := make([]git.Trailer, 0, len(mine)/2)
	for j := 0; j < len(mine); j += 2 {
		out = append(out, git.Trailer{Key: strs[mine[j]], Value: strs[mine[j+1]]})
	}
	return out, true
}

// write appends frames to the index's file. An index with no file keeps going in memory.
func (x *Index) write(frames []byte) error {
	if x.path == "" {
		return nil
	}
	x.mu.RLock()
	idSize := x.idSize
	x.mu.RUnlock()
	if idSize == 0 {
		return errors.New("the trailer index doesn't know the size of a commit id yet")
	}
	return appendFrames(x.path, x.parser, idSize, frames)
}
