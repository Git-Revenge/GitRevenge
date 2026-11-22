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
	"sync/atomic"
	"time"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/fsutil"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
	"github.com/git-revenge/gitrevenge/internal/settings"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
	"github.com/git-revenge/gitrevenge/internal/trust"
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
	GitDir string
	// CommonDir is the git directory shared by all of its worktrees: GitDir, except in a linked
	// worktree.
	CommonDir string
	Git       *git.Repo
	Objects   *git.ObjectReader
	// watcher runs only while the repository is on screen (see Manager.rewatch), which
	// guards it with watchMu.
	watcher  *watch.Watcher
	graph    *graphLoader
	trailers *trailerReader
	changes  changeGate
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
// patterns. A graph that hasn't loaded yet reads them when it does.
func (r *Repo) ReloadGraph() { r.graph.reloadStarted() }

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

// SetFilter narrows the graph to the commits filter keeps, or back to everything with nil,
// for this session only, and reloads it.
func (r *Repo) SetFilter(filter *Filter) { r.graph.SetFilter(filter) }

// Authored says how many commits of the history under the smart filter were authored in each
// of some spans of time: see graphLoader.Authored.
func (r *Repo) Authored(spans [][2]int64) ([]int, bool) { return r.graph.Authored(spans) }

// Filtered reports whether a filter is narrowing the graph, how many commits the load under
// it has looked through so far, and how many the history has in all (0 until counted).
func (r *Repo) Filtered() (on bool, looked, total int) { return r.graph.Filtered() }

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
	return branchkind.New(overrides, rs.Hide).HidingKinds(hidden).WithBotAuthors(rs.BotAuthors)
}

// close stops the repository's background work. Its watcher is the manager's to stop.
func (r *Repo) close() {
	r.Git.Close()
	r.graph.stop()
	r.trailers.close()
	_ = r.Objects.Close()
}

// Trailers returns the repository's trailer index: every commit's trailers, read in the
// background and kept (see trailerindex). It may still be filling.
func (r *Repo) Trailers() *trailerindex.Index { return r.trailers.index }

// Manager owns the open repositories. It is safe for concurrent use.
type Manager struct {
	runner   *git.Runner
	settings *settings.Store
	now      func() time.Time

	mu            sync.Mutex
	repos         []*Repo // tab order
	closed        bool
	onChange      func(path string, k watch.Kind)
	onWatchFailed func(path string, err error)
	onShown       func(path string, late bool)
	// startWatch is watch.New, except in tests.
	startWatch func(watch.Options) (*watch.Watcher, error)
	onGraph    func(path string, p GraphProgress)
	onTrailers func(path string, st trailerindex.Status)
	// trailerDir is where trailer indexes are kept; "" keeps them in memory only.
	trailerDir string
	// readAhead overrides DefaultReadAhead; tests use it.
	readAhead int
	// wait is watchWait, except in tests.
	wait time.Duration

	// saveMu orders the writes of the open tabs to settings, so the last one written lists
	// them as they are. It's taken before mu, never while holding it: saving syncs a file
	// to disk, and every call for every tab waits on mu.
	saveMu sync.Mutex

	// watchMu guards watching and each Repo's watcher. Starting a watcher can take a while
	// on a big tree, so it isn't done under mu either.
	watchMu sync.Mutex
	// watching are the repositories on screen, which are the ones watched (see rewatch).
	watching []*Repo

	// life ends when the manager closes, and work holds what it started on its own.
	life context.Context
	end  context.CancelFunc
	work sync.WaitGroup
}

// OnGraph registers fn to receive commit graph loading progress for open repositories.
// Call it before opening repositories.
func (m *Manager) OnGraph(fn func(path string, p GraphProgress)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onGraph = fn
}

// OnTrailers registers fn to hear how far reading a repository's trailers has got, and
// says where trailer indexes are kept: a folder of the app's own, never the repository.
// With dir "" they're kept in memory and read again each launch. Call it before opening
// repositories.
func (m *Manager) OnTrailers(dir string, fn func(path string, st trailerindex.Status)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trailerDir, m.onTrailers = dir, fn
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

// OnShown registers fn to be called (from a background goroutine) when a repository comes on
// screen and is watched from then on. Nothing reported what changed in it while it was off
// screen, so whatever fn's caller keeps about it wants reading again. late means something
// may have read the repository already, before it was watched: the watcher took so long to
// start that the caller of SetActive or Open went on without it, or the repository was on
// screen all along and its watcher had to restart. Call it before opening repositories.
func (m *Manager) OnShown(fn func(path string, late bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onShown = fn
}

// NewManager returns a Manager with no open repositories.
func NewManager(runner *git.Runner, store *settings.Store) *Manager {
	// Repositories from before trust was asked about are trusted as they stand.
	if !store.Get().TrustSeeded {
		_ = store.Update(func(s *settings.Settings) { s.SeedTrust() })
	}
	life, end := context.WithCancel(context.Background())
	return &Manager{runner: runner, settings: store, now: time.Now, startWatch: watch.New, wait: watchWait, life: life, end: end}
}

// spawn runs fn on its own goroutine, which CloseAll waits for. It reports false, without
// running fn, once the manager has closed.
func (m *Manager) spawn(fn func()) bool {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return false
	}
	m.work.Add(1)
	m.mu.Unlock()
	go func() {
		defer m.work.Done()
		fn()
	}()
	return true
}

// UntrustedError means a repository's own config or hooks would make git run programs, and
// the user hasn't agreed to that for this repository. Nothing has been run in it beyond
// finding where it is.
type UntrustedError struct {
	// Root is the work tree root.
	Root string
	// Findings are what it would run.
	Findings []trust.Finding
}

func (e *UntrustedError) Error() string {
	return fmt.Sprintf("%s: its own git config or hooks would run programs, and it hasn't been trusted to", e.Root)
}

// vet lets a repository through if the user trusts it, or if its config and hooks name
// nothing to run. Otherwise it returns an *UntrustedError, before any git command that
// would act on that config has run there.
func (m *Manager) vet(ctx context.Context, root, gitDir, commonDir string) error {
	if s := m.settings.Get(); s.Trusts(root, commonDir) {
		return nil
	}
	found, err := trust.Scan(ctx, m.runner, gitDir, commonDir)
	if err != nil {
		return err
	}
	if len(found) > 0 {
		return &UntrustedError{Root: root, Findings: found}
	}
	return nil
}

// Trust records that the user agrees the repository containing path may run what its config
// and hooks name, and returns its root. Linked worktrees share the main one's git folder, so
// trusting one covers them all.
func (m *Manager) Trust(ctx context.Context, path string) (string, error) {
	root, _, commonDir, err := m.resolve(ctx, path)
	if err != nil {
		return "", err
	}
	return root, m.settings.Update(func(s *settings.Settings) { s.Trust(root, commonDir) })
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
	root, _, _, err := m.resolve(ctx, path)
	return root, err
}

// resolve returns the work tree root, git directory and common git directory containing path.
func (m *Manager) resolve(ctx context.Context, path string) (root, gitDir, commonDir string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", "", err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return "", "", "", fmt.Errorf("%s: folder not found", abs)
	}
	// --show-toplevel fails outside a work tree, so classify the folder first.
	out, err := m.runner.Output(ctx, git.Cmd{
		Dir:      abs,
		Args:     []string{"rev-parse", "--is-bare-repository", "--is-inside-work-tree"},
		ReadOnly: true,
	})
	if err != nil {
		var gerr *git.Error
		switch {
		case !errors.As(err, &gerr):
		case strings.Contains(gerr.Stderr, "not a git repository"):
			return "", "", "", fmt.Errorf("%s: %w", abs, ErrNotRepo)
		case strings.Contains(gerr.Stderr, "cannot use bare repository"):
			// With safe.bareRepository=explicit, Git 3.0's default, git won't look inside a
			// bare repository it wasn't pointed at, and says so instead of answering.
			return "", "", "", fmt.Errorf("%s: %w", abs, ErrBare)
		}
		return "", "", "", err
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return "", "", "", fmt.Errorf("unexpected rev-parse output %q", out)
	}
	switch f[0] + " " + f[1] {
	case "true false":
		return "", "", "", fmt.Errorf("%s: %w", abs, ErrBare)
	case "false false":
		// Inside a .git directory.
		return "", "", "", fmt.Errorf("%s: %w", abs, ErrNotRepo)
	}
	out, err = m.runner.Output(ctx, git.Cmd{
		Dir:      abs,
		Args:     []string{"rev-parse", "--show-toplevel", "--absolute-git-dir", "--path-format=absolute", "--git-common-dir"},
		ReadOnly: true,
	})
	if err != nil {
		return "", "", "", err
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		return "", "", "", fmt.Errorf("unexpected rev-parse output %q", out)
	}
	clean := func(p string) string { return filepath.Clean(filepath.FromSlash(p)) }
	return clean(lines[0]), clean(lines[1]), clean(lines[2]), nil
}

// Open opens the repository containing path as a tab, or returns it if already open, and
// records it as most recently used and active.
func (m *Manager) Open(ctx context.Context, path string) (*Repo, error) {
	root, gitDir, commonDir, err := m.resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	if m.Get(root) == nil {
		if err := m.vet(ctx, root, gitDir, commonDir); err != nil {
			return nil, err
		}
	}
	r := m.add(root, gitDir, commonDir)
	r.graph.start()
	err = m.saveTabs(func(s *settings.Settings) {
		s.AddRecent(root, m.now())
		s.ActiveTab = root
		// Opened once, it's the user's from here on: a hook manager or Git LFS they set up
		// in it later doesn't stop its tab coming back. The question is for first contact.
		s.Trust(root, commonDir)
	})
	m.rewatch(m.wait)
	return r, err
}

// add returns the open repository at root, adding it as the last tab if it isn't open. A new
// one loads nothing and isn't watched until it's asked to.
func (m *Manager) add(root, gitDir, commonDir string) *Repo {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.findLocked(root); r != nil {
		return r
	}
	r := &Repo{
		Path:      root,
		GitDir:    gitDir,
		CommonDir: commonDir,
		Git:       git.OpenRepo(m.runner, root),
		Objects:   git.NewObjectReader(m.runner, root),
	}
	onGraph := m.onGraph
	ahead := m.readAhead
	if ahead <= 0 {
		ahead = DefaultReadAhead
	}
	onTrailers := m.onTrailers
	r.trailers = newTrailerReader(trailerindex.Open(m.trailerDir, commonDir), r.Git, func(st trailerindex.Status) {
		if onTrailers != nil {
			onTrailers(root, st)
		}
	})
	r.graph = newGraphLoader(r.Git, ahead, func(p GraphProgress) {
		if onGraph != nil {
			onGraph(root, p)
		}
		// The trailers are read once the graph has what it wants for now: the first screen
		// of commits comes before a walk of the whole history.
		if p.Done || p.Paused {
			r.trailers.kick()
		}
	})
	// Whatever made the graph load again, a commit or a fetch among them, may have brought
	// commits whose trailers haven't been read.
	r.graph.onReload = r.trailers.refsMoved
	store := m.settings
	r.graph.rules = func() branchkind.Rules { return Rules(store.Get().Repo(root)) }
	r.graph.stackOverrides = func() map[string]string { return store.Get().Repo(root).StackParents }
	onChange := m.onChange
	r.changes.deliver = func(k watch.Kind) {
		switch {
		case k.Has(watch.Head) || k.Has(watch.Refs):
			r.graph.reloadStarted()
		case k.Has(watch.Config) || k.Has(watch.WorkTree):
			// A saved .mailmap, or a mailmap.file setting, changes the names on every row.
			if r.graph.mailmapMoved(context.Background(), k.Has(watch.Config)) {
				r.graph.reloadStarted()
			}
		}
		if onChange != nil {
			onChange(r.Path, k)
		}
	}
	m.repos = append(m.repos, r)
	return r
}

// saveTabs writes the open tabs to settings, with change's edits in the same write.
func (m *Manager) saveTabs(change func(s *settings.Settings)) error {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	m.mu.Lock()
	paths := m.pathsLocked()
	m.mu.Unlock()
	return m.settings.Update(func(s *settings.Settings) {
		s.OpenTabs = paths
		if change != nil {
			change(s)
		}
	})
}

// Close closes the tab for path. Closing a path that isn't open is not an error.
func (m *Manager) Close(path string) error {
	m.mu.Lock()
	i := slices.IndexFunc(m.repos, func(r *Repo) bool { return fsutil.SamePath(r.Path, path) })
	if i < 0 {
		m.mu.Unlock()
		return nil
	}
	closed := m.repos[i]
	m.repos = slices.Delete(m.repos, i, i+1)
	next := ""
	if len(m.repos) > 0 {
		next = m.repos[min(i, len(m.repos)-1)].Path
	}
	m.mu.Unlock()
	err := m.saveTabs(func(s *settings.Settings) {
		if fsutil.SamePath(s.ActiveTab, path) {
			s.ActiveTab = next
		}
	})
	// However long it takes: once this returns, nothing of the app's is holding the folder,
	// which on Windows would stop it being moved or deleted.
	m.rewatch(0)
	closed.close()
	return err
}

// watchWait is how long Open and SetActive wait for the repository coming on screen to be
// watched. Their caller reads it once they return, and a change between that read and the
// watcher starting would go unseen, so the watcher is given the time it nearly always needs
// (a few milliseconds; one watch per folder on Linux can take longer in a big tree). Past
// that the caller goes on, and OnShown says the repository wants reading again.
const watchWait = 100 * time.Millisecond

// shown lists the open repositories on screen: the active tab's, or the ones the Ignore
// rules, People or Insights tab is showing. The start page and the Git Config tab show none (config
// files are watched on their own).
func (m *Manager) shown() []*Repo {
	s := m.settings.Get()
	var paths []string
	switch s.ActiveTab {
	case "", settings.ConfigTabID:
	case settings.IgnoreTabID:
		paths = []string{s.IgnoreTab}
	case settings.PeopleTabID:
		paths = []string{s.PeopleTab}
	case settings.InsightsTabID:
		switch kind, id, _ := strings.Cut(s.InsightsTab, ":"); kind {
		case "repo":
			paths = []string{id}
		case "group":
			for _, g := range s.Groups {
				if g.Name == id {
					paths = g.Paths
				}
			}
		}
	default:
		paths = []string{s.ActiveTab}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var repos []*Repo
	for _, p := range paths {
		if r := m.findLocked(p); r != nil && !slices.Contains(repos, r) {
			repos = append(repos, r)
		}
	}
	return repos
}

// Rewatch brings what's watched in line with what's on screen, after a change to what a tab
// shows that the manager didn't make: the Insights tab's scope, the Ignore rules tab's
// repository, or a group's repositories.
func (m *Manager) Rewatch() { m.rewatch(m.wait) }

// rewatch watches the repositories on screen and no others. A repository off screen has
// nothing to refresh, and a watcher for each open tab costs a subscription (on Linux, a
// watch for every folder in the tree) and a round of git calls on every change, for tabs
// nobody is looking at. One that comes on screen is brought up to date instead (see watch).
//
// It waits up to wait for the watchers to be in place, or however long that takes if wait
// is 0.
func (m *Manager) rewatch(wait time.Duration) {
	var late atomic.Bool
	done := make(chan struct{})
	if !m.spawn(func() {
		defer close(done)
		m.watchShown(&late)
	}) {
		return
	}
	if wait <= 0 {
		<-done
		return
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-done:
	case <-t.C:
		late.Store(true)
	}
}

// watchShown is rewatch's work. late is set once rewatch's caller has stopped waiting.
func (m *Manager) watchShown(late *atomic.Bool) {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()
	want := m.shown()
	if slices.Equal(want, m.watching) {
		return
	}
	// The file watching library keeps one tree of watches for the whole process. Stopping a
	// watch inside another one, as a linked worktree's git folder is inside the main
	// repository's, also silences the outer one, and starting it again only brings it back
	// once all of them have stopped. So a change stops every watcher before starting any.
	was := m.watching
	for _, r := range was {
		if r.watcher != nil {
			r.watcher.Close()
			r.watcher = nil
		}
	}
	m.watching = want
	m.mu.Lock()
	onShown := m.onShown
	m.mu.Unlock()
	for _, r := range want {
		r.watcher = m.watch(r)
		// One that was watched until a moment ago missed only that moment, but can't tell
		// what was in it, so it's brought up to date like one that's new on screen.
		m.spawn(func() { r.graph.refresh(m.life) })
		if onShown != nil {
			onShown(r.Path, late.Load() || slices.Contains(was, r))
		}
	}
}

// SetActive records which tab is selected, and watches what it shows in place of what the
// last one did. An empty path means the start page.
func (m *Manager) SetActive(path string) error {
	err := m.settings.Update(func(s *settings.Settings) { s.ActiveTab = path })
	m.rewatch(m.wait)
	return err
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

// restoreParallel is how many saved tabs Restore looks at at once.
const restoreParallel = 4

// Restore reopens the tabs saved in settings. Repositories that no longer open are
// dropped and returned with the reason. The saved active tab (or the start page, "") is
// kept if possible; if the active repo failed to reopen, the first tab is selected.
//
// Only the tab on screen loads its graph straight away. The others load after it, one at a
// time, so a window full of tabs doesn't start a `git log` each at once and make the one
// being looked at wait.
func (m *Manager) Restore(ctx context.Context) (active string, failed map[string]error) {
	saved := m.settings.Get()
	failed = map[string]error{}
	// Finding a repository and checking it's trusted is a few git calls: several tabs at a
	// time, and opened in their saved order whichever answers first.
	type place struct {
		root, gitDir, commonDir string
		err                     error
	}
	places := make([]place, len(saved.OpenTabs))
	var wg sync.WaitGroup
	slots := make(chan struct{}, restoreParallel)
	for i, p := range saved.OpenTabs {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			pl := &places[i]
			if pl.root, pl.gitDir, pl.commonDir, pl.err = m.resolve(ctx, p); pl.err == nil {
				pl.err = m.vet(ctx, pl.root, pl.gitDir, pl.commonDir)
			}
		})
	}
	wg.Wait()
	var opened []*Repo
	for i, pl := range places {
		if pl.err != nil {
			failed[saved.OpenTabs[i]] = pl.err
			continue
		}
		if r := m.add(pl.root, pl.gitDir, pl.commonDir); !slices.Contains(opened, r) {
			opened = append(opened, r)
		}
	}

	m.mu.Lock()
	shown := m.findLocked(saved.ActiveTab)
	switch {
	case saved.ActiveTab == "":
	case saved.ActiveTab == settings.ConfigTabID && saved.ConfigTab != "":
		active = settings.ConfigTabID
	case saved.ActiveTab == settings.InsightsTabID && saved.InsightsTab != "":
		active = settings.InsightsTabID
	case saved.ActiveTab == settings.IgnoreTabID && saved.IgnoreTab != "":
		active = settings.IgnoreTabID
	case saved.ActiveTab == settings.PeopleTabID && saved.PeopleTab != "":
		active = settings.PeopleTabID
	case shown != nil:
		active = shown.Path
	case len(m.repos) > 0:
		shown = m.repos[0]
		active = shown.Path
	}
	m.mu.Unlock()
	// Rewrite tabs even if nothing reopened, so failed paths aren't retried forever.
	if err := m.saveTabs(func(s *settings.Settings) {
		for _, r := range opened {
			s.AddRecent(r.Path, m.now())
			s.Trust(r.Path, r.CommonDir)
		}
		s.ActiveTab = active
	}); err != nil {
		failed[""] = err
	}
	m.warm(shown, opened)
	m.rewatch(m.wait)
	return active, failed
}

// warm loads the graph of first now, if there is one, and then those of the rest in order,
// each once the one before has loaded as far as it will for now. A tab that's asked for
// before its turn loads then.
func (m *Manager) warm(first *Repo, rest []*Repo) {
	if first != nil {
		first.graph.start()
	}
	m.spawn(func() {
		if first != nil && !first.graph.settled(m.life) {
			return
		}
		for _, r := range rest {
			r.graph.start()
			if !r.graph.settled(m.life) {
				return
			}
		}
	})
}

// CloseAll stops every repository's background processes. Settings are left as they are,
// so the tabs come back on next launch.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.end()
	m.work.Wait()
	m.watchMu.Lock()
	for _, r := range m.watching {
		if r.watcher != nil {
			r.watcher.Close()
			r.watcher = nil
		}
	}
	m.watching = nil
	m.watchMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.repos {
		r.close()
	}
	m.repos = nil
	// Ready to open repositories again, as Restore does in tests.
	m.life, m.end = context.WithCancel(context.Background())
	m.closed = false
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

// watch starts the change watcher for r. A watcher that fails to start is not fatal: the
// repo still works, it just won't refresh on its own.
func (m *Manager) watch(r *Repo) *watch.Watcher {
	w, err := m.startWatch(watch.Options{
		Root:      r.Path,
		GitDir:    r.GitDir,
		CommonDir: r.CommonDir,
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
		m.mu.Lock()
		failed := m.onWatchFailed
		m.mu.Unlock()
		if failed != nil {
			failed(r.Path, err)
		}
		return nil
	}
	return w
}

// GitFor returns a git handle for the repository containing path: the open tab's (sharing
// its write queue) if there is one, otherwise a fresh handle with no watcher or graph.
func (m *Manager) GitFor(ctx context.Context, path string) (*git.Repo, string, error) {
	root, gitDir, commonDir, err := m.resolve(ctx, path)
	if err != nil {
		return nil, "", err
	}
	if r := m.Get(root); r != nil {
		return r.Git, root, nil
	}
	if err := m.vet(ctx, root, gitDir, commonDir); err != nil {
		return nil, "", err
	}
	return git.OpenRepo(m.runner, root), root, nil
}
