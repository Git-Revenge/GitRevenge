package app

import (
	"context"
	"strings"

	"github.com/risadams/gitrevenge/internal/git"
)

// maxListedCommits is how many commits a warning names.
const maxListedCommits = 20

// SharedCommits is what an operation would affect that's on a branch's upstream.
type SharedCommits struct {
	Upstream string           `json:"upstream"`
	Commits  []git.CommitLine `json:"commits"`
	// Total counts them all, which can be more than Commits lists.
	Total int `json:"total"`
}

// PushedUndo says which commits the next Undo, or Redo, would take off the current branch that
// are already on its upstream. They'd stay there, so the branch and the remote would part ways.
// It's empty when that isn't so, including when the step would be refused anyway.
func (a *App) PushedUndo(path string, undo bool) (SharedCommits, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return SharedCommits{}, err
	}
	a.undo.mu.Lock()
	stack := a.undo.redo[r.Path]
	if undo {
		stack = a.undo.undo[r.Path]
	}
	var e undoEntry
	found := len(stack) > 0
	if found {
		e = stack[len(stack)-1]
	}
	a.undo.mu.Unlock()
	if !found || !movesPushedWork(e) {
		return SharedCommits{}, nil
	}
	expect, target := e.After, e.Before
	if !undo {
		expect, target = e.Before, e.After
	}
	ctx := context.Background()
	if expect.Branch == "" || expect.Branch != target.Branch || r.Git.Head(ctx) != expect {
		return SharedCommits{}, nil
	}
	upstream := r.Git.UpstreamOf(ctx, expect.Branch)
	if upstream == "" {
		return SharedCommits{}, nil
	}
	// What's on both the branch and its upstream ends at where they meet.
	out, err := r.Git.ReadString(ctx, "merge-base", expect.Commit, upstream)
	if err != nil {
		return SharedCommits{}, nil
	}
	commits, total, err := r.Git.CommitsBetween(ctx, target.Commit, strings.TrimSpace(out), maxListedCommits)
	if err != nil || total == 0 {
		return SharedCommits{}, nil
	}
	return SharedCommits{Upstream: upstream, Commits: commits, Total: total}, nil
}

// movesPushedWork says whether e can take commits off a branch that others might have. Undoing
// a pull doesn't count: it only puts the branch behind, and pulling again fixes that.
func movesPushedWork(e undoEntry) bool {
	switch e.Kind {
	case kindCommit, kindReset:
		return true
	case kindMove:
		return e.Label != "Pull"
	default:
		return false
	}
}

// ForcePushLoss says which commits on the current branch's upstream a force push would replace,
// as far as the last fetch knows.
func (a *App) ForcePushLoss(path string) (SharedCommits, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return SharedCommits{}, err
	}
	ctx := context.Background()
	head := r.Git.Head(ctx)
	if head.Branch == "" {
		return SharedCommits{}, nil
	}
	upstream := r.Git.UpstreamOf(ctx, head.Branch)
	if upstream == "" {
		return SharedCommits{}, nil
	}
	commits, total, err := r.Git.CommitsBetween(ctx, head.Commit, upstream, maxListedCommits)
	if err != nil || total == 0 {
		return SharedCommits{}, nil
	}
	return SharedCommits{Upstream: upstream, Commits: commits, Total: total}, nil
}

// pushRefusal explains a rejected push when the remote's commits are ones that were on this
// branch here and have left it: undone or rewritten, rather than pushed by someone else.
// Otherwise it's err. An upstream still in the branch's history doesn't count, however much
// the reflog remembers it: that's the remote having moved on since the last fetch.
func pushRefusal(ctx context.Context, g *git.Repo, branch, upstream string, err error) error {
	if upstream == "" || !isRejectedPush(err) {
		return err
	}
	tip, tipErr := g.ReadString(ctx, "rev-parse", upstream)
	if tipErr != nil {
		return err
	}
	tip = strings.TrimSpace(tip)
	if _, stillHere := g.ReadString(ctx, "merge-base", "--is-ancestor", tip, "refs/heads/"+branch); stillHere == nil || !g.WasOnBranch(ctx, branch, tip) {
		return err
	}
	return userError("The remote still has commits you undid or rewrote here, so it refused the push. To take them away for everyone, put them back and use Revert, which adds a commit undoing them. To replace them instead, force push (with lease).")
}
