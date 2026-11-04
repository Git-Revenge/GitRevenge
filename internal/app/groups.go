package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// groupConcurrency bounds how many repositories a dashboard or batch touches at once.
const groupConcurrency = 4

// Groups returns the project groups.
func (a *App) Groups() ([]settings.Group, error) {
	m, err := a.sessions()
	if err != nil {
		return nil, err
	}
	return m.Settings().Get().Groups, nil
}

// GroupAction edits groups: create (name), rename (from, to), delete (name),
// add (name, repo path), remove (name, repo path), move (name, index),
// move-repo (name, repo path, index).
func (a *App) GroupAction(action string, args []string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	arg := func(i int) string {
		if i < len(args) {
			return strings.TrimSpace(args[i])
		}
		return ""
	}
	if action == "add" {
		// Store the repository root, whatever folder inside it was chosen.
		root, err := m.Resolve(context.Background(), arg(1))
		if err != nil {
			return friendly(err)
		}
		args = []string{arg(0), root}
	}
	var problem string
	err = m.Settings().Update(func(s *settings.Settings) {
		switch action {
		case "create":
			switch {
			case arg(0) == "":
				problem = "Give the group a name."
			case s.GroupIndex(arg(0)) >= 0:
				problem = "A group named " + arg(0) + " already exists."
			default:
				s.Groups = append(s.Groups, settings.Group{Name: arg(0), Paths: []string{}})
			}
		case "rename":
			i := s.GroupIndex(arg(0))
			switch {
			case i < 0:
				problem = "That group no longer exists."
			case arg(1) == "":
				problem = "Give the group a name."
			case arg(1) != arg(0) && s.GroupIndex(arg(1)) >= 0:
				problem = "A group named " + arg(1) + " already exists."
			default:
				s.Groups[i].Name = arg(1)
			}
		case "delete":
			if i := s.GroupIndex(arg(0)); i >= 0 {
				s.Groups = append(s.Groups[:i], s.Groups[i+1:]...)
			}
		case "add":
			if !s.AddToGroup(arg(0), args[1]) {
				problem = "That group no longer exists."
			}
		case "remove":
			s.RemoveFromGroup(arg(0), arg(1))
		case "move":
			to, convErr := strconv.Atoi(arg(1))
			if convErr != nil || !s.MoveGroup(arg(0), to) {
				problem = "That group can't move there. The list may have changed."
			}
		case "move-repo":
			to, convErr := strconv.Atoi(arg(2))
			if convErr != nil || !s.MoveInGroup(arg(0), arg(1), to) {
				problem = "That repository can't move there. The list may have changed."
			}
		default:
			problem = "Unknown group action " + action + "."
		}
	})
	if problem != "" {
		return userError(problem)
	}
	return err
}

// GroupRepo is one row of a group dashboard.
type GroupRepo struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Exists    bool   `json:"exists"`
	Branch    string `json:"branch"`
	Upstream  string `json:"upstream"`
	Ahead     int    `json:"ahead"`
	Behind    int    `json:"behind"`
	Changes   int    `json:"changes"`
	Conflicts int    `json:"conflicts"`
	// LastFetch is when the repo was last fetched (Unix seconds), or 0 if never.
	LastFetch int64  `json:"lastFetch"`
	Error     string `json:"error"`
}

// GroupDashboard reports the state of every repository in a group.
func (a *App) GroupDashboard(name string) ([]GroupRepo, error) {
	m, paths, err := a.groupPaths(name)
	if err != nil {
		return nil, err
	}
	rows := make([]GroupRepo, len(paths))
	forEach(paths, func(i int, p string) {
		rows[i] = repoRow(m, p)
	})
	return rows, nil
}

func repoRow(m *session.Manager, p string) GroupRepo {
	row := GroupRepo{Path: p, Name: filepath.Base(p)}
	if _, err := os.Stat(p); err != nil {
		row.Error = "Folder not found."
		return row
	}
	row.Exists = true
	ctx := context.Background()
	g, _, err := m.GitFor(ctx, p)
	if err != nil {
		row.Error = friendly(err).Error()
		return row
	}
	st, err := g.Status(ctx)
	if err != nil {
		row.Error = err.Error()
		return row
	}
	row.Branch, row.Upstream, row.Ahead, row.Behind = st.Head, st.Upstream, st.Ahead, st.Behind
	if row.Branch == "" && st.Oid != "" {
		row.Branch = "detached at " + st.Oid[:7]
	}
	for _, f := range st.Files {
		row.Changes++
		if f.Conflicted {
			row.Conflicts++
		}
	}
	if fh, err := g.ReadString(ctx, "rev-parse", "--git-path", "FETCH_HEAD"); err == nil {
		if !filepath.IsAbs(fh) {
			fh = filepath.Join(p, fh)
		}
		if info, err := os.Stat(fh); err == nil {
			row.LastFetch = info.ModTime().Unix()
		}
	}
	return row
}

// BatchResult is the outcome of a batch operation on one repository.
type BatchResult struct {
	Path    string `json:"path"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	// Skipped means there was nothing to do (no remotes, or no upstream to pull); not a failure.
	Skipped bool `json:"skipped"`
}

// GroupBatch fetches ("fetch") or fast-forward pulls ("pull") every repository in a group,
// a few at a time. One repository failing doesn't stop the others; each gets a result, and
// a "group:progress" event is sent as each finishes.
func (a *App) GroupBatch(name, op string) ([]BatchResult, error) {
	if op != "fetch" && op != "pull" {
		return nil, userError("Unknown batch operation " + op + ".")
	}
	m, paths, err := a.groupPaths(name)
	if err != nil {
		return nil, err
	}
	env, err := a.askEnv()
	if err != nil {
		return nil, err
	}
	results := make([]BatchResult, len(paths))
	forEach(paths, func(i int, p string) {
		res := BatchResult{Path: p}
		ctx := context.Background()
		g, _, err := m.GitFor(ctx, p)
		if err == nil {
			res.Message = skipReason(ctx, g, op)
			res.Skipped = res.Message != ""
		}
		if err == nil && !res.Skipped {
			o := git.NetOptions{Env: env}
			a.upstreams.remember(ctx, p, g)
			if op == "fetch" {
				err = g.Fetch(ctx, "", o)
			} else {
				// Only fast-forwards: a batch must never create merges or rebase silently.
				err = g.Pull(ctx, git.PullFFOnly, o)
			}
		}
		res.OK = err == nil
		if err != nil {
			res.Message = batchMessage(err)
		}
		results[i] = res
		a.emit("group:progress", struct {
			Group string `json:"group"`
			BatchResult
		}{name, res})
	})
	return results, nil
}

// skipReason explains why op has nothing to do in a repository, or returns "".
func skipReason(ctx context.Context, g *git.Repo, op string) string {
	if remotes, err := g.Remotes(ctx); err == nil && len(remotes) == 0 {
		return "No remotes."
	}
	if op == "pull" {
		if st, err := g.Status(ctx); err == nil && st.Upstream == "" {
			return "No upstream for the current branch."
		}
	}
	return ""
}

func batchMessage(err error) string {
	var gerr *git.Error
	if errors.As(err, &gerr) {
		s := gerr.Stderr
		switch {
		case strings.Contains(s, "Not possible to fast-forward"), strings.Contains(s, "Diverging branches"):
			return "Can't fast-forward: the branch has diverged from its upstream. Pull it on its own."
		case strings.Contains(s, "would be overwritten"):
			return "Local changes would be overwritten. Commit or stash them first."
		}
	}
	return netErr(err).Error()
}

func (a *App) groupPaths(name string) (*session.Manager, []string, error) {
	m, err := a.sessions()
	if err != nil {
		return nil, nil, err
	}
	s := m.Settings().Get()
	i := s.GroupIndex(name)
	if i < 0 {
		return nil, nil, userError("That group no longer exists.")
	}
	return m, s.Groups[i].Paths, nil
}

// forEach runs fn for each path with bounded concurrency.
func forEach(paths []string, fn func(i int, p string)) {
	sem := make(chan struct{}, groupConcurrency)
	var wg sync.WaitGroup
	for i, p := range paths {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			fn(i, p)
		})
	}
	wg.Wait()
}
