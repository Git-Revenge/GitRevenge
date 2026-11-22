package trailerindex

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
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

	begun := p.done > 0 // a pass being carried on has its begin frame in the file
	var b passBatch
	local := map[string]uint32{}
	walked, lastFlush := 0, time.Now()
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
		select {
		case n := <-counted:
			x.status.Total = n
		default:
		}
		st := x.status
		x.mu.Unlock()
		// A file that can't be written leaves the index working from memory.
		_ = x.write(frames)
		b = passBatch{}
		clear(local)
		lastFlush = time.Now()
		progress(st)
	}

	err := src.CommitMessages(ctx, m, func(id, message string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := hex.DecodeString(id)
		if err != nil {
			return err
		}
		x.mu.Lock()
		if x.idSize == 0 {
			x.idSize = len(raw)
		}
		size := x.idSize
		x.mu.Unlock()
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
	// What was walked before the read stopped is kept either way, with its place.
	flush()
	if err != nil {
		return err
	}
	x.mu.Lock()
	x.complete = appendTips(x.complete, p.tips)
	x.partial = nil
	x.status.Total = max(x.status.Total, x.status.Read)
	x.mu.Unlock()
	_ = x.write(encodeComplete(p.nonce))
	return nil
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
