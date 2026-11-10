package watch

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// recorder collects reported kinds.
type recorder struct{ ch chan Kind }

func (r recorder) onChange(k Kind) { r.ch <- k }

// waitFor returns once a report includes want, failing after a timeout.
func (r recorder) waitFor(t *testing.T, want Kind) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	var seen Kind
	for {
		select {
		case k := <-r.ch:
			seen |= k
			if seen.Has(want) {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for %v; saw %v", want.Names(), seen.Names())
		}
	}
}

// expectNothing fails if anything is reported for a while.
func (r recorder) expectNothing(t *testing.T) {
	t.Helper()
	select {
	case k := <-r.ch:
		t.Errorf("reported %v", k.Names())
	case <-time.After(500 * time.Millisecond):
	}
}

// drain discards reports until the watcher has been quiet for a while.
func (r recorder) drain() {
	for {
		select {
		case <-r.ch:
		case <-time.After(300 * time.Millisecond):
			return
		}
	}
}

func start(t *testing.T, fx *gittest.Repo, filter func([]string) []string) recorder {
	t.Helper()
	rec := recorder{ch: make(chan Kind, 64)}
	w, err := New(Options{
		Root:           fx.Dir,
		GitDir:         fx.Path(".git"),
		Debounce:       50 * time.Millisecond,
		FilterWorkTree: filter,
		OnChange:       rec.onChange,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	rec.drain()
	return rec
}

func TestReportsWorkTreeIndexAndHead(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	rec := start(t, fx, nil)

	fx.WriteFile("new.txt", "hello")
	rec.waitFor(t, WorkTree)
	rec.drain()

	fx.Git("add", "new.txt")
	rec.waitFor(t, Index)
	rec.drain()

	fx.Git("commit", "-q", "-m", "add new")
	rec.waitFor(t, Refs)
	rec.drain()

	fx.Branch("feature")
	rec.waitFor(t, Head)
}

func TestReportsMergeState(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "base\n", "base")
	fx.Branch("other")
	fx.CommitFile("f.txt", "theirs\n", "theirs")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "ours\n", "ours")
	rec := start(t, fx, nil)

	if err := fx.Merge("other"); err == nil {
		t.Fatal("expected conflict")
	}
	rec.waitFor(t, State)
}

func TestFilterDropsIgnoredFiles(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	ignoreAll := func([]string) []string { return nil }
	rec := start(t, fx, ignoreAll)

	fx.WriteFile("build/out.bin", "x")
	select {
	case k := <-rec.ch:
		if k.Has(WorkTree) {
			t.Fatalf("filtered work-tree change was reported: %v", k.Names())
		}
	case <-time.After(500 * time.Millisecond):
	}
}

func TestDebounceCoalescesBursts(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	rec := start(t, fx, nil)

	for i := range 50 {
		fx.WriteFile("burst/"+string(rune('a'+i%26))+".txt", "x")
	}
	rec.waitFor(t, WorkTree)
	rec.drain()
	// Not a hard guarantee of exactly one report, but a burst must not produce 50.
	if n := len(rec.ch); n > 2 {
		t.Fatalf("burst produced %d extra reports", n)
	}
}

func TestClassifyGitPath(t *testing.T) {
	tests := map[string]Kind{
		"HEAD":                   Head,
		"refs/heads/main":        Refs,
		"refs/heads/main.lock":   0,
		"packed-refs":            Refs,
		"index":                  Index,
		"index.lock":             0,
		"MERGE_HEAD":             State,
		"rebase-merge/done":      State,
		"config":                 Config,
		"objects/ab/cdef":        0,
		"logs/HEAD":              0,
		"FETCH_HEAD":             Refs,
		"refs/remotes/origin/x":  Refs,
		"worktrees/wt/HEAD":      Refs,
		"worktrees/wt":           Refs,
		"worktrees/wt/gitdir":    Refs,
		"worktrees/wt/locked":    Refs,
		"worktrees":              Refs,
		"worktrees/wt/refs/x":    0,
		"worktrees/wt/index":     0,
		"worktrees/wt/ORIG_HEAD": 0,
		"worktrees/wt/logs/HEAD": 0,
	}
	for path, want := range tests {
		if got := classifyGitPath(path); got != want {
			t.Errorf("classifyGitPath(%q) = %v, want %v", path, got.Names(), want.Names())
		}
	}
}

func TestClassifyCommonPath(t *testing.T) {
	tests := map[string]Kind{
		"HEAD":                  Refs,
		"refs/heads/x":          Refs,
		"packed-refs":           Refs,
		"FETCH_HEAD":            Refs,
		"config":                Config,
		"worktrees/other/HEAD":  Refs,
		"index":                 0,
		"ORIG_HEAD":             0,
		"MERGE_HEAD":            0,
		"rebase-merge/done":     0,
		"objects/ab/cdef":       0,
		"refs/heads/x.lock":     0,
		"worktrees/other/index": 0,
	}
	for path, want := range tests {
		if got := classifyCommonPath(path); got != want {
			t.Errorf("classifyCommonPath(%q) = %v, want %v", path, got.Names(), want.Names())
		}
	}
}

// A linked worktree's refs live in the git directory it shares with the main worktree.
func TestLinkedWorktreeReportsSharedRefs(t *testing.T) {
	fx := gittest.New(t)
	fx.Linear(1)
	dir := filepath.Join(t.TempDir(), "wt")
	fx.Git("worktree", "add", "-b", "feature", dir)
	rec := recorder{ch: make(chan Kind, 64)}
	w, err := New(Options{
		Root:      dir,
		GitDir:    fx.Path(".git/worktrees/wt"),
		CommonDir: fx.Path(".git"),
		Debounce:  50 * time.Millisecond,
		OnChange:  rec.onChange,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)

	fx.Git("branch", "made-elsewhere")
	rec.waitFor(t, Refs)
	rec.drain()

	// Its own HEAD still reports as a head change, not as a shared one.
	fx.Git("-C", dir, "checkout", "-q", "-b", "other")
	rec.waitFor(t, Head)
	rec.drain()

	// The main worktree's index is its own business.
	fx.WriteFile("main-only.txt", "x")
	fx.Git("add", "main-only.txt")
	rec.expectNothing(t)
}

// A small `git worktree add` can finish before Linux watches the new .git/worktrees, which
// failed about one run in three with two CPUs (#608) when only worktrees/<name>/HEAD counted.
func TestReportsAWorktreeAddedOutside(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "base")
	fx.Git("branch", "feature")
	rec := start(t, fx, nil)
	fx.Git("worktree", "add", filepath.Join(t.TempDir(), "wt"), "feature")
	rec.waitFor(t, Refs)
	rec.drain()

	// A second one goes into the folder the first made.
	fx.Git("worktree", "add", "-b", "second", filepath.Join(t.TempDir(), "wt2"))
	rec.waitFor(t, Refs)
}

func TestKindNames(t *testing.T) {
	if got := (Head | WorkTree).Names(); !slices.Equal(got, []string{"head", "worktree"}) {
		t.Fatalf("Names = %v", got)
	}
}

func TestIsLimit(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{syscall.ENOSPC, true},
		{syscall.EMFILE, true},
		{fmt.Errorf("watching: %w", syscall.ENOSPC), true},
		{syscall.ENOENT, false},
		{errors.New("no space left on device"), false},
		{nil, false},
	} {
		if got := IsLimit(tc.err); got != tc.want {
			t.Errorf("IsLimit(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
