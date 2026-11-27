package app

import (
	"context"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// describeCalls holds each repository's describe in flight, so a new one stops the last: on
// the kernel one takes about a second, and stepping down the graph would otherwise pile them
// up.
type describeCalls struct {
	mu     sync.Mutex
	cancel map[string]context.CancelFunc
}

// start cancels path's describe in flight, if any, and returns a context for the next, with
// its cancel to call when it's done. Cancelling one that has finished does nothing, so the
// map keeps each repository's latest without tidying.
func (d *describeCalls) start(path string) (context.Context, context.CancelFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cancel == nil {
		d.cancel = map[string]context.CancelFunc{}
	}
	if stop := d.cancel[path]; stop != nil {
		stop()
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel[path] = cancel
	return ctx, cancel
}

// DescribeCommit says which tag a commit comes after, as git describe names it, and the first
// tag that contains it (#940). The commit panel asks when its Details tab shows a commit, not
// for every row. A newer ask for the same repository stops this one, which then answers
// empty: the panel has moved on by then.
func (a *App) DescribeCommit(path, id string) (git.Described, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.Described{}, err
	}
	ctx, done := a.describing.start(path)
	defer done()
	d, err := r.Git.Describe(ctx, id)
	if ctx.Err() != nil {
		return git.Described{}, nil
	}
	return d, err
}
