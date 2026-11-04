package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestMergeTargets(t *testing.T) {
	local := func(s string, head bool) git.Ref { return git.Ref{Short: s, Kind: git.LocalBranch, Head: head} }
	remote := func(s string) git.Ref { return git.Ref{Short: s, Kind: git.RemoteBranch} }
	rules := branchkind.New(nil, nil)
	remotes := []string{"origin"}

	t.Run("prefers local Trunk, then its remote, then the checked-out branch", func(t *testing.T) {
		refs := []git.Ref{remote("origin/main"), local("work", true), local("main", false), local("topic", false), remote("origin/topic")}
		got := mergeTargets(refs, rules, remotes, "topic")
		if want := []string{"main", "origin/main", "work"}; !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("lists a branch once", func(t *testing.T) {
		got := mergeTargets([]git.Ref{local("main", true), local("topic", false)}, rules, remotes, "topic")
		if want := []string{"main"}; !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("never compares a branch with itself", func(t *testing.T) {
		got := mergeTargets([]git.Ref{local("main", false), remote("origin/main")}, rules, remotes, "main")
		if want := []string{"origin/main"}; !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("stops at a limit", func(t *testing.T) {
		many := branchkind.New(map[branchkind.Kind][]string{branchkind.Trunk: {"t*"}}, nil)
		var refs []git.Ref
		for _, s := range []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8"} {
			refs = append(refs, local(s, false))
		}
		if got := mergeTargets(refs, many, remotes, "topic"); len(got) != maxMergeTargets {
			t.Errorf("got %d targets", len(got))
		}
	})
}

// #464
func TestDeleteMergedBranches(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}

	// Each one is pushed, squash-merged unless it's "unmerged", then gone from the remote.
	for _, b := range []string{"done", "also-done", "release/x", "busy", "below", "unmerged"} {
		local.Git("checkout", "-q", "-b", b, "main")
		file := strings.ReplaceAll(b, "/", "-") + ".txt"
		local.CommitFile(file, b, b)
		local.Git("push", "-q", "-u", "origin", b)
		if b == "below" {
			// A branch that isn't merged builds on this one, so it stays.
			local.Git("checkout", "-q", "-b", "above")
			local.CommitFile("above.txt", "above", "above")
		}
		local.Git("checkout", "-q", "main")
		if b != "unmerged" {
			local.Git("merge", "-q", "--squash", b)
			local.Commit("squash " + b)
		}
		remote.Git("branch", "-D", b)
	}
	local.Git("worktree", "add", "-q", t.TempDir()+"/wt", "busy")
	if err := a.SetStackParent(tab.Path, "above", "below", false); err != nil {
		t.Fatal(err)
	}
	if err := a.Fetch(tab.Path, "origin"); err != nil {
		t.Fatal(err)
	}
	refsAfterReload(t, a, tab.Path, func(rs []git.Ref) bool {
		n := 0
		for _, r := range rs {
			if r.Gone {
				n++
			}
		}
		return n == 6
	})
	doneAt := strings.TrimSpace(local.Git("rev-parse", "done"))

	merged, err := a.MergedBranches(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{"done", "also-done", "release/x", "busy", "below"} {
		if merged[b] == "" {
			t.Errorf("%s should read as merged: %v", b, merged)
		}
	}
	if merged["unmerged"] != "" {
		t.Errorf("unmerged read as merged: %v", merged)
	}

	names := []string{"done", "also-done", "release/x", "busy", "below", "main"}
	safe, err := a.DeletableBranches(tab.Path, names)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"done", "also-done"}; !slices.Equal(safe, want) {
		t.Errorf("deletable: got %v, want %v", safe, want)
	}

	// A branch asked for that isn't merged stays, whatever the list thought.
	gone, err := a.DeleteMergedBranches(tab.Path, append(names, "unmerged"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(gone)
	if want := []string{"also-done", "done"}; !slices.Equal(gone, want) {
		t.Errorf("deleted: got %v, want %v", gone, want)
	}
	left := localBranches(local)
	for _, b := range []string{"release/x", "busy", "below", "above", "unmerged", "main"} {
		if !slices.Contains(left, b) {
			t.Errorf("%s was deleted: %v", b, left)
		}
	}
	for _, b := range gone {
		if slices.Contains(left, b) {
			t.Errorf("%s is still there", b)
		}
	}

	// One undo brings them all back, where they were.
	if err := a.Undo(tab.Path); err != nil {
		t.Fatal(err)
	}
	left = localBranches(local)
	for _, b := range gone {
		if !slices.Contains(left, b) {
			t.Errorf("%s didn't come back: %v", b, left)
		}
	}
	if got := strings.TrimSpace(local.Git("rev-parse", "done")); got != doneAt {
		t.Errorf("done came back at %s, was at %s", got, doneAt)
	}
	// Tracking too, or they'd no longer read as needing care.
	if got := strings.TrimSpace(local.Git("for-each-ref", "--format=%(upstream:short) %(upstream:track)", "refs/heads/done")); got != "origin/done [gone]" {
		t.Errorf("done came back tracking %q", got)
	}
}
