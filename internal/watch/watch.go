// Package watch reports changes to a repository's refs, index, state and work tree (ADR-0005).
//
// It uses one native recursive subscription per tree (ReadDirectoryChangesW on Windows,
// FSEvents on macOS) rather than a watch per directory, so cost does not grow with the
// size of the work tree. Linux has no such subscription: inotify takes one watch per
// directory, ignored ones included, so a big tree can run into the system's limits.
package watch

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/syncthing/notify"
)

// IsLimit reports whether New failed because of the system's inotify limits: ENOSPC is too
// many watches, and EMFILE is too many inotify instances.
func IsLimit(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EMFILE)
}

// Kind says what part of the repository changed. Values combine as a bit set.
type Kind uint8

const (
	// Head means HEAD moved (checkout, commit, reset…).
	Head Kind = 1 << iota
	// Refs means branches, tags or remotes changed.
	Refs
	// Index means the staging area changed.
	Index
	// State means a merge, rebase, cherry-pick, revert or bisect started or ended.
	State
	// Config means .git/config changed (remotes, upstreams…).
	Config
	// WorkTree means files in the working tree changed.
	WorkTree
)

// Has reports whether k includes all of other.
func (k Kind) Has(other Kind) bool { return k&other == other }

// Names returns the set kinds as strings, for the frontend.
func (k Kind) Names() []string {
	names := []string{}
	for _, n := range []struct {
		k    Kind
		name string
	}{{Head, "head"}, {Refs, "refs"}, {Index, "index"}, {State, "state"}, {Config, "config"}, {WorkTree, "worktree"}} {
		if k.Has(n.k) {
			names = append(names, n.name)
		}
	}
	return names
}

// Options configures a Watcher.
type Options struct {
	// Root is the work tree root.
	Root string
	// GitDir is the repository's git directory; usually Root/.git, but it can live
	// elsewhere for linked worktrees and submodules.
	GitDir string
	// Debounce is how long the tree must be quiet before changes are reported. Default 150ms.
	Debounce time.Duration
	// MaxWait bounds how long a constant stream of changes can delay a report. Default 1s.
	MaxWait time.Duration
	// FilterWorkTree, if set, receives changed work-tree paths (relative, slash-separated)
	// and returns those that matter. It lets the caller drop ignored files.
	FilterWorkTree func(paths []string) []string
	// OnChange receives the combined kinds after each debounce window.
	OnChange func(Kind)
}

// Watcher watches one repository until Close is called.
type Watcher struct {
	opts   Options
	events chan notify.EventInfo
	done   chan struct{}
	wg     sync.WaitGroup
}

// New starts watching. Callers must Close the Watcher.
func New(opts Options) (*Watcher, error) {
	if opts.Debounce == 0 {
		opts.Debounce = 150 * time.Millisecond
	}
	if opts.MaxWait == 0 {
		opts.MaxWait = time.Second
	}
	opts.Root = filepath.Clean(opts.Root)
	opts.GitDir = filepath.Clean(opts.GitDir)
	if r, err := filepath.EvalSymlinks(opts.Root); err == nil {
		opts.Root = r
	}
	if g, err := filepath.EvalSymlinks(opts.GitDir); err == nil {
		opts.GitDir = g
	}

	w := &Watcher{
		opts: opts,
		// Buffered so bursts (e.g. a checkout touching thousands of files) don't block
		// the OS callback while we debounce.
		events: make(chan notify.EventInfo, 4096),
		done:   make(chan struct{}),
	}
	if err := notify.Watch(filepath.Join(opts.Root, "..."), w.events, notify.All); err != nil {
		return nil, err
	}
	if !within(opts.GitDir, opts.Root) {
		if err := notify.Watch(filepath.Join(opts.GitDir, "..."), w.events, notify.All); err != nil {
			notify.Stop(w.events)
			return nil, err
		}
	}
	w.wg.Add(1)
	go w.loop()
	return w, nil
}

// Close stops watching and waits for any pending report to finish.
func (w *Watcher) Close() {
	notify.Stop(w.events)
	close(w.done)
	w.wg.Wait()
}

func (w *Watcher) loop() {
	defer w.wg.Done()
	var (
		kinds     Kind
		workPaths = map[string]struct{}{}
		quiet     *time.Timer
		deadline  *time.Timer
		quietC    <-chan time.Time
		deadlineC <-chan time.Time
	)
	flush := func() {
		if quiet != nil {
			quiet.Stop()
		}
		if deadline != nil {
			deadline.Stop()
		}
		quietC, deadlineC = nil, nil
		if len(workPaths) > 0 {
			paths := make([]string, 0, len(workPaths))
			for p := range workPaths {
				paths = append(paths, p)
			}
			if w.opts.FilterWorkTree == nil || len(w.opts.FilterWorkTree(paths)) > 0 {
				kinds |= WorkTree
			}
			clear(workPaths)
		}
		if kinds != 0 && w.opts.OnChange != nil {
			w.opts.OnChange(kinds)
		}
		kinds = 0
	}

	for {
		select {
		case <-w.done:
			return
		case ev := <-w.events:
			k, rel := w.classify(ev.Path())
			if k == 0 && rel == "" {
				continue
			}
			kinds |= k
			if rel != "" {
				workPaths[rel] = struct{}{}
			}
			if quiet == nil {
				quiet = time.NewTimer(w.opts.Debounce)
			} else {
				quiet.Reset(w.opts.Debounce)
			}
			quietC = quiet.C
			if deadlineC == nil {
				if deadline == nil {
					deadline = time.NewTimer(w.opts.MaxWait)
				} else {
					deadline.Reset(w.opts.MaxWait)
				}
				deadlineC = deadline.C
			}
		case <-quietC:
			flush()
		case <-deadlineC:
			flush()
		}
	}
}

// classify maps an absolute path to the kind of change it represents. For work-tree files
// it returns the slash-separated path relative to the root instead.
func (w *Watcher) classify(path string) (Kind, string) {
	if rel, ok := relTo(path, w.opts.GitDir); ok {
		return classifyGitPath(rel), ""
	}
	rel, ok := relTo(path, w.opts.Root)
	if !ok || rel == "." {
		return 0, ""
	}
	return 0, rel
}

// classifyGitPath maps a path inside the git directory (slash-separated) to a Kind. Lock
// files, objects and logs are ignored: the rename that completes a write is what counts.
func classifyGitPath(rel string) Kind {
	if strings.HasSuffix(rel, ".lock") {
		return 0
	}
	first, rest, _ := strings.Cut(rel, "/")
	switch first {
	case "worktrees":
		// Another worktree added, removed or switched branches, which changes what the refs
		// report. Its commits and the like are its own business.
		if strings.HasSuffix(rest, "/HEAD") && strings.Count(rest, "/") == 1 {
			return Refs
		}
		return 0
	case "HEAD", "ORIG_HEAD":
		return Head
	case "refs", "packed-refs", "FETCH_HEAD":
		return Refs
	case "index":
		return Index
	case "config":
		return Config
	case "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG", "BISECT_START",
		"rebase-merge", "rebase-apply", "sequencer":
		return State
	}
	return 0
}

// relTo returns path relative to base, slash-separated, if path is base or inside it.
func relTo(path, base string) (string, bool) {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func within(path, base string) bool {
	_, ok := relTo(path, base)
	return ok
}
