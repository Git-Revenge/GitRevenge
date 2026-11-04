package app

import (
	"context"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// sigCache remembers signature checks per repository: verifying runs gpg, ssh-keygen or
// gpgsm once per signed commit, and the graph asks for each screen it shows.
type sigCache struct {
	mu   sync.Mutex
	byRe map[string]map[string]git.SignatureCheck
}

// maxSignatureBatch caps one request; the graph asks for about a screen at a time.
const maxSignatureBatch = 500

// CommitSignatures verifies the signatures of the given commits, using git's own
// verification and the user's keys and allowed signers. Results are cached for the session.
func (a *App) CommitSignatures(path string, ids []string) ([]git.SignatureCheck, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	if len(ids) > maxSignatureBatch {
		ids = ids[:maxSignatureBatch]
	}
	c := &a.sigs
	c.mu.Lock()
	if c.byRe == nil {
		c.byRe = map[string]map[string]git.SignatureCheck{}
	}
	known := c.byRe[path]
	if known == nil {
		known = map[string]git.SignatureCheck{}
		c.byRe[path] = known
	}
	out := make([]git.SignatureCheck, 0, len(ids))
	var missing []string
	for _, id := range ids {
		if s, ok := known[id]; ok {
			out = append(out, s)
		} else {
			missing = append(missing, id)
		}
	}
	c.mu.Unlock()

	checked, err := r.Git.VerifySignatures(context.Background(), missing)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, s := range checked {
		known[id] = s
		out = append(out, s)
	}
	return out, nil
}
