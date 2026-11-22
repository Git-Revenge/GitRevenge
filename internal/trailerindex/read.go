package trailerindex

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Source is what reading the history needs from git; *git.Repo is one.
type Source interface {
	HistoryTips(ctx context.Context) ([]string, error)
	MissingObjects(ctx context.Context, ids []string) ([]string, error)
	CountMessages(ctx context.Context, m git.MessageRange) (int, error)
	CommitMessages(ctx context.Context, m git.MessageRange, each func(id, message string) error) error
}

// How much is read between writes to the file: a batch is one frame, and what a crash or a
// closed tab loses is at most the batch being gathered. The time limit keeps progress
// moving on a slow disk.
const (
	batchCommits = 20000
	batchEvery   = 1500 * time.Millisecond
)

// Read brings the index up to date with the repository: it reads the trailers of every
// commit its branches, remote-tracking branches and tags reach that it hasn't read
// already, newest first, and returns when it has them all or ctx ends. progress, if given,
// is told how far it has got, a few times a second at most.
//
// The first read of a repository walks its whole history. Every read after that asks git
// only for what the tips of the last finished read don't reach, so with nothing new it
// reads nothing. A read that's cut short has its place kept, and the next one carries on
// from there.
//
// One read runs at a time for an index; another call waits its turn, then finds the work
// done.
func (x *Index) Read(ctx context.Context, src Source, progress func(Status)) error {
	x.reading.Lock()
	defer x.reading.Unlock()
	x.load()
	if progress == nil {
		progress = func(Status) {}
	}
	defer func() {
		x.mu.Lock()
		x.status.Reading = false
		// The table that finds a string's number is only for taking commits in.
		x.lookup = nil
		st := x.status
		x.mu.Unlock()
		progress(st)
	}()

	// A commit already held can turn up again only when a pass that was cut short can't
	// be carried on, and the history has to be walked again from the top.
	again := false
	x.mu.RLock()
	p := x.partial
	x.mu.RUnlock()
	if p != nil {
		missing, err := src.MissingObjects(ctx, hexIDs(append(append([][]byte{}, p.tips...), p.not...)))
		if err != nil {
			return err
		}
		if len(missing) == 0 {
			if err := x.run(ctx, src, p, false, progress); err != nil {
				return err
			}
		} else {
			// One of the commits it started from is gone, so its place means nothing now.
			x.mu.Lock()
			x.partial = nil
			x.mu.Unlock()
			again = true
		}
	}

	tips, err := src.HistoryTips(ctx)
	if err != nil {
		return err
	}
	x.mu.RLock()
	complete := hexIDs(x.complete)
	x.mu.RUnlock()
	known := make(map[string]bool, len(complete))
	for _, id := range complete {
		known[id] = true
	}
	var fresh []string
	for _, id := range tips {
		if !known[id] {
			fresh = append(fresh, id)
		}
	}
	if len(fresh) > 0 {
		missing, err := src.MissingObjects(ctx, complete)
		if err != nil {
			return err
		}
		gone := make(map[string]bool, len(missing))
		for _, id := range missing {
			gone[id] = true
		}
		var not []string
		for _, id := range complete {
			if !gone[id] {
				not = append(not, id)
			}
		}
		next := &pass{passBegin: passBegin{nonce: newNonce()}}
		if next.tips, err = rawIDs(fresh); err != nil {
			return err
		}
		if next.not, err = rawIDs(not); err != nil {
			return err
		}
		if err := x.run(ctx, src, next, again, progress); err != nil {
			return err
		}
	}
	x.mu.Lock()
	x.status.Done = true
	x.mu.Unlock()
	return nil
}

// run walks one pass, from where it stopped if it's one being carried on, and marks it
// complete when the walk ends. With again set, commits already held are left out.
//
// A long walk is read in slices side by side (see split), but taken in walk order: this
// reader takes the first slice as it reads it, and each of the others is taken once the ones
// before it are in. So what's held, and what's in the file, is always the walk from its start
// up to the pass's place, as it is when one reader reads it all, and a read that's cut short
// carries on from there.
func (x *Index) run(ctx context.Context, src Source, p *pass, again bool, progress func(Status)) error {
	m := git.MessageRange{Tips: hexIDs(p.tips), Not: hexIDs(p.not), Skip: p.done}
	x.mu.Lock()
	x.partial = p
	x.status.Reading, x.status.Done = true, false
	x.status.Read, x.status.Total = p.done, 0
	var seen map[[32]byte]struct{}
	if again {
		seen = x.seenLocked()
	}
	st := x.status
	x.mu.Unlock()
	progress(st)

	// Counting is quick beside reading, and runs alongside so the first trailers aren't
	// kept waiting for it.
	counted := make(chan int, 1)
	go func() {
		n, err := src.CountMessages(ctx, git.MessageRange{Tips: m.Tips, Not: m.Not})
		if err != nil {
			n = 0
		}
		counted <- n
	}()

	// The slices others read, and where this reader stops once they have the rest (-1 while
	// it reads to the end). They all stop when the pass does, however it ends.
	sctx, stop := context.WithCancel(ctx)
	var later []*slice
	end := -1
	defer func() {
		stop()
		for _, s := range later {
			<-s.done
		}
	}()

	begun := p.done > 0 // a pass being carried on has its begin frame in the file
	var b passBatch
	local := map[string]uint32{}
	walked, lastFlush, total := 0, time.Now(), -1
	number := func(s string) uint32 {
		n, ok := local[s]
		if !ok {
			n = uint32(len(b.strs))
			b.strs = append(b.strs, s)
			local[s] = n
		}
		return n
	}
	flush := func() {
		if total < 0 {
			select {
			case n := <-counted:
				// The walk's length is known: the rest of it can be shared out.
				total = n
				later, end = x.split(sctx, src, m, p.nonce, p.done+walked, n)
			default:
			}
		}
		if walked == 0 && begun {
			return
		}
		p.done += walked
		walked = 0
		b.nonce, b.done = p.nonce, p.done
		var frames []byte
		if !begun {
			frames = encodeBegin(p.passBegin)
			begun = true
		}
		frames = append(frames, encodeBatch(b)...)
		x.mu.Lock()
		x.addLocked(b, seen)
		x.status.Read = p.done
		x.status.Commits = x.commitsLocked()
		x.status.Total = max(total, 0)
		st := x.status
		x.mu.Unlock()
		// A file that can't be written leaves the index working from memory.
		_ = x.write(frames)
		b = passBatch{}
		clear(local)
		lastFlush = time.Now()
		progress(st)
	}
	if x.splits.waitCount {
		select {
		case total = <-counted:
			later, end = x.split(sctx, src, m, p.nonce, p.done, total)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	err := src.CommitMessages(ctx, m, func(id, message string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if end >= 0 && p.done+walked >= end {
			return errSliceRead // the rest is being read by others
		}
		raw, size, err := x.idOf(id)
		if err != nil {
			return err
		}
		walked++
		if trailers := git.MessageTrailers(message); len(trailers) > 0 && len(raw) == size {
			c := batchCommit{id: raw, pairs: make([]uint32, 0, len(trailers)*2)}
			for _, t := range trailers {
				c.pairs = append(c.pairs, number(t.Key), number(t.Value))
			}
			b.commits = append(b.commits, c)
		}
		if walked >= batchCommits || time.Since(lastFlush) >= batchEvery {
			flush()
		}
		return nil
	})
	if errors.Is(err, errSliceRead) {
		err = nil
	}
	// What was walked before the read stopped is kept either way, with its place.
	flush()
	if err != nil {
		return err
	}
	// The slices the others read, in order, each once the one before it is in.
	for _, s := range later {
		select {
		case <-s.done:
		case <-ctx.Done():
			return ctx.Err()
		}
		if s.err != nil {
			return s.err
		}
		if s.from != p.done {
			return fmt.Errorf("reading trailers lost its place: a slice of the walk starts at %d, and the walk is at %d", s.from, p.done)
		}
		x.take(s, p, seen, progress)
	}
	x.mu.Lock()
	x.complete = appendTips(x.complete, p.tips)
	x.partial = nil
	x.status.Total = max(x.status.Total, x.status.Read)
	x.mu.Unlock()
	_ = x.write(encodeComplete(p.nonce))
	return nil
}

// A long walk is read in slices side by side. Git spends a first read unpacking commits,
// one at a time on one core: on the Linux kernel, one `git log` of every message takes 29
// seconds, and four over a quarter of the walk each take 11 (#900).
const (
	// splitFrom is how many commits a walk must still have to be shared out: a shorter one
	// is read in a few seconds by one reader.
	splitFrom = 100000
	// mostReaders is how many read a walk at once, at most. A few more would be a little
	// quicker still, but the app has other work, and so may the machine.
	mostReaders = 4
)

// errSliceRead stops a reader at the end of its slice of a walk.
var errSliceRead = errors.New("the rest of the walk is being read by others")

// splitting says, for tests, when a walk is shared out and among how many readers. Zero
// values take splitFrom and as many readers as half the machine's cores, up to mostReaders.
// waitCount has the pass's own reader wait for the walk's length before it reads, so a
// test's short walk is shared out for certain; otherwise it's shared out at the first batch
// after the length is known.
type splitting struct {
	from, readers int
	waitCount     bool
}

// slice is part of a walk, read by a reader of its own.
type slice struct {
	// from is where it starts in the walk, and max how many commits it reads, or 0 for the
	// rest of the walk. end is where it ended.
	from, max, end int
	// frames are its batches, encoded as they're kept in the file, waiting to be taken.
	frames [][]byte
	err    error
	done   chan struct{}
}

// split shares out the rest of a walk that's at at, total commits long, when it's long: the
// reader that called keeps reading the first part, and stops where split returns; others
// read the rest, a part each, side by side. A short walk is left to one reader, and split
// returns no slices and -1.
func (x *Index) split(ctx context.Context, src Source, m git.MessageRange, nonce uint64, at, total int) ([]*slice, int) {
	readers := x.splits.readers
	if readers == 0 {
		readers = min(mostReaders, runtime.NumCPU()/2)
	}
	from := x.splits.from
	if from == 0 {
		from = splitFrom
	}
	rest := total - at
	if readers < 2 || rest < max(from, readers) {
		return nil, -1
	}
	part := (rest + readers - 1) / readers
	var out []*slice
	for i := 1; i < readers; i++ {
		s := &slice{from: at + i*part, done: make(chan struct{})}
		if i < readers-1 {
			s.max = part
		}
		out = append(out, s)
		go x.readSlice(ctx, src, m, nonce, s)
	}
	return out, at + part
}

// readSlice reads one slice of a pass's walk into frames, to be taken in turn.
func (x *Index) readSlice(ctx context.Context, src Source, m git.MessageRange, nonce uint64, s *slice) {
	defer close(s.done)
	var b passBatch
	local := map[string]uint32{}
	number := func(v string) uint32 {
		n, ok := local[v]
		if !ok {
			n = uint32(len(b.strs))
			b.strs = append(b.strs, v)
			local[v] = n
		}
		return n
	}
	at, framed := s.from, s.from
	frame := func() {
		if at == framed {
			return
		}
		b.nonce, b.done = nonce, at
		s.frames = append(s.frames, encodeBatch(b))
		b = passBatch{}
		clear(local)
		framed = at
	}
	m.Skip, m.Max = s.from, s.max
	s.err = src.CommitMessages(ctx, m, func(id, message string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, size, err := x.idOf(id)
		if err != nil {
			return err
		}
		at++
		if trailers := git.MessageTrailers(message); len(trailers) > 0 && len(raw) == size {
			c := batchCommit{id: raw, pairs: make([]uint32, 0, len(trailers)*2)}
			for _, t := range trailers {
				c.pairs = append(c.pairs, number(t.Key), number(t.Value))
			}
			b.commits = append(b.commits, c)
		}
		if at-framed >= batchCommits {
			frame()
		}
		return nil
	})
	frame()
	s.end = at
}

// take puts a slice another reader read into the index and its file, as if the pass's own
// reader had read it: the pass's place moves to where the slice ended.
func (x *Index) take(s *slice, p *pass, seen map[[32]byte]struct{}, progress func(Status)) {
	x.mu.Lock()
	for _, f := range s.frames {
		if b, ok := decodeBatch(f[frameHeaderSize:], x.idSize); ok {
			x.addLocked(b, seen)
		}
	}
	p.done = s.end
	x.status.Read = p.done
	x.status.Commits = x.commitsLocked()
	st := x.status
	x.mu.Unlock()
	for _, f := range s.frames {
		// A file that can't be written leaves the index working from memory.
		_ = x.write(f)
	}
	s.frames = nil
	progress(st)
}

// idOf reads a commit's id, and says the size the index keeps ids at, which the first id
// read sets.
func (x *Index) idOf(id string) ([]byte, int, error) {
	raw, err := hex.DecodeString(id)
	if err != nil {
		return nil, 0, err
	}
	x.mu.Lock()
	if x.idSize == 0 {
		x.idSize = len(raw)
	}
	size := x.idSize
	x.mu.Unlock()
	return raw, size, nil
}

func newNonce() uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint64(b[:])
}

func hexIDs(ids [][]byte) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = hex.EncodeToString(id)
	}
	return out
}

func rawIDs(ids []string) ([][]byte, error) {
	out := make([][]byte, len(ids))
	for i, id := range ids {
		raw, err := hex.DecodeString(id)
		if err != nil {
			return nil, err
		}
		out[i] = raw
	}
	return out, nil
}
