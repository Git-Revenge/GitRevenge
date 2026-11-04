package app

import (
	"context"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// maxReflog caps how many reflog entries are listed; reflogs keep 90 days by default.
const maxReflog = 2000

// Reflog lists a ref's reflog, newest first: "HEAD" or a branch's full ref name.
func (a *App) Reflog(path, ref string) ([]git.ReflogEntry, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	return r.Git.Reflog(context.Background(), ref, maxReflog)
}
