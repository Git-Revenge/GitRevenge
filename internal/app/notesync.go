package app

import (
	"context"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/watch"
)

// Notice is the payload of the "notice" event: something worth a toast that isn't an
// error in the operation the user asked for.
type Notice struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// watchFailedMessage explains why a repository won't refresh on its own.
func watchFailedMessage(err error) string {
	const stale = "Git Revenge can't watch this repository, so changes made outside the app won't show up on their own."
	if watch.IsLimit(err) {
		return stale + " It needs more inotify watches than your system allows: raise fs.inotify.max_user_watches" +
			" and fs.inotify.max_user_instances, then open it again."
	}
	return stale
}

func (a *App) notice(path, message string) {
	a.emit("notice", Notice{Path: path, Message: message})
}

// syncNotesIn fetches notes from each remote and fast-forwards the local notes refs, when
// the repository's setting is on. Problems become a notice: the fetch or pull itself has
// already succeeded.
func (a *App) syncNotesIn(ctx context.Context, r *session.Repo, remotes []string, o git.NetOptions) {
	if !a.repoSettings(r.Path).SyncNotes {
		return
	}
	var diverged []string
	for _, rm := range remotes {
		if err := r.Git.FetchNotes(ctx, rm, o); err != nil {
			a.notice(r.Path, "Couldn't fetch notes from "+rm+": "+sentence(netErr(err).Error()))
			continue
		}
		d, err := r.Git.IntegrateNotes(ctx, rm)
		if err != nil {
			a.notice(r.Path, "Couldn't update notes from "+rm+": "+sentence(err.Error()))
		}
		for _, ref := range d {
			diverged = append(diverged, strings.TrimPrefix(ref, "refs/notes/")+" ("+rm+")")
		}
	}
	if len(diverged) > 0 {
		a.notice(r.Path, "Notes have diverged from the remote, so they were left as they are: "+strings.Join(diverged, ", ")+
			". Merge them with git notes merge, then push.")
	}
}

// pushNotesTo pushes notes refs to remote after a push, when the setting is on.
func (a *App) pushNotesTo(ctx context.Context, r *session.Repo, remote string, o git.NetOptions) {
	if !a.repoSettings(r.Path).SyncNotes {
		return
	}
	if err := r.Git.PushNotes(ctx, remote, o); err != nil {
		a.notice(r.Path, "The branch was pushed, but notes weren't: "+sentence(netErr(err).Error())+" Fetch to bring in the remote's notes first.")
	}
}

// remotesFor returns the remotes a fetch of remote ("" for all) covers.
func remotesFor(ctx context.Context, g *git.Repo, remote string) []string {
	if remote != "" {
		return []string{remote}
	}
	rs, err := g.Remotes(ctx)
	if err != nil {
		return nil
	}
	out := make([]string, len(rs))
	for i, rm := range rs {
		out[i] = rm.Name
	}
	return out
}

// pullRemote returns the remote a pull of the current branch fetches from: its upstream's,
// else the only remote, else origin.
func pullRemote(ctx context.Context, g *git.Repo) string {
	if st, err := g.Status(ctx); err == nil {
		if rm, _, ok := strings.Cut(st.Upstream, "/"); ok {
			return rm
		}
	}
	if rs, err := g.Remotes(ctx); err == nil && len(rs) == 1 {
		return rs[0].Name
	}
	return "origin"
}

// sentence trims msg and makes sure it ends with a full stop, for joining messages.
func sentence(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" || strings.HasSuffix(msg, ".") {
		return msg
	}
	return msg + "."
}
