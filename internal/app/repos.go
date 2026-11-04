package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/settings"
	"github.com/git-revenge/gitrevenge/internal/watch"
)

// RepoTab describes an open repository for the tab bar.
type RepoTab struct {
	Path string `json:"path"`
	Name string `json:"name"`
	// Head is the current branch, "detached at <sha>", or empty for a repo with no commits.
	Head string `json:"head"`
}

// SessionState is what the UI needs to rebuild its tabs on launch.
type SessionState struct {
	Tabs []RepoTab `json:"tabs"`
	// Active is a repository path, ConfigTabID for the Git Config tab, or "" for the start page.
	Active string       `json:"active"`
	Failed []FailedRepo `json:"failed"`
	// ConfigTab is the config file the Git Config tab had selected, or "" if it was closed.
	ConfigTab string `json:"configTab"`
	// ConfigTabID is Active's value for the Git Config tab.
	ConfigTabID string `json:"configTabId"`
	// InsightsTab is the Insights tab's scope from last time, or "" if it was closed.
	InsightsTab string `json:"insightsTab"`
	// InsightsTabID is Active's value for the Insights tab.
	InsightsTabID string `json:"insightsTabId"`
}

// FailedRepo is a saved tab that could not be reopened.
type FailedRepo struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// RecentRepo is an entry in the start page's recent list.
type RecentRepo struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	LastOpened time.Time `json:"lastOpened"`
	// Exists is false when the folder is gone; the UI greys it out.
	Exists bool `json:"exists"`
}

// sessions returns the session manager, creating it on first use once git is known.
func (a *App) sessions() (*session.Manager, error) {
	a.sessOnce.Do(func() {
		env := a.Environment()
		if env.Problem != "" {
			a.sessErr = errors.New(env.Problem)
			return
		}
		path := a.settingsPath
		if path == "" {
			if path, a.sessErr = settings.DefaultPath(); a.sessErr != nil {
				return
			}
		}
		store, err := settings.Open(path)
		if err != nil {
			a.sessErr = err
			return
		}
		a.sess = session.NewManager(git.NewRunner(env.GitPath), store)
		// GITREVENGE_READ_AHEAD tunes how far the graph reads ahead of the viewport.
		if n, err := strconv.Atoi(os.Getenv("GITREVENGE_READ_AHEAD")); err == nil {
			a.sess.SetReadAhead(n)
		}
		a.sess.OnGraph(a.emitGraphProgress)
		a.sess.OnWatchFailed(func(path string, err error) { a.notice(path, watchFailedMessage(err)) })
		a.sess.OnChange(func(path string, k watch.Kind) {
			a.emit("repo:changed", RepoChange{Path: path, Kinds: k.Names()})
		})
	})
	return a.sess, a.sessErr
}

// RepoChange is the payload of the "repo:changed" event.
type RepoChange struct {
	Path string `json:"path"`
	// Kinds lists what changed: head, refs, index, state, config, worktree.
	Kinds []string `json:"kinds"`
}

// emit sends an event to the frontend. It is a no-op until Wails has started (and in
// tests), because the Wails runtime aborts the process when called without its context.
func (a *App) emit(name string, data any) {
	if a.emitHook != nil {
		a.emitHook(name, data)
	}
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data)
}

// RepoInfo returns fresh tab details (such as the current branch) for an open repository.
func (a *App) RepoInfo(path string) (RepoTab, error) {
	m, err := a.sessions()
	if err != nil {
		return RepoTab{}, err
	}
	r := m.Get(path)
	if r == nil {
		return RepoTab{}, userError("That repository isn't open.")
	}
	return tabFor(context.Background(), r), nil
}

// RestoreSession reopens the tabs from the previous run.
func (a *App) RestoreSession() (SessionState, error) {
	m, err := a.sessions()
	if err != nil {
		return SessionState{}, err
	}
	ctx := context.Background()
	// Start watching global config now, so repositories refresh when it changes even if the
	// Git Config tab is never opened.
	_, _ = a.gitConfig()
	active, failed := m.Restore(ctx)
	st := SessionState{
		Active:        active,
		Tabs:          []RepoTab{},
		Failed:        []FailedRepo{},
		ConfigTab:     m.Settings().Get().ConfigTab,
		ConfigTabID:   settings.ConfigTabID,
		InsightsTab:   m.Settings().Get().InsightsTab,
		InsightsTabID: settings.InsightsTabID,
	}
	for _, r := range m.List() {
		st.Tabs = append(st.Tabs, tabFor(ctx, r))
	}
	for p, err := range failed {
		st.Failed = append(st.Failed, FailedRepo{Path: p, Error: err.Error()})
	}
	return st, nil
}

// ChooseAndOpenRepo shows a folder picker and opens the chosen repository. It returns nil
// if the user cancels.
func (a *App) ChooseAndOpenRepo() (*RepoTab, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Open repository"})
	if err != nil || dir == "" {
		return nil, err
	}
	tab, err := a.OpenRepo(dir)
	if err != nil {
		return nil, err
	}
	return &tab, nil
}

// OpenRepo opens the repository containing path as a tab (or focuses it if already open).
func (a *App) OpenRepo(path string) (RepoTab, error) {
	m, err := a.sessions()
	if err != nil {
		return RepoTab{}, err
	}
	ctx := context.Background()
	r, err := m.Open(ctx, path)
	if err != nil {
		return RepoTab{}, friendly(err)
	}
	return tabFor(ctx, r), nil
}

// CloseRepo closes the tab for path.
func (a *App) CloseRepo(path string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	a.undo.forget(path)
	return m.Close(path)
}

// SetActiveRepo records the selected tab; an empty path means the start page.
func (a *App) SetActiveRepo(path string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.SetActive(path)
}

// RecentRepos lists recently opened repositories, most recent first.
func (a *App) RecentRepos() ([]RecentRepo, error) {
	m, err := a.sessions()
	if err != nil {
		return nil, err
	}
	recent := m.Settings().Get().Recent
	out := make([]RecentRepo, 0, len(recent))
	for _, r := range recent {
		_, statErr := os.Stat(r.Path)
		out = append(out, RecentRepo{Path: r.Path, Name: filepath.Base(r.Path), LastOpened: r.LastOpened, Exists: statErr == nil})
	}
	return out, nil
}

// RemoveRecentRepo drops path from the recent list.
func (a *App) RemoveRecentRepo(path string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) { s.RemoveRecent(path) })
}

func tabFor(ctx context.Context, r *session.Repo) RepoTab {
	return RepoTab{Path: r.Path, Name: r.Name(), Head: head(ctx, r.Git)}
}

// head describes HEAD for display. Errors yield an empty string rather than failing the tab.
func head(ctx context.Context, r *git.Repo) string {
	if branch, err := r.ReadString(ctx, "symbolic-ref", "--short", "-q", "HEAD"); err == nil {
		return branch
	}
	if sha, err := r.ReadString(ctx, "rev-parse", "--short", "HEAD"); err == nil {
		return "detached at " + sha
	}
	return ""
}

// userError is an error whose message is written to be shown to the user as is.
type userError string

func (e userError) Error() string { return string(e) }

// friendly rewrites well-known errors into messages fit for a dialog.
func friendly(err error) error {
	switch {
	case errors.Is(err, session.ErrNotRepo):
		return userError("That folder isn't a git repository.")
	case errors.Is(err, session.ErrBare):
		return userError("Bare repositories aren't supported, because they have no working files to show.")
	}
	return err
}

// gitMessage turns git's stderr into a message for a dialog: "error: " / "fatal: " /
// "hint: " prefixes are dropped, since the UI already presents it as an error, and so are
// status lines such as clone's "Cloning into ..." and worktree's "Preparing worktree ...".
func gitMessage(stderr string) userError {
	var lines []string
	for l := range strings.SplitSeq(strings.TrimSpace(stderr), "\n") {
		if strings.HasPrefix(l, "Cloning into ") || strings.HasPrefix(l, "Preparing worktree") {
			continue
		}
		for _, p := range []string{"error: ", "fatal: ", "hint: "} {
			l = strings.TrimPrefix(l, p)
		}
		lines = append(lines, l)
	}
	return userError(strings.Join(lines, "\n"))
}

// ChooseFolder shows a folder picker and returns the chosen path, or "" if cancelled.
func (a *App) ChooseFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}
