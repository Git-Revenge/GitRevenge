package gitconfig

import (
	"os"
	"sync"
	"time"
)

// pollInterval is how often config files are checked: the longest an outside edit takes to
// show. Checking is one stat per file, and there are only a few files.
const pollInterval = 2 * time.Second

// Watcher reports changes to individual config files, which can live anywhere, so repo
// watchers don't cover them.
//
// It polls rather than watching their folders. The global config usually sits in the home
// folder, and the notify package keeps one watch tree per process, so a watch on that folder
// silences the recursive watch of every repository inside it (#377).
type Watcher struct {
	onChange func(path string)
	interval time.Duration
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
	return newWatcher(onChange, pollInterval)
}

func newWatcher(onChange func(path string), interval time.Duration) *Watcher {
	w := &Watcher{
		onChange: onChange,
		interval: interval,
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

// check returns the files changed since the last check. It doesn't hold the lock while it
// reads the disk, since a file on a slow network share mustn't hold up Add.
func (w *Watcher) check() []string {
	w.mu.Lock()
	files := make(map[string]string, len(w.files))
	for key, f := range w.files {
		files[key] = f.path
	}
	w.mu.Unlock()

	states := make(map[string]fileState, len(files))
	for key, path := range files {
		states[key] = stateOf(path)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	var changed []string
	for key, state := range states {
		if f := w.files[key]; f.state != state {
			f.state = state
			changed = append(changed, f.path)
		}
	}
	return changed
}
