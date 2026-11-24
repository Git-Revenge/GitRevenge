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

// Context ends when the Repo closes. Work for the repository done in Go rather than by git,
// such as comparing two versions of a file, stops with it.
func (r *Repo) Context() context.Context { return r.open }

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

// Attributes returns the gitattributes the work tree sets on path, by name, for the names
// asked for. An attribute that isn't set, or is unspecified, is missing from the result; one
// set without a value is "set", and one unset is "unset", as git check-attr says them.
func (r *Repo) Attributes(ctx context.Context, path string, names ...string) (map[string]string, error) {
	all, err := r.AttributesOf(ctx, []string{path}, names...)
	return all[path], err
}

// AttributesOf is Attributes for several paths at once, by path. Every path has a map, empty
// when nothing is set on it.
func (r *Repo) AttributesOf(ctx context.Context, paths []string, names ...string) (map[string]map[string]string, error) {
	all := make(map[string]map[string]string, len(paths))
	var in strings.Builder
	for _, p := range paths {
		all[p] = map[string]string{}
		in.WriteString(p)
		in.WriteByte(0)
	}
	if len(paths) == 0 {
		return all, nil
	}
	out, err := r.readCmd(ctx, Cmd{Args: append([]string{"check-attr", "-z", "--stdin"}, names...), Stdin: strings.NewReader(in.String())})
	if err != nil {
		return all, err
	}
	// -z writes path, attribute and value, each ended by NUL.
	f := strings.Split(string(out), "\x00")
	for i := 0; i+2 < len(f); i += 3 {
		if m, ok := all[f[i]]; ok && f[i+2] != "unspecified" {
			m[f[i+1]] = f[i+2]
		}
	}
	return all, nil
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
