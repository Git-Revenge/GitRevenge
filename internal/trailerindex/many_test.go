package trailerindex

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A commit with a quarter of a million trailers is counted in time that grows with them, not
// with their square, and still counts once for each key and value however often it says
// them, past the point where once moves from a list to a set (#893).
func TestACommitWithManyTrailersIsCountedOnce(t *testing.T) {
	t.Parallel()
	const lines, values, others = 250000, 100000, 50
	var b strings.Builder
	for i := range lines {
		// Each value is said two or three times, and the other keys over and over.
		fmt.Fprintf(&b, "Key: v%d\n", i%values)
		if i%3 == 0 {
			fmt.Fprintf(&b, "Other-%d: x\n", i%others)
		}
	}
	x, _ := indexOf(b.String(), "Key: v1\nOther-0: x")

	done := make(chan struct{})
	go func() {
		defer close(done)
		keys := x.Keys()
		if len(keys) != 1+others {
			t.Errorf("%d keys, want %d", len(keys), 1+others)
			return
		}
		if k := keys[0]; k.Key != "Key" || k.Commits != 2 || k.Lines != lines+1 || k.Values != values {
			t.Errorf("Key = %+v, want 2 commits, %d lines, %d values", k, lines+1, values)
		}
		for _, k := range keys[1:] {
			want := 1
			if k.Key == "Other-0" {
				want = 2
			}
			if k.Commits != want || k.Values != 1 {
				t.Errorf("%+v: want %d commits and one value", k, want)
			}
		}
		vals := x.Values("Key")
		if len(vals) != values || vals[0].Value != "v1" || vals[0].Commits != 2 || vals[1].Commits != 1 {
			t.Errorf("Values: %d, first %+v, then %+v", len(vals), vals[0], vals[1])
		}
		r := x.Reading(Rules{})
		k, ok := r.Key(KeyID("Key"))
		if !ok || k.Commits != 2 || k.Lines != lines+1 || k.Values != values {
			t.Errorf("reading's Key = %+v", k)
		}
		if top := r.Values(KeyID("Key")); len(top) != values || top[0].Label != "v1" || top[0].Commits != 2 {
			t.Errorf("reading's values: %d, first %+v", len(top), top[0])
		}
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		panic("still counting a commit's trailers after 20 seconds")
	}
}
