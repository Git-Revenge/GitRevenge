package app

import (
	"github.com/git-revenge/gitrevenge/internal/git"
)

// DescribeCommit says which tag a commit comes after, as git describe names it, and the first
// tag that contains it (#940). The commit panel asks when its Details tab shows a commit, not
// for every row. A newer ask for the same repository stops this one, which then answers
// empty: the panel has moved on by then. On the kernel one takes about a second, and stepping
// down the graph would otherwise pile them up.
func (a *App) DescribeCommit(path, id string) (git.Described, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.Described{}, err
	}
	ctx, done := a.latest.start("describe", r.Path)
	defer done()
	d, err := r.Git.Describe(ctx, id)
	if ctx.Err() != nil {
		return git.Described{}, nil
	}
	return d, err
}
