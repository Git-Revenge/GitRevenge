package git

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func newTestRunner(t *testing.T) *Runner {
	t.Helper()
	bin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	return NewRunner(bin)
}

func TestRunOutput(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	repo := gittest.New(t)
	sha := repo.Linear(1)[0]

	got, err := r.Output(context.Background(), Cmd{Dir: repo.Dir, Args: []string{"rev-parse", "HEAD"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != sha {
		t.Fatalf("rev-parse HEAD = %q, want %q", got, sha)
	}
}

func TestRunErrorCarriesStderrAndExitCode(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	repo := gittest.New(t)

	_, err := r.Run(context.Background(), Cmd{Dir: repo.Dir, Args: []string{"rev-parse", "does-not-exist"}})
	var gerr *Error
	if !errors.As(err, &gerr) {
		t.Fatalf("err = %v, want *Error", err)
	}
	if gerr.ExitCode == 0 || !strings.Contains(gerr.Stderr, "does-not-exist") {
		t.Fatalf("unexpected error details: exit=%d stderr=%q", gerr.ExitCode, gerr.Stderr)
	}
}

func TestStreamReadsLines(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	repo := gittest.New(t)
	repo.Linear(50)

	var lines int
	err := r.Stream(context.Background(), Cmd{Dir: repo.Dir, Args: []string{"log", "--format=%H"}, ReadOnly: true},
		func(rd io.Reader) error {
			sc := bufio.NewScanner(rd)
			for sc.Scan() {
				lines++
			}
			return sc.Err()
		})
	if err != nil {
		t.Fatal(err)
	}
	if lines != 50 {
		t.Fatalf("got %d lines, want 50", lines)
	}
}

func TestStreamStopsOnConsumerError(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	repo := gittest.New(t)
	repo.Linear(20)

	stop := errors.New("stop")
	err := r.Stream(context.Background(), Cmd{Dir: repo.Dir, Args: []string{"log", "--format=%H"}},
		func(io.Reader) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, want consumer error", err)
	}
}

func TestRunCanceled(t *testing.T) {
	t.Parallel()
	r := newTestRunner(t)
	repo := gittest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Run(ctx, Cmd{Dir: repo.Dir, Args: []string{"status"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestBaseEnv(t *testing.T) {
	t.Parallel()
	env := baseEnv([]string{"PATH=/bin", "GIT_DIR=/elsewhere", "git_work_tree=/x", "LC_ALL=fr_FR", "HOME=/home/u"})
	for _, banned := range []string{"GIT_DIR=/elsewhere", "git_work_tree=/x", "LC_ALL=fr_FR"} {
		if slices.Contains(env, banned) {
			t.Errorf("env still contains %q", banned)
		}
	}
	for _, want := range []string{"PATH=/bin", "HOME=/home/u", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"} {
		if !slices.Contains(env, want) {
			t.Errorf("env missing %q", want)
		}
	}
}

func TestWithPath(t *testing.T) {
	t.Parallel()
	sep := string(os.PathListSeparator)
	cases := []struct {
		name string
		env  []string
		want string
	}{
		{"appends missing dirs after the user's", []string{"PATH=/usr/bin" + sep + "/bin"}, "PATH=/usr/bin" + sep + "/bin" + sep + "/opt/homebrew/bin"},
		{"keeps dirs already present", []string{"PATH=/opt/homebrew/bin" + sep + "/usr/bin"}, "PATH=/opt/homebrew/bin" + sep + "/usr/bin"},
		{"matches the name case-insensitively", []string{"Path=/usr/bin"}, "Path=/usr/bin" + sep + "/opt/homebrew/bin"},
		{"adds PATH when unset", []string{"HOME=/home/u"}, "PATH=/usr/bin" + sep + "/opt/homebrew/bin"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := withPath(c.env, "/usr/bin", "/opt/homebrew/bin")
			if !slices.Contains(env, c.want) {
				t.Errorf("env = %q, want it to contain %q", env, c.want)
			}
		})
	}
}

func TestMissingLFSError(t *testing.T) {
	t.Parallel()
	for _, stderr := range []string{
		"This repository is configured for Git LFS but 'git-lfs' was not found on your path. If you no longer wish to use Git LFS, remove this hook by deleting the 'post-checkout' file in the hooks directory (set by 'core.hookspath'; usually '.git/hooks').",
		"git-lfs filter-process: git-lfs: command not found\nfatal: the remote end hung up unexpectedly",
		"git: 'lfs' is not a git command. See 'git --help'.",
	} {
		err := &Error{Args: []string{"checkout", "main"}, Stderr: stderr}
		if !err.MissingLFS() || err.Error() != lfsInstallHint {
			t.Errorf("stderr %q: MissingLFS = %v, Error = %q", stderr, err.MissingLFS(), err.Error())
		}
	}
	other := &Error{Args: []string{"checkout", "nope"}, Stderr: "error: pathspec 'nope' did not match"}
	if other.MissingLFS() || !strings.Contains(other.Error(), "pathspec") {
		t.Errorf("unrelated error rewritten: %q", other.Error())
	}
}
