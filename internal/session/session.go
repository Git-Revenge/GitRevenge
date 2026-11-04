// Package session tracks the repositories open as tabs and keeps settings in sync with them.
package session

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/fsutil"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
	"github.com/git-revenge/gitrevenge/internal/settings"
	"github.com/git-revenge/gitrevenge/internal/watch"
)

// ErrNotRepo means the chosen folder is not inside a git work tree.
var ErrNotRepo = errors.New("not a git repository")

// ErrBare means the chosen folder is a bare repository, which has no work tree to show.
var ErrBare = errors.New("bare repositories are not supported")

// Repo is one open repository with its warm backend state.
type Repo struct {
	// Path is the absolute work tree root; it is also the repo's ID.
	Path string
	// GitDir is the absolute git directory (usually Path/.git).
	GitDir  string
	Git     *git.Repo
	Objects *git.ObjectReader
	watcher *watch.Watcher
	graph   *graphLoader
	changes changeGate
}

// changeGate holds back reports of changes while an operation runs.
type changeGate struct {
	mu      sync.Mutex
	holding int
	held    watch.Kind
	deliver func(watch.Kind)
}

// report passes k on, or keeps it for when the gate opens.
func (g *changeGate) report(k watch.Kind) {
	g.mu.Lock()
	if g.holding > 0 {
		g.held |= k
		g.mu.Unlock()
		return
	}
	deliver := g.deliver
	g.mu.Unlock()
	if deliver != nil {
		deliver(k)
	}
}

// Quietly runs op without reloading or refreshing for what it changes until it's done, then
// does so once. A multi-step git operation changes refs many times, and each change starts
// reads of the same files git is renaming, which on Windows makes git wait and try again.
func (r *Repo) Quietly(op func()) {
	g := &r.changes
	g.mu.Lock()
	g.holding++
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.holding--
		var held watch.Kind
		if g.holding == 0 {
			held, g.held = g.held, 0
		}
		deliver := g.deliver
		g.mu.Unlock()
		if held != 0 && deliver != nil {
			deliver(held)
		}
	}()
	op()
}

// Graph returns the commit graph to display and whether a (re)load is in progress.
func (r *Repo) Graph() (*graph.Graph, bool) { return r.graph.Graph() }

// Refs returns the refs matching the displayed graph.
func (r *Repo) Refs() *RefIndex { return r.graph.Refs() }

// GraphStatus reports whether the graph reader is paused ahead of the viewport, and the
// estimated size of the whole history (0 until known).
func (r *Repo) GraphStatus() (paused bool, estimate int) { return r.graph.Status() }

// DemandRows records that the UI wants graph rows up to row, so a paused reader reads on.
func (r *Repo) DemandRows(row int) { r.graph.Demand(row) }

// FindRow returns the graph row of the commit id, reading on through the history until it
// appears. It returns false if the commit isn't in the history.
func (r *Repo) FindRow(ctx context.Context, id string) (int, bool, error) {
	g, err := r.graph.LoadUntil(ctx, func(g *graph.Graph) bool {
		_, ok := g.RowOf(id)
		return ok
	})
	if err != nil || g == nil {
		return -1, false, err
	}
	row, ok := g.RowOf(id)
	return row, ok, nil
}

// FullGraph returns the graph once the whole history has loaded, for searching it.
func (r *Repo) FullGraph(ctx context.Context) (*graph.Graph, error) {
	return r.graph.LoadUntil(ctx, func(*graph.Graph) bool { return false })
}

// Name is the folder name shown on the tab.
func (r *Repo) Name() string { return filepath.Base(r.Path) }

// ReloadGraph reloads the commit graph, for a change git can't see, such as new hide
// patterns.
func (r *Repo) ReloadGraph() { r.graph.reload() }

// SetShowHidden shows the repository's hidden branches again (or hides them), for this
// session only, and reloads the graph.
func (r *Repo) SetShowHidden(show bool) { r.graph.SetShowHidden(show) }

// ShowingHidden reports whether hidden branches are being shown.
func (r *Repo) ShowingHidden() bool { return r.graph.ShowingHidden() }

// SetFocus narrows the graph to one branch or stack, or back to everything with the zero
// Focus, for this session only, and reloads it.
func (r *Repo) SetFocus(focus Focus) { r.graph.SetFocus(focus) }

// Focus returns what the graph is focused on.
func (r *Repo) Focus() Focus { return r.graph.Focus() }

// Rules returns the branch rules (kinds and hide patterns) in a repository's settings.
func Rules(rs settings.RepoSettings) branchkind.Rules {
	overrides := map[branchkind.Kind][]string{}
	for k, p := range rs.Kinds {
		overrides[branchkind.Kind(k)] = p
	}
	var hidden []branchkind.Kind
	for _, k := range rs.HideKinds {
		hidden = append(hidden, branchkind.Kind(k))
	}
	return branchkind.New(overrides, rs.Hide).HidingKinds(hidden)
}

func (r *Repo) close() {
	if r.watcher != nil {
		r.watcher.Close()
	}
	r.graph.stop()
	_ = r.Objects.Close()
}

// Manager owns the open repositories. It is safe for concurrent use.
type Manager struct {
	runner   *git.Runner
	settings *settings.Store
	now      func() time.Time

	mu            sync.Mutex
	repos         []*Repo // tab order
	onChange      func(path string, k watch.Kind)
	onWatchFailed func(path string, err error)
	// startWatch is watch.New, except in tests.
	startWatch func(watch.Options) (*watch.Watcher, error)
	onGraph    func(path string, p GraphProgress)
	// readAhead overrides DefaultReadAhead; tests use it.
	readAhead int
}

// OnGraph registers fn to receive commit graph loading progress for open repositories.
// Call it before opening repositories.
func (m *Manager) OnGraph(fn func(path string, p GraphProgress)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onGraph = fn
}

// OnChange registers fn to be called (from a background goroutine) when an open
// repository changes on disk. Call it before opening repositories.
func (m *Manager) OnChange(fn func(path string, k watch.Kind)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// OnWatchFailed registers fn to be called when an open repository can't be watched, so it
// won't refresh on its own. Call it before opening repositories.
func (m *Manager) OnWatchFailed(fn func(path string, err error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onWatchFailed = fn
}

// NewManager returns a Manager with no open repositories.
func NewManager(runner *git.Runner, store *settings.Store) *Manager {
	return &Manager{runner: runner, settings: store, now: time.Now, startWatch: watch.New}
}

// SetReadAhead sets how many rows the graph reader loads past the furthest row requested
// before pausing, for repositories opened after the call. n <= 0 restores the default.
func (m *Manager) SetReadAhead(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readAhead = n
}

// Resolve returns the work tree root containing path.
func (m *Manager) Resolve(ctx context.Context, path string) (string, error) {
	root, _, err := m.resolve(ctx, path)
	return root, err
}

// resolve returns the work tree root and git directory containing path.
func (m *Manager) resolve(ctx context.Context, path string) (root, gitDir string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("%s: folder not found", abs)
	}
	// --show-toplevel fails outside a work tree, so classify the folder first.
	out, err := m.runner.Output(ctx, git.Cmd{
		Dir:      abs,
		Args:     []string{"rev-parse", "--is-bare-repository", "--is-inside-work-tree"},
		ReadOnly: true,
	})
	if err != nil {
		var gerr *git.Error
		if errors.As(err, &gerr) && strings.Contains(gerr.Stderr, "not a git repository") {
			return "", "", fmt.Errorf("%s: %w", abs, ErrNotRepo)
		}
		return "", "", err
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return "", "", fmt.Errorf("unexpected rev-parse output %q", out)
	}
	switch f[0] + " " + f[1] {
	case "true false":
		return "", "", fmt.Errorf("%s: %w", abs, ErrBare)
	case "false false":
		// Inside a .git directory.
		return "", "", fmt.Errorf("%s: %w", abs, ErrNotRepo)
	}
	out, err = m.runner.Output(ctx, git.Cmd{
		Dir:      abs,
		Args:     []string{"rev-parse", "--show-toplevel", "--absolute-git-dir"},
		ReadOnly: true,
	})
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		return "", "", fmt.Errorf("unexpected rev-parse output %q", out)
	}
	return filepath.Clean(filepath.FromSlash(lines[0])), filepath.Clean(filepath.FromSlash(lines[1])), nil
}

// Open opens the repository containing path as a tab, or returns it if already open, and
// records it as most recently used and active.
func (m *Manager) Open(ctx context.Context, path string) (*Repo, error) {
	root, gitDir, err := m.resolve(ctx, path)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.findLocked(root)
	if r == nil {
		r = &Repo{
			Path:    root,
			GitDir:  gitDir,
			Git:     git.OpenRepo(m.runner, root),
			Objects: git.NewObjectReader(m.runner, root),
		}
		onGraph := m.onGraph
		ahead := m.readAhead
		if ahead <= 0 {
			ahead = DefaultReadAhead
		}
		r.graph = newGraphLoader(r.Git, ahead, func(p GraphProgress) {
			if onGraph != nil {
				onGraph(root, p)
			}
		})
		store := m.settings
		r.graph.rules = func() branchkind.Rules { return Rules(store.Get().Repo(root)) }
		r.graph.stackOverrides = func() map[string]string { return store.Get().Repo(root).StackParents }
		r.graph.reload()
		r.watcher = m.watchLocked(r)
		m.repos = append(m.repos, r)
	}
	err = m.settings.Update(func(s *settings.Settings) {
		s.AddRecent(root, m.now())
		s.OpenTabs = m.pathsLocked()
		s.ActiveTab = root
	})
	return r, err
}

// Close closes the tab for path. Closing a path that isn't open is not an error.
func (m *Manager) Close(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.repos, func(r *Repo) bool { return fsutil.SamePath(r.Path, path) })
	if i < 0 {
		return nil
	}
	m.repos[i].close()
	m.repos = slices.Delete(m.repos, i, i+1)
	return m.settings.Update(func(s *settings.Settings) {
		s.OpenTabs = m.pathsLocked()
		if fsutil.SamePath(s.ActiveTab, path) {
			s.ActiveTab = ""
			if len(m.repos) > 0 {
				s.ActiveTab = m.repos[min(i, len(m.repos)-1)].Path
			}
		}
	})
}

// SetActive records which tab is selected. An empty path means the start page.
func (m *Manager) SetActive(path string) error {
	return m.settings.Update(func(s *settings.Settings) { s.ActiveTab = path })
}

// Get returns the open repository at path, or nil.
func (m *Manager) Get(path string) *Repo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.findLocked(path)
}

// List returns the open repositories in tab order.
func (m *Manager) List() []*Repo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.repos)
}

// Restore reopens the tabs saved in settings. Repositories that no longer open are
// dropped and returned with the reason. The saved active tab (or the start page, "") is
// kept if possible; if the active repo failed to reopen, the first tab is selected.
func (m *Manager) Restore(ctx context.Context) (active string, failed map[string]error) {
	saved := m.settings.Get()
	failed = map[string]error{}
	for _, p := range saved.OpenTabs {
		if _, err := m.Open(ctx, p); err != nil {
			failed[p] = err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch r := m.findLocked(saved.ActiveTab); {
	case saved.ActiveTab == "":
	case saved.ActiveTab == settings.ConfigTabID && saved.ConfigTab != "":
		active = settings.ConfigTabID
	case saved.ActiveTab == settings.InsightsTabID && saved.InsightsTab != "":
		active = settings.InsightsTabID
	case r != nil:
		active = r.Path
	case len(m.repos) > 0:
		active = m.repos[0].Path
	}
	// Rewrite tabs even if nothing reopened, so failed paths aren't retried forever.
	if err := m.settings.Update(func(s *settings.Settings) {
		s.OpenTabs = m.pathsLocked()
		s.ActiveTab = active
	}); err != nil {
		failed[""] = err
	}
	return active, failed
}

// CloseAll stops every repository's background processes. Settings are left as they are,
// so the tabs come back on next launch.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.repos {
		r.close()
	}
	m.repos = nil
}

func (m *Manager) findLocked(path string) *Repo {
	for _, r := range m.repos {
		if fsutil.SamePath(r.Path, path) {
			return r
		}
	}
	return nil
}

func (m *Manager) pathsLocked() []string {
	paths := make([]string, len(m.repos))
	for i, r := range m.repos {
		paths[i] = r.Path
	}
	return paths
}

// Settings returns the settings store the manager keeps in sync.
func (m *Manager) Settings() *settings.Store { return m.settings }

// watchLocked starts the change watcher for r. A watcher that fails to start is not fatal:
// the repo still works, it just won't refresh on its own.
func (m *Manager) watchLocked(r *Repo) *watch.Watcher {
	onChange := m.onChange
	r.changes.mu.Lock()
	r.changes.deliver = func(k watch.Kind) {
		if k.Has(watch.Head) || k.Has(watch.Refs) {
			r.graph.reload()
		}
		if onChange != nil {
			onChange(r.Path, k)
		}
	}
	r.changes.mu.Unlock()
	w, err := m.startWatch(watch.Options{
		Root:   r.Path,
		GitDir: r.GitDir,
		FilterWorkTree: func(paths []string) []string {
			kept, err := r.Git.NotIgnored(context.Background(), paths)
			if err != nil {
				return paths // when unsure, refresh
			}
			return kept
		},
		OnChange: r.changes.report,
	})
	if err != nil {
		log.Printf("watching %s: %v", r.Path, err)
		if m.onWatchFailed != nil {
			m.onWatchFailed(r.Path, err)
		}
		return nil
	}
	return w
}

// GitFor returns a git handle for the repository containing path: the open tab's (sharing
// its write queue) if there is one, otherwise a fresh handle with no watcher or graph.
func (m *Manager) GitFor(ctx context.Context, path string) (*git.Repo, string, error) {
	root, err := m.Resolve(ctx, path)
	if err != nil {
		return nil, "", err
	}
	if r := m.Get(root); r != nil {
		return r.Git, root, nil
	}
	return git.OpenRepo(m.runner, root), root, nil
}
