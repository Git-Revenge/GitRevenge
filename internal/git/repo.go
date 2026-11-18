package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Repo runs git commands against one repository. Mutating commands are serialized so the
// app never races itself; read commands run concurrently and never take optional locks.
type Repo struct {
	runner *Runner
	dir    string
	// writes is a one-slot semaphore. A channel rather than a mutex so waiting respects ctx.
	writes chan struct{}
	// open ends when the Repo is closed, which stops the reads still running (see Close).
	open   context.Context
	closed context.CancelFunc
}

// OpenRepo returns a Repo rooted at dir. It does not validate that dir is a repository.
func OpenRepo(r *Runner, dir string) *Repo {
	open, closed := context.WithCancel(context.Background())
	return &Repo{runner: r, dir: dir, writes: make(chan struct{}, 1), open: open, closed: closed}
}

// Dir returns the repository's working directory.
func (r *Repo) Dir() string { return r.dir }

// Close stops the reads still running in the repository, and any started later fail at once:
// a blame or a file's history on a big repository can outlast the tab that asked for it by
// minutes. Writes are left to finish, since a git command killed halfway leaves its lock
// file and half its work behind. Closing is optional; a Repo holds nothing else.
func (r *Repo) Close() { r.closed() }

// reading returns ctx for a read: it also ends when the Repo closes. Call done when the
// read is over.
func (r *Repo) reading(ctx context.Context) (reading context.Context, done func()) {
	reading, cancel := context.WithCancel(ctx)
	if r.open.Err() != nil {
		// Already closed. AfterFunc would say so from a goroutine of its own, a moment after
		// a quick command had started and finished.
		cancel()
		return reading, func() {}
	}
	stop := context.AfterFunc(r.open, cancel)
	return reading, func() {
		stop()
		cancel()
	}
}

// readCmd runs c in the repository as a read, for one that needs more than arguments (an
// environment, or input).
func (r *Repo) readCmd(ctx context.Context, c Cmd) ([]byte, error) {
	ctx, done := r.reading(ctx)
	defer done()
	c.Dir, c.ReadOnly = r.dir, true
	return r.runner.Run(ctx, c)
}

// streamCmd is readCmd for output read as it arrives.
func (r *Repo) streamCmd(ctx context.Context, c Cmd, consume func(io.Reader) error) error {
	ctx, done := r.reading(ctx)
	defer done()
	c.Dir, c.ReadOnly = r.dir, true
	return r.runner.Stream(ctx, c, consume)
}

// Read runs a read-only git command and returns its stdout.
func (r *Repo) Read(ctx context.Context, args ...string) ([]byte, error) {
	return r.readCmd(ctx, Cmd{Args: args})
}

// ReadString runs a read-only git command and returns trimmed stdout.
func (r *Repo) ReadString(ctx context.Context, args ...string) (string, error) {
	out, err := r.readCmd(ctx, Cmd{Args: args})
	return strings.TrimSpace(string(out)), err
}

// Stream runs a read-only git command and passes its stdout to consume as it arrives.
func (r *Repo) Stream(ctx context.Context, consume func(io.Reader) error, args ...string) error {
	return r.streamCmd(ctx, Cmd{Args: args}, consume)
}

// lockRetryDelays is how long Write waits before each retry while another process holds a
// git lock file. The total is about 1.5s, long enough for a typical CLI command to finish.
var lockRetryDelays = []time.Duration{
	100 * time.Millisecond,
	200 * time.Millisecond,
	400 * time.Millisecond,
	800 * time.Millisecond,
}

// Write runs a mutating git command. It waits for any earlier Write on this Repo to finish,
// and retries briefly if another process holds a git lock. If the lock persists it returns
// a *LockedError.
func (r *Repo) Write(ctx context.Context, c Cmd) ([]byte, error) {
	select {
	case r.writes <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-r.writes }()

	c.Dir = r.dir
	c.ReadOnly = false
	for attempt := 0; ; attempt++ {
		out, err := r.runner.Run(ctx, c)
		lockPath, locked := lockFileFrom(err)
		if !locked {
			return out, err
		}
		if attempt == len(lockRetryDelays) {
			return out, &LockedError{Path: lockPath, Err: err}
		}
		select {
		case <-time.After(lockRetryDelays[attempt]):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// ErrLocked matches any *LockedError via errors.Is.
var ErrLocked = errors.New("another git process is running")

// LockedError means a git lock file (usually index.lock) stayed held by another process.
type LockedError struct {
	Path string
	Err  error
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("%s: %s exists. If no other git process is running, delete that file and try again", ErrLocked, e.Path)
}

func (e *LockedError) Unwrap() []error { return []error{ErrLocked, e.Err} }

// lockRE matches git's message when it cannot create a lock file, e.g.
// "fatal: Unable to create '/repo/.git/index.lock': File exists."
var lockRE = regexp.MustCompile(`Unable to create '([^']+\.lock)': File exists`)

func lockFileFrom(err error) (string, bool) {
	var gerr *Error
	if !errors.As(err, &gerr) {
		return "", false
	}
	m := lockRE.FindStringSubmatch(gerr.Stderr)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// NotIgnored returns the paths (relative to the work tree) that .gitignore rules do not
// exclude. It runs one `git check-ignore` for the whole batch.
func (r *Repo) NotIgnored(ctx context.Context, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	in := strings.Join(paths, "\x00") + "\x00"
	out, err := r.readCmd(ctx, Cmd{Args: []string{"check-ignore", "--stdin", "-z"}, Stdin: strings.NewReader(in)})
	// Exit status 1 means none of the paths are ignored.
	var gerr *Error
	if errors.As(err, &gerr) && gerr.ExitCode == 1 && gerr.Stderr == "" {
		return paths, nil
	}
	if err != nil {
		return nil, err
	}
	ignored := map[string]struct{}{}
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			ignored[p] = struct{}{}
		}
	}
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, ok := ignored[p]; !ok {
			kept = append(kept, p)
		}
	}
	return kept, nil
}
