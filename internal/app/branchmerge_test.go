package app

import (
	"slices"
	"testing"

	"github.com/risadams/gitrevenge/internal/branchkind"
	"github.com/risadams/gitrevenge/internal/git"
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
