package git

import (
	"context"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// The demo's feature/offline-cache before and after an autosquash rebase (2026-11-27).
const demoRangeDiff = `1:  1637a6b ! 1:  f6df5ac Cache the last reading from each station
    @@ src/cache.js (new)
     +const cache = new Map()
     +
     +export function remember(id, reading) {
    -+  cache.set(id, { reading, at: Date.now() })
    ++  if (reading) cache.set(id, { reading, at: Date.now() })
     +}
2:  eda7bec ! 2:  f16c299 wip
    @@ Commit message
      ## src/cache.js ##
     @@ src/cache.js: const cache = new Map()
      export function remember(id, reading) {
    -   cache.set(id, { reading, at: Date.now() })
    +   if (reading) cache.set(id, { reading, at: Date.now() })
      }
     +
     +export function recall(id, maxAgeMs = 15 * 60_000) {
3:  b7f914f < -:  ------- fixup! Cache the last reading from each station
4:  50d62a1 = 3:  bce321d Fall back to the cached reading when a station is down
5:  2f75eb1 = 4:  ae8b098 debugging
`

func TestParseRangeDiffReadsTheDemo(t *testing.T) {
	pairs := ParseRangeDiff(demoRangeDiff)
	type row struct {
		old, new int
		kind     string
		into     int
	}
	want := []row{{1, 1, PairChanged, 0}, {2, 2, PairAround, 0}, {3, 0, PairFolded, 1}, {4, 3, PairSame, 0}, {5, 4, PairSame, 0}}
	if len(pairs) != len(want) {
		t.Fatalf("%d pairs: %+v", len(pairs), pairs)
	}
	for i, w := range want {
		p := pairs[i]
		if p.Old != w.old || p.New != w.new || p.Kind != w.kind || p.Into != w.into || p.Moved {
			t.Errorf("pair %d = %+v, want %+v", i+1, p, w)
		}
	}
	if pairs[0].OldID != "1637a6b" || pairs[0].NewID != "f6df5ac" || pairs[2].NewID != "" || pairs[0].Subject != "Cache the last reading from each station" {
		t.Errorf("first and third = %+v / %+v", pairs[0], pairs[2])
	}
	if !strings.Contains(pairs[0].Text, "if (reading)") || !strings.HasPrefix(pairs[3].Text, "4:  50d62a1 = 3:") {
		t.Errorf("text kept: %q / %q", pairs[0].Text, pairs[3].Text)
	}
}

func TestParseRangeDiffWideNumbers(t *testing.T) {
	text := " 9:  aaaaaaa =  9:  bbbbbbb Ninth\n10:  ccccccc ! 10:  ddddddd Tenth\n    @@ Commit message\n    -    Tenth\n    +    Tenth, again\n     \n"
	pairs := ParseRangeDiff(text)
	if len(pairs) != 2 || pairs[0].Old != 9 || pairs[1].New != 10 || pairs[1].Kind != PairMessage {
		t.Fatalf("pairs = %+v", pairs)
	}
}

func TestMarkMovedMarksOnlyWhatMoved(t *testing.T) {
	// One commit moved from first to last: the others kept their order.
	pairs := []RangePair{{Old: 1, New: 4}, {Old: 2, New: 1}, {Old: 3, New: 2}, {Old: 4, New: 3}}
	markMoved(pairs)
	for i, p := range pairs {
		if p.Moved != (i == 0) {
			t.Errorf("pair %d moved = %v", i+1, p.Moved)
		}
	}
}

// versionsRepo makes a branch's two versions from one base: old has five commits, newer is
// what an autosquash rebase, a reword, a reorder and one more commit make of them.
func versionsRepo(t *testing.T) (fx *gittest.Repo, old, newer RangeSide) {
	t.Helper()
	// Commits the size of real ones, so git pairs one changed in a line with its old self.
	lines := func(prefix string, n int, change map[int]string) string {
		var b strings.Builder
		for i := 1; i <= n; i++ {
			if s, ok := change[i]; ok {
				b.WriteString(s + "\n")
			} else {
				b.WriteString(prefix + " line " + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + "\n")
			}
		}
		return b.String()
	}
	cache := lines("cache", 12, nil)
	// Line 11 is in wip's context: the fixup changes what wip's hunk sits under.
	cacheFixed := lines("cache", 12, map[int]string{11: "remember only a reading"})
	withRecall := func(c string) string { return c + "\n" + lines("recall", 8, nil) }
	fx = gittest.New(t)
	fx.CommitFile("a.txt", lines("a", 12, nil), "base")
	base := fx.Head()
	fx.Git("checkout", "-q", "-b", "old")
	fx.CommitFile("cache.txt", cache, "Add the cache")
	fx.CommitFile("cache.txt", withRecall(cache), "wip")
	fx.CommitFile("cache.txt", withRecall(cacheFixed), "fixup! Add the cache")
	fx.CommitFile("fall.txt", lines("fall", 10, nil), "Fall back to the cache")
	fx.CommitFile("a.txt", lines("a", 12, map[int]string{6: "wind in knots"}), "Report wind in metres")
	oldTip := fx.Head()
	fx.Git("checkout", "-q", "-b", "new", base)
	fx.CommitFile("cache.txt", cacheFixed, "Add the cache")
	fx.CommitFile("cache.txt", withRecall(cacheFixed), "wip")
	fx.CommitFile("a.txt", lines("a", 12, map[int]string{6: "wind in knots"}), "Report wind in knots")
	fx.CommitFile("fall.txt", lines("fall", 10, nil), "Fall back to the cache")
	fx.CommitFile("test.txt", lines("test", 10, nil), "Test the expiry")
	return fx, RangeSide{Base: base, Tip: oldTip}, RangeSide{Base: base, Tip: fx.Head()}
}

func TestRangeDiffPairsTwoVersions(t *testing.T) {
	t.Parallel()
	fx, old, newer := versionsRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	pairs, text, err := repo.RangeDiff(ctx, old, newer, false)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	moved := 0
	for _, p := range pairs {
		kinds[p.Subject] = p.Kind
		if p.Moved {
			moved++
		}
		if (p.Old > 0) != (len(p.OldID) == 40) || (p.New > 0) != (len(p.NewID) == 40) {
			t.Errorf("%q: ids %q %q should be full where it has a side", p.Subject, p.OldID, p.NewID)
		}
	}
	want := map[string]string{
		"Add the cache":          PairChanged,
		"wip":                    PairAround,
		"fixup! Add the cache":   PairFolded,
		"Fall back to the cache": PairSame,
		"Report wind in knots":   PairMessage,
		"Test the expiry":        PairNew,
	}
	for subject, kind := range want {
		if kinds[subject] != kind {
			t.Errorf("%q: kind %q, want %q (all: %v)\n%s", subject, kinds[subject], kind, kinds, text)
		}
	}
	if moved != 1 {
		t.Errorf("%d moved, want 1: the reword and Fall back swapped places\n%s", moved, text)
	}
	for _, p := range pairs {
		if p.Subject == "Report wind in knots" && p.OldSubject != "Report wind in metres" {
			t.Errorf("the reword keeps its old subject: %+v", p)
		}
	}

	// Looser matching finds no fewer pairs.
	loose, _, err := repo.RangeDiff(ctx, old, newer, true)
	if err != nil || len(loose) > len(pairs) {
		t.Fatalf("loose: %d pairs, %v", len(loose), err)
	}
}

func TestRangeCountLeavesMergesOut(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "base")
	base := fx.Head()
	fx.Git("checkout", "-q", "-b", "side")
	fx.CommitFile("b.txt", "b\n", "side")
	fx.Git("checkout", "-q", "main")
	fx.CommitFile("c.txt", "c\n", "main")
	if err := fx.Merge("side"); err != nil {
		t.Fatal(err)
	}
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	commits, merges, err := repo.RangeCount(context.Background(), RangeSide{Base: base, Tip: fx.Head()})
	if err != nil || commits != 2 || merges != 1 {
		t.Fatalf("commits %d, merges %d, %v", commits, merges, err)
	}
}
