package gitconfig

import (
	"os"
	"sync"
	"time"
)

const (
	// pollInterval is how often config files are checked. Checking is one stat per file, and
	// there are only a few files.
	pollInterval = 2 * time.Second
	// settleDelay is how long a changed file must stay the same before it's reported, so it
	// isn't read half written: a write that isn't atomic truncates first.
	settleDelay = 150 * time.Millisecond
	// settleChecks bounds the wait, so a file that never stops changing is still reported.
	settleChecks = 10
)

// Watcher reports changes to individual config files, which can live anywhere, so repo
// watchers don't cover them.
//
// It polls rather than watching their folders. The global config usually sits in the home
// folder, and the notify package keeps one watch tree per process, so a watch on that folder
// silences the recursive watch of every repository inside it (#377).
type Watcher struct {
	onChange func(path string)
	interval time.Duration
	settle   time.Duration
	done     chan struct{}
	wg       sync.WaitGroup

	mu sync.Mutex
	// files maps a normalized path to what's known about it.
	files map[string]*watchedFile
}

type watchedFile struct {
	// path is the path given to Add: what's checked, and what's reported.
	path  string
	state fileState
}

// fileState is what a check compares. A file that doesn't exist has the zero value, so
// creating or deleting one counts as a change.
type fileState struct {
	exists  bool
	modTime time.Time
	size    int64
}

// stateOf follows symlinks, so an edit to a link's target or a repointed link both show.
func stateOf(path string) fileState {
	info, err := os.Stat(path)
	if err != nil {
		return fileState{}
	}
	return fileState{exists: true, modTime: info.ModTime(), size: info.Size()}
}

// NewWatcher starts a Watcher that calls onChange, from its own goroutine, with the path
// given to Add.
func NewWatcher(onChange func(path string)) *Watcher {
	return newWatcher(onChange, pollInterval, settleDelay)
}

func newWatcher(onChange func(path string), interval, settle time.Duration) *Watcher {
	w := &Watcher{
		onChange: onChange,
		interval: interval,
		settle:   settle,
		done:     make(chan struct{}),
		files:    map[string]*watchedFile{},
	}
	w.wg.Add(1)
	go w.loop()
	return w
}

// Add watches path, which needn't exist yet. Adding a file again only changes the path it's
// reported under.
func (w *Watcher) Add(path string) {
	key := Normalize(path)
	state := stateOf(path)
	w.mu.Lock()
	defer w.mu.Unlock()
	if f, ok := w.files[key]; ok {
		f.path = path
		return
	}
	w.files[key] = &watchedFile{path: path, state: state}
}

// Close stops watching.
func (w *Watcher) Close() {
	close(w.done)
	w.wg.Wait()
}

func (w *Watcher) loop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			for _, path := range w.check() {
				w.onChange(path)
			}
		}
	}
}

// check returns the files changed since the last check, once they've stopped changing. It
// doesn't hold the lock while it reads the disk or waits, since a file on a slow network share
// mustn't hold up Add.
func (w *Watcher) check() []string {
	w.mu.Lock()
	paths := make(map[string]string, len(w.files))
	known := make(map[string]fileState, len(w.files))
	for key, f := range w.files {
		paths[key] = f.path
		known[key] = f.state
	}
	w.mu.Unlock()

	states := make(map[string]fileState)
	for key, path := range paths {
		if state := stateOf(path); state != known[key] {
			states[key] = state
		}
	}
	if len(states) == 0 {
		return nil
	}
	for range settleChecks {
		select {
		case <-w.done:
			return nil
		case <-time.After(w.settle):
		}
		settled := true
		for key := range states {
			if state := stateOf(paths[key]); state != states[key] {
				states[key] = state
				settled = false
			}
		}
		if settled {
			break
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	changed := make([]string, 0, len(states))
	for key, state := range states {
		f := w.files[key]
		f.state = state
		changed = append(changed, f.path)
	}
	return changed
}
