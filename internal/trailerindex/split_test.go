package trailerindex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// slicing is a Source that records the slices of the walk it's asked for, and can make the
// one starting at failAt fail. Several readers use it at once.
type slicing struct {
	Source
	mu     sync.Mutex
	asked  [][2]int // Skip and Max of each
	failAt int
}

func (s *slicing) CommitMessages(ctx context.Context, m git.MessageRange, each func(id, message string) error) error {
	s.mu.Lock()
	s.asked = append(s.asked, [2]int{m.Skip, m.Max})
	fail := s.failAt > 0 && m.Skip == s.failAt
	s.mu.Unlock()
	if fail {
		return errors.New("this slice fails")
	}
	return s.Source.CommitMessages(ctx, m, each)
}

// longer is a history of n commits, most with trailers: a key every commit has, one with a
// value of its own, and one that only some have.
func longer(t *testing.T, n int) *gittest.Repo {
	t.Helper()
	fx := gittest.New(t)
	for i := range n {
		msg := fmt.Sprintf("Change %d\n\nTicket: #%d\nSigned-off-by: P%d <p%d@tidewater.example>", i, i, i%3, i%3)
		if i%4 == 0 {
			msg += "\nReviewed-by: Sarah Lind <sarah@tidewater.example>"
		}
		if i%5 == 0 {
			msg = fmt.Sprintf("Change %d with no trailers", i)
		}
		fx.Git("commit", "--allow-empty", "-q", "--cleanup=verbatim", "-m", msg)
	}
	return fx
}

// sameAs fails unless x holds what one reader reading the whole walk holds, in the same order.
func sameAs(t *testing.T, x, want *Index) {
	t.Helper()
	x.mu.RLock()
	defer x.mu.RUnlock()
	want.mu.RLock()
	defer want.mu.RUnlock()
	if !bytes.Equal(x.ids, want.ids) || !slices.Equal(x.first, want.first) {
		t.Fatalf("holds %d commits, want %d, in walk order", x.commitsLocked(), want.commitsLocked())
	}
	for c := range x.commitsLocked() {
		for j := x.first[c]; j < x.first[c+1]; j++ {
			if x.strs[x.pairs[j]] != want.strs[want.pairs[j]] {
				t.Fatalf("commit %d, trailer string %d: %q, want %q", c, j, x.strs[x.pairs[j]], want.strs[want.pairs[j]])
			}
		}
	}
}

// A long walk is read in slices side by side, and taken in walk order: the index, and the
// file it's kept in, are what one reader would have made (#900).
func TestALongWalkIsReadInSlicesAndTakenInOrder(t *testing.T) {
	t.Parallel()
	fx := longer(t, 40)
	repo := repoOf(t, fx)
	alone := Open(t.TempDir(), fx.Dir)
	defer alone.Close()
	read(t, alone, repo)

	dir := t.TempDir()
	x := Open(dir, fx.Dir)
	x.splits = splitting{from: 4, readers: 4, waitCount: true}
	src := &slicing{Source: repo}
	read(t, x, src)
	if want := [][2]int{{0, 0}, {10, 10}, {20, 10}, {30, 0}}; !slices.Equal(sortedAsks(src), want) {
		t.Fatalf("slices asked for = %v, want %v", src.asked, want)
	}
	if got, want := x.Status(), alone.Status(); got != want {
		t.Fatalf("status = %+v, want %+v", got, want)
	}
	sameAs(t, x, alone)
	if got, want := x.Keys(), alone.Keys(); !slices.Equal(got, want) {
		t.Fatalf("keys = %+v\nwant %+v", got, want)
	}
	x.Close()

	// As the next launch finds it: everything there, in order, and nothing to read.
	y := Open(dir, fx.Dir)
	defer y.Close()
	again := &counting{Source: repo}
	read(t, y, again)
	if again.read != 0 {
		t.Fatalf("opened again, it read %d commits", again.read)
	}
	sameAs(t, y, alone)
}

// When a slice can't be read, what came before it is kept, with the pass's place, and the
// next read carries on from there (#900).
func TestASliceThatFailsLeavesThePassToCarryOn(t *testing.T) {
	t.Parallel()
	fx := longer(t, 40)
	repo := repoOf(t, fx)
	alone := Open(t.TempDir(), fx.Dir)
	defer alone.Close()
	read(t, alone, repo)

	dir := t.TempDir()
	x := Open(dir, fx.Dir)
	x.splits = splitting{from: 4, readers: 4, waitCount: true}
	if err := x.Read(context.Background(), &slicing{Source: repo, failAt: 20}, nil); err == nil {
		t.Fatal("no error from a read whose third slice failed")
	}
	if st := x.Status(); st.Done || st.Read != 20 {
		t.Fatalf("status = %+v, want 20 commits read and not done", st)
	}
	x.Close()

	// Next launch: the place is in the file, and the rest is read by one reader.
	y := Open(dir, fx.Dir)
	defer y.Close()
	rest := &counting{Source: repo}
	read(t, y, rest)
	if rest.read != 20 {
		t.Fatalf("carrying on read %d commits, want the 20 after the place", rest.read)
	}
	if st := y.Status(); !st.Done || st.Read != 40 {
		t.Fatalf("status = %+v", st)
	}
	sameAs(t, y, alone)
}

func sortedAsks(s *slicing) [][2]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Clone(s.asked)
	slices.SortFunc(out, func(a, b [2]int) int { return a[0] - b[0] })
	return out
}
