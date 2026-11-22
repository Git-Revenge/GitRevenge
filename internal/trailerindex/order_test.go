package trailerindex

import (
	"bytes"
	"encoding/hex"
	"math/rand/v2"
	"slices"
	"testing"
)

// The index keeps its commits in the order of their ids as batches come in, sorting only what
// each brings, and every reading and Index.Trailers finds a commit through it (#898).
func TestTheIndexKeepsItsCommitsInIDOrder(t *testing.T) {
	t.Parallel()
	rnd := rand.New(rand.NewPCG(1, 2))
	x := New()
	x.idSize = 20
	var all [][]byte
	addBatch := func(n int) {
		b := passBatch{strs: []string{"Key", "value"}}
		for i := range n {
			id := make([]byte, 20)
			for k := range id {
				id[k] = byte(rnd.IntN(256))
			}
			// Some share their first eight bytes with an earlier one, so the whole id decides.
			if len(all) > 0 && i%7 == 0 {
				copy(id, all[rnd.IntN(len(all))][:8])
			}
			all = append(all, id)
			b.commits = append(b.commits, batchCommit{id: id, pairs: []uint32{0, 1}})
		}
		x.mu.Lock()
		x.addLocked(b, nil)
		x.mu.Unlock()
	}
	check := func(order []uint32, n int) {
		t.Helper()
		if len(order) != n {
			t.Fatalf("order covers %d commits, want %d", len(order), n)
		}
		if !slices.IsSortedFunc(order, func(a, b uint32) int { return bytes.Compare(all[a], all[b]) }) {
			t.Fatal("the order isn't by id")
		}
		seen := make([]bool, n)
		for _, c := range order {
			if seen[c] {
				t.Fatalf("commit %d is in the order twice", c)
			}
			seen[c] = true
		}
	}

	for _, n := range []int{1, 50, 0, 333, 1000} {
		addBatch(n)
		x.mu.RLock()
		ids, held := x.ids, x.commitsLocked()
		x.mu.RUnlock()
		check(x.orderOf(ids, 20, held), held)
		// Every commit is found, through the index and through a reading of it.
		r := x.Reading(Rules{})
		for _, c := range []int{0, held / 2, held - 1} {
			id := hex.EncodeToString(all[c])
			if got, ok := find(ids, 20, r.byID, all[c]); !ok || got != c {
				t.Fatalf("find(commit %d) = %d, %v", c, got, ok)
			}
			if tr, ok := x.Trailers(id); !ok || len(tr) != 1 || tr[0].Key != "Key" {
				t.Fatalf("Trailers(commit %d) = %+v, %v", c, tr, ok)
			}
			if hits, ok := r.Picked(id, []Pick{{Key: "key", Value: "text:value"}}); !ok || !slices.Equal(hits, []bool{true}) {
				t.Fatalf("Picked(commit %d) = %v, %v", c, hits, ok)
			}
			if chips := r.Chips(id, []string{"key"}); len(chips) != 1 || chips[0].Label != "value" {
				t.Fatalf("Chips(commit %d) = %+v", c, chips)
			}
		}
	}
	// A reading of an older moment gets the order of what it held then.
	x.mu.RLock()
	ids := x.ids
	x.mu.RUnlock()
	check(x.orderOf(ids, 20, 51), 51)

	// What isn't held isn't found, and an id of the wrong size is no id.
	if _, ok := x.Trailers(hex.EncodeToString(make([]byte, 20))); ok {
		t.Error("found a commit that isn't held")
	}
	if _, ok := x.Trailers("abc"); ok {
		t.Error("found a commit by a short id")
	}
	if _, ok := x.Trailers(hex.EncodeToString(make([]byte, 32))); ok {
		t.Error("found a commit by an id of another size")
	}
}

// An index that hasn't loaded its file yet is asked about all the same, as a tab opens with a
// key pinned, and holds nothing: it used to make a slice of -1 commits and panic.
func TestAnIndexNotYetLoadedIsAskedAboutAllTheSame(t *testing.T) {
	t.Parallel()
	x := Open(t.TempDir(), t.TempDir())
	defer x.Close()
	id := hex.EncodeToString(make([]byte, 20))
	r := x.Reading(Rules{})
	if chips := r.Chips(id, []string{"key"}); chips != nil {
		t.Errorf("Chips = %+v", chips)
	}
	if _, ok := r.Picked(id, []Pick{{Key: "key", Value: "text:value"}}); ok {
		t.Error("Picked found a commit")
	}
	if _, ok := x.Trailers(id); ok {
		t.Error("Trailers found a commit")
	}
}
