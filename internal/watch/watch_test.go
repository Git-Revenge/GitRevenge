package watch

import (
	"errors"
	"fmt"
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
		"HEAD":                  Head,
		"refs/heads/main":       Refs,
		"refs/heads/main.lock":  0,
		"packed-refs":           Refs,
		"index":                 Index,
		"index.lock":            0,
		"MERGE_HEAD":            State,
		"rebase-merge/done":     State,
		"config":                Config,
		"objects/ab/cdef":       0,
		"logs/HEAD":             0,
		"FETCH_HEAD":            Refs,
		"refs/remotes/origin/x": Refs,
	}
	for path, want := range tests {
		if got := classifyGitPath(path); got != want {
			t.Errorf("classifyGitPath(%q) = %v, want %v", path, got.Names(), want.Names())
		}
	}
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
