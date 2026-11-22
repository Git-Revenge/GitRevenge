package trailerindex

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"slices"
)

// Finding a commit by its id: the commits are kept in the order they were read, so finding
// one takes their numbers in the order of their ids. Sorting a long history's ids takes a
// while (842 ms for the Linux kernel's 1.4 million, done the first time a row's chips were
// asked for, on the graph's own call), so the index keeps the order and every reading shares
// it. What's held only grows, so when more has been read, only the new commits are sorted,
// and merged in (#898).

// orderOf returns the numbers of the first n commits held, in the order of their ids. ids is
// the index's ids as they were at a moment it held at least n commits, size bytes each. The
// slice returned is shared and never changed.
func (x *Index) orderOf(ids []byte, size, n int) []uint32 {
	// An index that hasn't loaded its file yet has no table of commits at all, which counts
	// as -1, and is asked all the same (a pinned key's chips, as a tab opens).
	if n <= 0 || size == 0 {
		return nil
	}
	x.orderMu.Lock()
	defer x.orderMu.Unlock()
	have := x.order
	switch {
	case len(have) == n:
		return have
	case len(have) > n:
		// Asked about an older moment than the last: leave out what came since.
		out := make([]uint32, 0, n)
		for _, c := range have {
			if int(c) < n {
				out = append(out, c)
			}
		}
		return out
	}
	x.order = mergeOrders(ids, size, have, sortIDs(ids, size, len(have), n))
	return x.order
}

// sortIDs returns the numbers from to to, in the order of their commits' ids. It compares
// each id's first eight bytes as a number, which tells nearly all of them apart, and the
// whole id only when those are the same.
func sortIDs(ids []byte, size, from, to int) []uint32 {
	type entry struct {
		prefix uint64
		c      uint32
	}
	entries := make([]entry, to-from)
	for i := range entries {
		c := from + i
		entries[i] = entry{binary.BigEndian.Uint64(ids[c*size:]), uint32(c)}
	}
	slices.SortFunc(entries, func(a, b entry) int {
		if a.prefix != b.prefix {
			return cmp.Compare(a.prefix, b.prefix)
		}
		return bytes.Compare(idAt(ids, size, a.c), idAt(ids, size, b.c))
	})
	out := make([]uint32, len(entries))
	for i, e := range entries {
		out[i] = e.c
	}
	return out
}

// mergeOrders merges two lists of commit numbers, each in the order of their ids, into a new one.
func mergeOrders(ids []byte, size int, a, b []uint32) []uint32 {
	if len(a) == 0 {
		return b
	}
	out := make([]uint32, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if bytes.Compare(idAt(ids, size, a[i]), idAt(ids, size, b[j])) <= 0 {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}

// find returns the number of the commit whose id is raw, given the commits' numbers in the
// order of their ids, or false when none has it.
func find(ids []byte, size int, order []uint32, raw []byte) (int, bool) {
	if size == 0 || len(raw) != size {
		return 0, false
	}
	i, found := slices.BinarySearchFunc(order, raw, func(c uint32, want []byte) int {
		return bytes.Compare(idAt(ids, size, c), want)
	})
	if !found {
		return 0, false
	}
	return int(order[i]), true
}

func idAt(ids []byte, size int, c uint32) []byte {
	return ids[int(c)*size : (int(c)+1)*size]
}
