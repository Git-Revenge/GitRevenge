// Package git runs the system git CLI (ADR-0002).
package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// baseArgs are prepended to every invocation so output is stable and machine-readable.
var baseArgs = []string{
	"-c", "core.quotepath=off",
	"-c", "color.ui=false",
	"-c", "core.pager=cat",
}

// Runner executes git commands with a fixed binary and a safe environment.
type Runner struct {
	bin string
	env []string
}

// NewRunner returns a Runner for the git binary at bin.
func NewRunner(bin string) *Runner {
	return &Runner{bin: bin, env: withPath(baseEnv(os.Environ()), toolDirs(bin)...)}
}

// toolDirs are directories git's helpers (git-lfs, credential helpers, hooks' tools) are
// likely installed beside. macOS apps launched from Finder get a minimal PATH without
// Homebrew, so findBinary can find git there while git itself can't find git-lfs.
func toolDirs(bin string) []string {
	dirs := []string{filepath.Dir(bin)}
	if runtime.GOOS == "darwin" {
		dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
	}
	return dirs
}

// withPath appends dirs missing from env's PATH, so the user's own PATH order still wins.
func withPath(env []string, dirs ...string) []string {
	out := slices.Clone(env)
	i := slices.IndexFunc(out, func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return strings.EqualFold(name, "PATH")
	})
	if i < 0 {
		out = append(out, "PATH=")
		i = len(out) - 1
	}
	name, value, _ := strings.Cut(out[i], "=")
	list := filepath.SplitList(value)
	for _, d := range dirs {
		if !slices.Contains(list, d) {
			list = append(list, d)
		}
	}
	out[i] = name + "=" + strings.Join(slices.DeleteFunc(list, func(d string) bool { return d == "" }), string(os.PathListSeparator))
	return out
}

// baseEnv strips variables that would redirect git to another repo and adds the
// settings every call needs. Git must never block on a terminal prompt (ADR-0004).
func baseEnv(environ []string) []string {
	env := make([]string, 0, len(environ)+3)
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(name) {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
			"GIT_NAMESPACE", "GIT_CEILING_DIRECTORIES", "LC_ALL", "LANG":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "LANG=C")
}

// Bin returns the path of the git binary.
func (r *Runner) Bin() string { return r.bin }

// Cmd describes a single git invocation.
type Cmd struct {
	// Dir is the working directory; usually the repository root.
	Dir string
	// Args are the git arguments, without the binary name.
	Args []string
	// Env holds extra KEY=VALUE pairs, applied after the runner's environment.
	Env []string
	// Stdin, if set, is fed to the process.
	Stdin io.Reader
	// ReadOnly sets GIT_OPTIONAL_LOCKS=0 so reads such as status never take the index
	// lock and race with the user's own git commands.
	ReadOnly bool
	// Progress, if set, receives stderr line by line as git writes it. git redraws progress
	// meters with carriage returns, so a carriage return ends a line too.
	Progress func(line string)
}

// Run executes c and returns its stdout.
func (r *Runner) Run(ctx context.Context, c Cmd) ([]byte, error) {
	var stdout bytes.Buffer
	if err := r.run(ctx, c, &stdout); err != nil {
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

// Output executes c and returns stdout with surrounding whitespace trimmed.
func (r *Runner) Output(ctx context.Context, c Cmd) (string, error) {
	out, err := r.Run(ctx, c)
	return strings.TrimSpace(string(out)), err
}

// Stream executes c and passes its stdout to consume as it is produced. If consume returns
// an error, the process is killed and that error is returned.
func (r *Runner) Stream(ctx context.Context, c Cmd, consume func(io.Reader) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := r.run(ctx, c, pw)
		pw.CloseWithError(err)
		done <- err
	}()

	consumeErr := consume(bufio.NewReaderSize(pr, 64*1024))
	if consumeErr != nil {
		cancel()
	}
	// Drain so the process can exit if consume stopped early without error.
	_, _ = io.Copy(io.Discard, pr)
	runErr := <-done
	if consumeErr != nil {
		return consumeErr
	}
	return runErr
}

// command builds the exec.Cmd for c without starting it. Stdin is set from c; the caller
// wires up stdout and stderr.
func (r *Runner) command(ctx context.Context, c Cmd) *exec.Cmd {
	args := append(append(make([]string, 0, len(baseArgs)+len(c.Args)), baseArgs...), c.Args...)
	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Dir = c.Dir
	cmd.Env = append(append(make([]string, 0, len(r.env)+len(c.Env)+1), r.env...), c.Env...)
	if c.ReadOnly {
		cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0")
	}
	cmd.Stdin = c.Stdin
	hideWindow(cmd)
	// After cancellation, don't wait forever on pipes held open by git's own children
	// (e.g. git-remote-https).
	cmd.WaitDelay = 2 * time.Second
	return cmd
}

func (r *Runner) run(ctx context.Context, c Cmd, stdout io.Writer) error {
	cmd := r.command(ctx, c)
	cmd.Stdout = stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if c.Progress != nil {
		cmd.Stderr = &progressWriter{buf: &stderr, fn: c.Progress}
	}

	err := cmd.Run()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	gerr := &Error{Args: c.Args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		gerr.ExitCode = exitErr.ExitCode()
	}
	return gerr
}

// Error is returned when git exits unsuccessfully.
type Error struct {
	Args     []string
	ExitCode int
	Stderr   string
	Err      error
}

// lfsMissingSigns are what git, the LFS hooks and the LFS filter print when git-lfs
// isn't installed or isn't on PATH.
var lfsMissingSigns = []string{
	"'git-lfs' was not found on your path",
	"git-lfs: command not found",
	"'git-lfs filter-process' failed",
	"'lfs' is not a git command",
}

// MissingLFS reports whether git failed because git-lfs isn't available.
func (e *Error) MissingLFS() bool {
	return slices.ContainsFunc(lfsMissingSigns, func(s string) bool { return strings.Contains(e.Stderr, s) })
}

// lfsInstallHint replaces git's raw output when git-lfs is missing.
const lfsInstallHint = "This repository uses Git LFS, but git-lfs isn't installed. Install it from https://git-lfs.com " +
	"(on macOS: brew install git-lfs; on Ubuntu or Debian: sudo apt install git-lfs; on RHEL: sudo dnf install git-lfs), " +
	"then restart Git Revenge."

func (e *Error) Error() string {
	if e.MissingLFS() {
		return lfsInstallHint
	}
	msg := e.Stderr
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg)
}

func (e *Error) Unwrap() error { return e.Err }

// maxStderr bounds the stderr kept for error messages from long-running commands.
const maxStderr = 64 << 10

// progressWriter splits stderr into lines (on \n or \r) for a callback, keeping the last
// maxStderr bytes for error reporting.
type progressWriter struct {
	buf     *bytes.Buffer
	fn      func(string)
	partial []byte
}

func (w *progressWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > maxStderr {
		keep := w.buf.Bytes()[max(0, w.buf.Len()+len(p)-maxStderr):]
		rest := append([]byte(nil), keep...)
		w.buf.Reset()
		w.buf.Write(rest)
	}
	w.buf.Write(p)
	for _, b := range p {
		if b == '\n' || b == '\r' {
			if len(w.partial) > 0 {
				w.fn(string(w.partial))
				w.partial = w.partial[:0]
			}
			continue
		}
		w.partial = append(w.partial, b)
	}
	return len(p), nil
}
