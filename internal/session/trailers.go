package session

import (
	"context"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// trailerReader keeps an open repository's trailer index up to date, in the background.
//
// It reads once the graph has loaded as far as it will for now, so the first screen of
// commits isn't kept waiting by a walk of the whole history, and again whenever a branch
// or a tag has moved since. The index knows what it has read, so a look that finds nothing
// new costs two short git commands.
type trailerReader struct {
	index  *trailerindex.Index
	repo   *git.Repo
	report func(trailerindex.Status)

	mu sync.Mutex
	// stale is true until a read has finished since the refs last moved.
	stale   bool
	running bool
	closed  bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newTrailerReader(index *trailerindex.Index, repo *git.Repo, report func(trailerindex.Status)) *trailerReader {
	return &trailerReader{index: index, repo: repo, report: report, stale: true}
}

// refsMoved says a branch or a tag changed, so there may be commits that haven't been read.
func (t *trailerReader) refsMoved() {
	t.mu.Lock()
	t.stale = true
	t.mu.Unlock()
}

// kick starts a read, unless one is running or nothing has moved since the last one.
func (t *trailerReader) kick() {
	t.mu.Lock()
	if t.closed || t.running || !t.stale {
		t.mu.Unlock()
		return
	}
	t.running, t.stale = true, false
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.wg.Add(1)
	t.mu.Unlock()
	go func() {
		defer t.wg.Done()
		// Commit messages only: in a partial clone nothing is downloaded behind it (#1010).
		err := t.index.Read(git.NoDownloads(ctx), t.repo, t.report)
		cancel()
		t.mu.Lock()
		t.running, t.cancel = false, nil
		// A read that failed is tried again the next time the graph settles. One that was
		// overtaken, by a fetch that landed while it ran, looks again now.
		overtaken := err == nil && t.stale
		if err != nil {
			t.stale = true
		}
		t.mu.Unlock()
		if overtaken {
			t.kick()
		}
	}()
}

// close stops a read that's under way, which keeps its place, and lets go of the index.
func (t *trailerReader) close() {
	t.mu.Lock()
	t.closed = true
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()
	t.wg.Wait()
	t.index.Close()
}
