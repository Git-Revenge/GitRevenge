// Package gittest builds throwaway git repositories with the real git CLI for tests.
//
// Repositories are isolated from the user's global and system git config, and commit
// timestamps are deterministic, so the same sequence of operations always produces the
// same commit SHAs.
package gittest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// epoch is the timestamp of the first commit made by any Repo. Each commit advances one minute.
var epoch = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// Repo is a git repository in a temporary directory.
type Repo struct {
	t     testing.TB
	Dir   string
	env   []string
	clock time.Time
	bare  bool
}

// Git3Env names the environment variable that makes every test repository the way Git 3.0
// will make a new one by default: SHA-256 object IDs, reftable refs and main, with git
// refusing to find a bare repository it wasn't pointed at. Set it to 1.
const Git3Env = "GITREVENGE_TEST_GIT3"

// Git3 reports whether the tests run with Git 3.0's defaults (see Git3Env).
func Git3() bool { return os.Getenv(Git3Env) == "1" }

// git3Config is what Git3 adds to the tests' global config.
const git3Config = "[init]\n\tdefaultRefFormat = reftable\n\tdefaultObjectFormat = sha256\n\tdefaultBranch = main\n[safe]\n\tbareRepository = explicit\n"

// New initializes an empty repository on branch main.
func New(t testing.TB) *Repo {
	t.Helper()
	return newAt(t, t.TempDir(), "init", "-b", "main")
}

// NewBare initializes an empty bare repository, useful as a remote.
func NewBare(t testing.TB) *Repo {
	t.Helper()
	r := &Repo{t: t, Dir: t.TempDir(), env: isolatedEnv(t), clock: epoch}
	r.Git("init", "--bare", "-b", "main")
	r.bare = true
	r.configure()
	return r
}

// Clone clones src into a new temporary directory.
func Clone(t testing.TB, src *Repo) *Repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	r := &Repo{t: t, Dir: dir, env: isolatedEnv(t), clock: epoch}
	r.run(filepath.Dir(dir), "clone", src.Dir, dir)
	r.configure()
	return r
}

func newAt(t testing.TB, dir string, args ...string) *Repo {
	t.Helper()
	r := &Repo{t: t, Dir: dir, env: isolatedEnv(t), clock: epoch}
	r.Git(args...)
	r.configure()
	return r
}

// globalConfig is the global git config every test runs with, in place of the user's.
func globalConfig() string {
	// Allow local file:// submodules (blocked by default since git 2.38.1). This must live in
	// global config because submodule clones run in a child git process.
	config := "[protocol \"file\"]\n\tallow = always\n"
	if Git3() {
		config += git3Config
	}
	return config
}

// Main runs a package's tests with git's global and system config replaced by the tests' own.
// Call it from TestMain in any package whose tests run git:
//
//	func TestMain(m *testing.M) { gittest.Main(m) }
//
// A Repo's own git calls are isolated already, but the code under test runs git through its
// own Runner, which would otherwise read the config of whoever runs the tests: a global
// core.hooksPath, for one, stops a test's hooks from running. It's set for the whole process
// because tests run in parallel, where t.Setenv isn't allowed.
func Main(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "gitrevenge-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gittest:", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	global := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(global, []byte(globalConfig()), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "gittest:", err)
		return 1
	}
	for k, v := range map[string]string{"GIT_CONFIG_GLOBAL": global, "GIT_CONFIG_NOSYSTEM": "1"} {
		if err := os.Setenv(k, v); err != nil {
			fmt.Fprintln(os.Stderr, "gittest:", err)
			return 1
		}
	}
	return m.Run()
}

// isolatedEnv returns an environment that ignores the user's git config and never prompts.
func isolatedEnv(t testing.TB) []string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte(globalConfig()), 0o600); err != nil {
		t.Fatal(err)
	}
	env := make([]string, 0, len(os.Environ())+4)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	return append(env,
		"GIT_CONFIG_GLOBAL="+global,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	)
}

func (r *Repo) configure() {
	r.Git("config", "user.name", "Test User")
	r.Git("config", "user.email", "test@example.com")
	r.Git("config", "core.autocrlf", "false")
	r.Git("config", "commit.gpgsign", "false")
	r.Git("config", "tag.gpgsign", "false")
}

// Git runs git in the repository and returns trimmed stdout. The test fails on error.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return r.run(r.Dir, args...)
}

// TryGit runs git and returns trimmed stdout and any error, for commands expected to fail.
func (r *Repo) TryGit(args ...string) (string, error) {
	r.t.Helper()
	return r.exec(r.Dir, args...)
}

func (r *Repo) run(dir string, args ...string) string {
	r.t.Helper()
	out, err := r.exec(dir, args...)
	if err != nil {
		r.t.Fatal(err)
	}
	return out
}

func (r *Repo) exec(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	date := r.clock.Format(time.RFC3339)
	cmd.Env = slices.Concat(r.env, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date})
	if r.bare && dir == r.Dir {
		// Named outright: with safe.bareRepository=explicit, Git 3.0's default, git won't find
		// a bare repository from its working directory.
		cmd.Env = append(cmd.Env, "GIT_DIR="+dir)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Path returns the absolute path of a file inside the work tree.
func (r *Repo) Path(name string) string {
	return filepath.Join(r.Dir, filepath.FromSlash(name))
}

// WriteFile writes content to name (slash-separated, relative to the work tree), creating directories.
func (r *Repo) WriteFile(name, content string) {
	r.t.Helper()
	p := r.Path(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// RemoveFile deletes name from the work tree.
func (r *Repo) RemoveFile(name string) {
	r.t.Helper()
	if err := os.Remove(r.Path(name)); err != nil {
		r.t.Fatal(err)
	}
}

// Commit stages everything and commits with msg, returning the new commit SHA.
func (r *Repo) Commit(msg string) string {
	r.t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "--allow-empty", "-m", msg)
	r.tick()
	return r.Head()
}

// CommitFile writes a single file and commits it, returning the new commit SHA.
func (r *Repo) CommitFile(name, content, msg string) string {
	r.t.Helper()
	r.WriteFile(name, content)
	return r.Commit(msg)
}

// Linear makes n commits, each changing file.txt, and returns their SHAs oldest first.
func (r *Repo) Linear(n int) []string {
	r.t.Helper()
	shas := make([]string, n)
	for i := range n {
		shas[i] = r.CommitFile("file.txt", fmt.Sprintf("line %d\n", i), fmt.Sprintf("commit %d", i))
	}
	return shas
}

// Head returns the SHA of HEAD.
func (r *Repo) Head() string {
	r.t.Helper()
	return r.Git("rev-parse", "HEAD")
}

// Checkout switches to an existing branch or commit.
func (r *Repo) Checkout(ref string) {
	r.t.Helper()
	r.Git("checkout", "-q", ref)
}

// Branch creates branch name at HEAD and switches to it.
func (r *Repo) Branch(name string) {
	r.t.Helper()
	r.Git("checkout", "-q", "-b", name)
}

// Tag creates an annotated tag at HEAD.
func (r *Repo) Tag(name, msg string) {
	r.t.Helper()
	r.Git("tag", "-a", name, "-m", msg)
	r.tick()
}

// Merge merges branch into the current branch with a merge commit. It returns the error
// from git instead of failing, so tests can set up conflicts.
func (r *Repo) Merge(branch string) error {
	r.t.Helper()
	_, err := r.TryGit("merge", "--no-ff", "--no-edit", branch)
	r.tick()
	return err
}

// Stash stashes all changes, including untracked files.
func (r *Repo) Stash(msg string) {
	r.t.Helper()
	r.Git("stash", "push", "--include-untracked", "-m", msg)
	r.tick()
}

// AddRemote adds a remote pointing at other.
func (r *Repo) AddRemote(name string, other *Repo) {
	r.t.Helper()
	r.Git("remote", "add", name, other.Dir)
}

// AddSubmodule adds sub as a submodule at path and commits it.
func (r *Repo) AddSubmodule(sub *Repo, path string) string {
	r.t.Helper()
	r.Git("submodule", "add", "-q", sub.Dir, path)
	return r.Commit("add submodule " + path)
}

// SetupLFS installs git-lfs hooks locally and tracks pattern. The test is skipped if
// git-lfs is not installed.
func (r *Repo) SetupLFS(pattern string) {
	r.t.Helper()
	if _, err := r.TryGit("lfs", "version"); err != nil {
		r.t.Skip("git-lfs not installed")
	}
	r.Git("lfs", "install", "--local")
	r.Git("lfs", "track", pattern)
}

func (r *Repo) tick() {
	r.clock = r.clock.Add(time.Minute)
}
