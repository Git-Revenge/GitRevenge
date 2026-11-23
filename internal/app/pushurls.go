package app

import (
	"context"
	"slices"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// Pushing to several places (#926): a remote's push URLs, kept in git's own
// remote.<name>.pushurl, and what a push did at each.

// SetRemotePushURLs makes a remote push to urls, in order. Its own URL is one of them as
// any other; pushing only there leaves no pushurl in config.
func (a *App) SetRemotePushURLs(path, remote string, urls []string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return writeErr(r.Git.SetPushURLs(context.Background(), remote, urls))
}

// followURL rewrites push URLs that were a remote's old URL, after its URL changed: other
// remotes' that follow it, with nothing stored to say so, and the remote's own, which git
// writes among its push URLs once it has extras and `git remote set-url` leaves alone.
func followURL(ctx context.Context, g *git.Repo, oldURL, newURL string) {
	remotes, err := g.Remotes(ctx)
	if err != nil {
		return
	}
	for _, rm := range remotes {
		if len(rm.PushURLs) < 2 {
			continue
		}
		changed := false
		urls := slices.Clone(rm.PushURLs)
		for i, u := range urls {
			if git.SameRemoteURL(u, oldURL) {
				urls[i], changed = newURL, true
			}
		}
		if changed {
			_ = g.SetPushURLs(ctx, rm.Name, urls)
		}
	}
}

// PushResult is the payload of the "push:result" event: what a push to a remote with several
// push URLs did at each, so a partial push is never silent.
type PushResult struct {
	Path    string     `json:"path"`
	Remote  string     `json:"remote"`
	Refspec string     `json:"refspec"`
	Dests   []PushDest `json:"dests"`
}

// PushDest is one push URL's outcome, with the name the result shows for it.
type PushDest struct {
	git.PushDest
	// Name is the host, or the path of a folder on this machine.
	Name string `json:"name"`
}

func named(ds []git.PushDest) []PushDest {
	out := make([]PushDest, len(ds))
	for i, d := range ds {
		name := git.ParseLocation(d.URL).Host
		if name == "" {
			name = d.URL
		}
		out[i] = PushDest{PushDest: d, Name: name}
	}
	return out
}

// pushReported pushes, and for a remote with several push URLs reports each one's outcome.
// Then it's an error only when nothing reached anywhere: the result says what didn't.
func (a *App) pushReported(ctx context.Context, r *session.Repo, p git.PushOptions, o git.NetOptions) (multi bool, err error) {
	dests, err := r.Git.PushWithReport(ctx, p, o)
	if len(dests) < 2 {
		return false, err
	}
	a.emit("push:result", PushResult{Path: r.Path, Remote: p.Remote, Refspec: p.Refspec, Dests: named(dests)})
	for _, d := range dests {
		if d.State == git.PushPushed || d.State == git.PushUpToDate {
			return true, nil
		}
	}
	return true, err
}

// PushToURLs pushes one refspec to some of a remote's push URLs, one at a time, carrying on
// past any that fail: for the URLs a push didn't reach, or a retry. Only the remote's own push
// URLs are accepted.
func (a *App) PushToURLs(path, remote, refspec string, urls []string) error {
	return a.runNetwork(path, "push", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		remotes, err := r.Git.Remotes(ctx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(remotes, func(x git.Remote) bool { return x.Name == remote })
		if i < 0 {
			return userError("There's no remote called " + remote + ".")
		}
		var dests []git.PushDest
		for _, u := range urls {
			if !slices.Contains(remotes[i].PushURLs, u) {
				return userError(u + " isn't one of " + remote + "'s push URLs.")
			}
			dests = append(dests, r.Git.PushToURL(ctx, u, refspec, o))
		}
		a.emit("push:result", PushResult{Path: r.Path, Remote: remote, Refspec: refspec, Dests: named(dests)})
		return nil
	})
}
