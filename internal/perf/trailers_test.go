package perf

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// What the trailer index has to manage on a big repository. The first read is as long as
// git takes to hand over every message, and isn't held to a number: it happens once, in
// the background. What is held is everything after it.
const (
	// A question to the index is answered while the user watches.
	targetTrailerQuery = 250 * time.Millisecond
	// The whole list of keys is counted once and kept until something is read.
	targetTrailerKeys = 2 * time.Second
	// Opening a repository whose trailers are already read: load the file, find nothing new.
	targetTrailerReopen = 15 * time.Second
	// Reading the whole index through the rules, done once and kept until either changes.
	targetTrailerReading = 20 * time.Second
	// Finding the questions Trailer care asks, done once for a reading and kept with it.
	targetTrailerCare = 5 * time.Second
)

// countingMessages says how many commits a read asked git for.
type countingMessages struct {
	trailerindex.Source
	read int
}

func (c *countingMessages) CommitMessages(ctx context.Context, m git.MessageRange, each func(id, message string) error) error {
	return c.Source.CommitMessages(ctx, m, func(id, message string) error {
		c.read++
		return each(id, message)
	})
}

func heapNow() uint64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

func TestTrailerIndex(t *testing.T) {
	r := benchRepo(t)
	ctx := context.Background()
	dir := t.TempDir()
	commonDir, err := r.ReadString(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		t.Fatal(err)
	}

	before := heapNow()
	x := trailerindex.Open(dir, commonDir)
	first := &countingMessages{Source: r}
	start := time.Now()
	var last time.Time
	if err := x.Read(ctx, first, func(s trailerindex.Status) {
		if time.Since(last) > 20*time.Second {
			last = time.Now()
			t.Logf("  %d of %d commits after %s", s.Read, s.Total, time.Since(start).Round(time.Second))
		}
	}); err != nil {
		t.Fatal(err)
	}
	st := x.Status()
	t.Logf("first read: %d commits, %d with trailers, in %s", first.read, st.Commits, time.Since(start).Round(time.Second))
	t.Logf("held in memory: %d MB", (heapNow()-before)>>20)
	var size int64
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if info, err := e.Info(); err == nil {
				size += info.Size()
				_ = filepath.Join(dir, e.Name())
			}
		}
	}
	t.Logf("on disk: %d MB", size>>20)

	start = time.Now()
	keys := x.Keys()
	took := time.Since(start)
	t.Logf("keys: %d, counted in %s", len(keys), took)
	if took > targetTrailerKeys {
		t.Errorf("counting the keys took %s, target %s", took, targetTrailerKeys)
	}
	if len(keys) == 0 {
		t.Fatal("no trailers in the benchmark repository")
	}
	lines := 0
	for _, k := range keys {
		lines += k.Lines
	}
	t.Logf("trailer lines: %d; most used: %s on %d commits, %d values", lines, keys[0].Key, keys[0].Commits, keys[0].Values)

	start = time.Now()
	values := x.Values(keys[0].Key)
	took = time.Since(start)
	t.Logf("values of %s: %d, in %s", keys[0].Key, len(values), took)
	if took > targetTrailerKeys {
		t.Errorf("listing a key's values took %s, target %s", took, targetTrailerKeys)
	}

	start = time.Now()
	commits := x.Commits(keys[0].Key, values[0].Value)
	took = time.Since(start)
	t.Logf("commits with %s: %s: %d, in %s", keys[0].Key, values[0].Value, len(commits), took)
	if took > targetTrailerQuery {
		t.Errorf("finding the commits with a trailer took %s, target %s", took, targetTrailerQuery)
	}
	if len(commits) != values[0].Commits {
		t.Errorf("%d commits found, and the value is counted on %d", len(commits), values[0].Commits)
	}
	// The reading: keys joined, values told apart by kind, commit ids checked with git.
	asked := 0
	start = time.Now()
	reading := x.Reading(trailerindex.Rules{
		Total: first.read,
		Commits: func(ids []string) map[string]string {
			asked = len(ids)
			full, err := r.CommitIDs(ctx, ids)
			if err != nil {
				t.Fatal(err)
			}
			return full
		},
	})
	took = time.Since(start)
	rkeys := reading.Keys()
	held := map[string]int{}
	for _, k := range rkeys {
		held[k.Held]++
	}
	t.Logf("reading: %d keys (%d offered, %d rare, %d identifiers, %d everywhere), %d ids asked of git, in %s", len(rkeys), held[""], held[trailerindex.HeldRare], held[trailerindex.HeldIdentifier], held[trailerindex.HeldEverywhere], asked, took.Round(time.Millisecond))
	if took > targetTrailerReading {
		t.Errorf("reading the index took %s, target %s", took, targetTrailerReading)
	}
	for i, k := range rkeys {
		if i < 14 {
			t.Logf("  %-18s %-10s %8d commits %7d values  held: %q", k.Label, k.Kind, k.Commits, k.Values, k.Held)
		}
	}
	start = time.Now()
	fixed := reading.Values("Fixes")
	took = time.Since(start)
	if len(fixed) > 0 {
		start = time.Now()
		n := len(reading.Commits("Fixes", fixed[0].ID))
		t.Logf("values of Fixes: %d in %s; commits that fix %q: %d in %s", len(fixed), took, fixed[0].Label, n, time.Since(start))
		if n != fixed[0].Commits {
			t.Errorf("%d commits found, and the value is counted on %d", n, fixed[0].Commits)
		}
		if d := time.Since(start); d > targetTrailerQuery {
			t.Errorf("finding the commits with a value took %s, target %s", d, targetTrailerQuery)
		}
	}

	// The questions Trailer care asks: every pair of keys is looked at, and a long history
	// must not raise hundreds.
	start = time.Now()
	questions, open := reading.Care(nil, 30)
	byKind := map[string]int{}
	all, _ := reading.Care(nil, 0)
	for _, q := range all {
		byKind[q.Kind]++
	}
	people, peopleOpen := reading.People(5)
	took = time.Since(start)
	t.Logf("care: %d questions (%d spelling, %d same things), %d listed; %d people written two ways, %d listed; in %s", open, byKind[trailerindex.CareSpelling], byKind[trailerindex.CareSame], len(questions), peopleOpen, len(people), took.Round(time.Millisecond))
	for i, q := range questions {
		if i < 14 {
			t.Logf("  %-9s %-22s (%d) as %-22s (%d)  shared %d  %v", q.Kind, q.OtherLabel, q.OtherCommits, q.KeyLabel, q.Commits, q.Shared, q.Examples)
		}
	}
	if took > targetTrailerCare {
		t.Errorf("finding the questions took %s, target %s", took, targetTrailerCare)
	}

	// A page of graph rows asks for each commit's chips: the first asking sorts the commits
	// by id, once, and after that a page is answered while the user scrolls.
	ids, err := r.ReadString(ctx, "rev-list", "--max-count=400", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	page := strings.Fields(ids)
	pinned := []string{"Signed-off-by", "Fixes"}
	start = time.Now()
	reading.Chips(page[0], pinned)
	first400 := time.Since(start)
	start = time.Now()
	chips := 0
	for _, id := range page {
		chips += len(reading.Chips(id, pinned))
	}
	took = time.Since(start)
	t.Logf("chips: the first lookup (which sorts the commits) in %s; a page of %d rows, %d chips, in %s", first400.Round(time.Millisecond), len(page), chips, took)
	if took > targetTrailerQuery {
		t.Errorf("a page of chips took %s, target %s", took, targetTrailerQuery)
	}
	if chips == 0 {
		t.Error("no commit of the newest 400 has a Signed-off-by chip")
	}
	x.Close()

	// As the next launch finds it: everything on disk, and nothing new to read.
	start = time.Now()
	y := trailerindex.Open(dir, commonDir)
	defer y.Close()
	again := &countingMessages{Source: r}
	if err := y.Read(ctx, again, nil); err != nil {
		t.Fatal(err)
	}
	took = time.Since(start)
	t.Logf("opened again: %d commits read, %d held, in %s", again.read, y.Status().Commits, took.Round(time.Millisecond))
	if again.read != 0 {
		t.Errorf("opening again read %d commits, want none", again.read)
	}
	if y.Status().Commits != st.Commits {
		t.Errorf("opened again it holds %d commits, had %d", y.Status().Commits, st.Commits)
	}
	if took > targetTrailerReopen {
		t.Errorf("opening again took %s, target %s", took, targetTrailerReopen)
	}
}
