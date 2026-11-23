package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// Tidy (#924): which branches of each remote are dead, and tidying them away.
//
//   - gone: a local branch whose upstream the remote deleted, or a remote-tracking branch the
//     remote didn't send in its last fetch (left behind when pruning is off).
//   - merged: a remote branch whose every commit is on the remote's default branch.
//   - quiet: a remote branch with no commit for longer than the repository's window. There's
//     no window until one is chosen.
//
// Nothing is ticked that deletes for other people: only gone branches start ticked.

// Tidy kinds.
const (
	TidyGone   = "gone"
	TidyMerged = "merged"
	TidyQuiet  = "quiet"
)

// TidyItem is one branch Tidy could delete.
type TidyItem struct {
	Kind string `json:"kind"`
	// Name is the branch: "feature" for a local branch, "origin/feature" for a remote one.
	Name string `json:"name"`
	// Local is a local branch whose upstream is gone; Stale a remote-tracking branch the
	// remote no longer has. Either is deleted here only, never on the remote.
	Local bool `json:"local,omitempty"`
	Stale bool `json:"stale,omitempty"`
	// Unpushed counts a local branch's commits on no remote.
	Unpushed int `json:"unpushed,omitempty"`
	// OnlyHere counts a quiet branch's commits no other branch or tag holds: they'd live
	// only in the reflog once it's deleted. Only RemoteTidy counts them.
	OnlyHere   int    `json:"onlyHere,omitempty"`
	LastAuthor string `json:"lastAuthor,omitempty"`
	LastDate   int64  `json:"lastDate,omitempty"`
	// Ticked says it starts ticked: only gone branches with nothing to lose do.
	Ticked bool `json:"ticked"`
}

// TidyReport is what one remote could have tidied.
type TidyReport struct {
	Remote string `json:"remote"`
	// Base is the remote's default branch, or "" when git doesn't know it (then nothing
	// counts as merged).
	Base string `json:"base"`
	// QuietDays is the repository's quiet window, 0 when none is chosen.
	QuietDays int        `json:"quietDays"`
	Items     []TidyItem `json:"items"`
}

// tidyCache keeps each repository's last scan, keyed by what it was read from: the remote
// branches' tips, the local branches' upstream state, FETCH_HEAD and the window.
var tidyCache = struct {
	sync.Mutex
	by map[string]tidyScanned
}{by: map[string]tidyScanned{}}

type tidyScanned struct {
	key     string
	reports map[string]*TidyReport
}

// quietDays returns the repository's quiet window, 0 for none.
func (a *App) quietDays(path string) int {
	m, err := a.sessions()
	if err != nil {
		return 0
	}
	return m.Settings().Get().Repo(a.repoKey(path)).QuietDays
}

// SetQuietDays sets how long a remote branch goes without a commit before Tidy calls it
// quiet, or 0 to call none quiet.
func (a *App) SetQuietDays(path string, days int) error {
	if days < 0 || days > 3650 {
		return userError("Choose a window between none and ten years.")
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return a.updateRepoSettings(m, a.repoKey(path), func(rs *settings.RepoSettings) { rs.QuietDays = days })
}

// tidyScan reads every remote's tidy report, from the cache when nothing it reads changed.
func (a *App) tidyScan(ctx context.Context, r *session.Repo, now time.Time) (map[string]*TidyReport, error) {
	remotes, err := r.Git.Remotes(ctx)
	if err != nil {
		return nil, err
	}
	tips := r.Git.RemoteBranchTips(ctx)
	heads := r.Git.RemoteHeads(ctx)
	days := a.quietDays(r.Path)
	gone := r.Git.GoneBranches(ctx)

	var key strings.Builder
	fmt.Fprintf(&key, "%d|%d|", days, now.Unix()/3600)
	for _, t := range tips {
		key.WriteString(t.Short + "=" + t.Commit + ";")
	}
	for _, g := range gone {
		fmt.Fprintf(&key, "gone:%s=%s:%v;", g.Name, g.Commit, g.Busy)
	}
	for _, rm := range remotes {
		names, ok := r.Git.LastFetchedBranches(ctx, rm.FetchURL)
		fmt.Fprintf(&key, "fh:%s:%v:%s;", rm.Name, ok, strings.Join(names, ","))
	}
	tidyCache.Lock()
	if c, ok := tidyCache.by[r.Path]; ok && c.key == key.String() {
		tidyCache.Unlock()
		return c.reports, nil
	}
	tidyCache.Unlock()

	names := make([]string, len(remotes))
	for i, rm := range remotes {
		names[i] = rm.Name
	}
	remoteOf := func(short string) string {
		best := ""
		for _, n := range names {
			if len(n) > len(best) && strings.HasPrefix(short, n+"/") {
				best = n
			}
		}
		return best
	}

	reports := map[string]*TidyReport{}
	for _, rm := range remotes {
		rep := &TidyReport{Remote: rm.Name, Base: heads[rm.Name], QuietDays: days, Items: []TidyItem{}}
		reports[rm.Name] = rep
		seen := map[string]bool{}
		if rep.Base != "" {
			seen[rm.Name+"/"+rep.Base] = true
		}

		// Gone: local branches whose upstream the remote deleted.
		for _, g := range gone {
			if remoteOf(g.Upstream) != rm.Name {
				continue
			}
			unpushed := r.Git.Unpushed(ctx, g.Name)
			rep.Items = append(rep.Items, TidyItem{Kind: TidyGone, Name: g.Name, Local: true, Unpushed: unpushed, Ticked: unpushed == 0 && !g.Busy})
		}
		// Gone: remote-tracking branches the remote didn't send last time.
		if sent, ok := r.Git.LastFetchedBranches(ctx, rm.FetchURL); ok {
			for _, t := range tips {
				if remoteOf(t.Short) != rm.Name || seen[t.Short] {
					continue
				}
				if !slices.Contains(sent, strings.TrimPrefix(t.Short, rm.Name+"/")) {
					seen[t.Short] = true
					rep.Items = append(rep.Items, TidyItem{Kind: TidyGone, Name: t.Short, Stale: true, LastAuthor: t.Author, LastDate: t.Date, Ticked: true})
				}
			}
		}
		// Merged into the default branch.
		if rep.Base != "" {
			for _, short := range r.Git.MergedRemoteBranches(ctx, rm.Name, "refs/remotes/"+rm.Name+"/"+rep.Base) {
				if seen[short] {
					continue
				}
				seen[short] = true
				item := TidyItem{Kind: TidyMerged, Name: short}
				if i := slices.IndexFunc(tips, func(t git.RemoteBranchTip) bool { return t.Short == short }); i >= 0 {
					item.LastAuthor, item.LastDate = tips[i].Author, tips[i].Date
				}
				rep.Items = append(rep.Items, item)
			}
		}
		// Quiet, once there's a window.
		if days > 0 {
			cutoff := now.Add(-time.Duration(days) * 24 * time.Hour).Unix()
			for _, t := range tips {
				if remoteOf(t.Short) != rm.Name || seen[t.Short] || t.Date >= cutoff {
					continue
				}
				seen[t.Short] = true
				rep.Items = append(rep.Items, TidyItem{Kind: TidyQuiet, Name: t.Short, LastAuthor: t.Author, LastDate: t.Date})
			}
		}
	}
	tidyCache.Lock()
	tidyCache.by[r.Path] = tidyScanned{key: key.String(), reports: reports}
	tidyCache.Unlock()
	return reports, nil
}

// TidyMarks says which branches could be tidied, by name ("feature" for a local branch,
// "origin/feature" for a remote one), with their kind: for the marks on the Remotes panel's
// rows and the count at each remote's foot.
func (a *App) TidyMarks(path string) (map[string]string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	reports, err := a.tidyScan(context.Background(), r, time.Now())
	if err != nil {
		return nil, err
	}
	marks := map[string]string{}
	for _, rep := range reports {
		for _, it := range rep.Items {
			marks[it.Name] = it.Kind
		}
	}
	return marks, nil
}

// RemoteTidy returns what one remote could have tidied, for the Tidy dialog, with how many
// commits each quiet branch alone holds.
func (a *App) RemoteTidy(path, remote string) (TidyReport, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TidyReport{}, err
	}
	ctx := context.Background()
	reports, err := a.tidyScan(ctx, r, time.Now())
	if err != nil {
		return TidyReport{}, err
	}
	rep, ok := reports[remote]
	if !ok {
		return TidyReport{}, userError("There's no remote called " + remote + ".")
	}
	out := *rep
	out.Items = slices.Clone(rep.Items)
	for i := range out.Items {
		if out.Items[i].Kind == TidyQuiet {
			out.Items[i].OnlyHere = r.Git.OnlyOn(ctx, out.Items[i].Name)
		}
	}
	return out, nil
}

// TidyResult is what happened to one branch.
type TidyResult struct {
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
}

// TidyHere deletes local leftovers: local branches whose upstream is gone (locals) and
// remote-tracking branches the remote no longer has (stale). Nothing on any remote changes;
// deleting a remote's branch is a push, and goes through PushRef. A local branch that's
// checked out, here or in a worktree, is refused rather than deleted.
func (a *App) TidyHere(path string, locals, stale []string) ([]TidyResult, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	out := []TidyResult{}
	for _, name := range locals {
		res := TidyResult{Name: name}
		if err := git.CheckRevs(name); err != nil {
			res.Error = err.Error()
		} else if _, err := r.Git.Write(ctx, git.Cmd{Args: []string{"branch", "-D", name}}); err != nil {
			res.Error = gitSays(err)
		}
		out = append(out, res)
	}
	for _, name := range stale {
		res := TidyResult{Name: name}
		if err := git.CheckRevs(name); err != nil {
			res.Error = err.Error()
		} else if _, err := r.Git.Write(ctx, git.Cmd{Args: []string{"branch", "-r", "-d", name}}); err != nil {
			res.Error = gitSays(err)
		}
		out = append(out, res)
	}
	r.ReloadGraph()
	return out, nil
}

// gitSays is git's own explanation of a failure, its first line, or the error itself.
func gitSays(err error) string {
	var gerr *git.Error
	if errors.As(err, &gerr) && strings.TrimSpace(gerr.Stderr) != "" {
		line, _, _ := strings.Cut(strings.TrimSpace(gerr.Stderr), "\n")
		return strings.TrimPrefix(strings.TrimPrefix(line, "error: "), "fatal: ")
	}
	return err.Error()
}
