package git

import (
	"context"
	"testing"

	"github.com/risadams/gitrevenge/internal/gittest"
)

// branched makes a repository with main, and a topic branch off it with these files,
// one commit each. It ends up on main.
func branched(t *testing.T, files ...string) (*gittest.Repo, *Repo) {
	t.Helper()
	fx := gittest.New(t)
	fx.Linear(1)
	fx.Git("branch", "-M", "main")
	fx.Branch("topic")
	for _, f := range files {
		fx.CommitFile(f, f+"\n", "add "+f)
	}
	fx.Checkout("main")
	fx.CommitFile("other.txt", "other\n", "main moves on")
	return fx, OpenRepo(newTestRunner(t), fx.Dir)
}

func TestBranchMergeState(t *testing.T) {
	ctx := context.Background()
	state := func(g *Repo) BranchMerge { return g.BranchMergeState(ctx, "topic", []string{"main"}) }

	t.Run("a real merge", func(t *testing.T) {
		fx, g := branched(t, "a.txt")
		fx.Git("merge", "-q", "--no-ff", "-m", "merge", "topic")
		if got := state(g); got.How != MergedIn || got.Unmerged != 0 || got.Target != "main" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a squash of one commit", func(t *testing.T) {
		fx, g := branched(t, "a.txt")
		fx.Git("merge", "-q", "--squash", "topic")
		fx.Commit("squashed")
		if got := state(g); got.How != SquashedIn || got.Unmerged != 0 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a squash of several commits", func(t *testing.T) {
		fx, g := branched(t, "a.txt", "b.txt", "c.txt")
		fx.Git("merge", "-q", "--squash", "topic")
		fx.Commit("squashed")
		if got := state(g); got.How != SquashedIn || got.Unmerged != 0 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("rebased onto the target", func(t *testing.T) {
		fx, g := branched(t, "a.txt", "b.txt")
		fx.Git("cherry-pick", "main..topic")
		if got := state(g); got.How != SquashedIn || got.Unmerged != 0 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("not merged at all", func(t *testing.T) {
		_, g := branched(t, "a.txt", "b.txt")
		if got := state(g); got.How != "" || got.Unmerged != 2 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("only partly on the target", func(t *testing.T) {
		fx, g := branched(t, "a.txt", "b.txt")
		fx.Git("cherry-pick", "topic~1")
		if got := state(g); got.How != "" || got.Unmerged != 1 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a squash with more on the branch afterwards", func(t *testing.T) {
		fx, g := branched(t, "a.txt", "b.txt")
		fx.Git("merge", "-q", "--squash", "topic")
		fx.Commit("squashed")
		fx.Checkout("topic")
		fx.CommitFile("later.txt", "later\n", "more work")
		fx.Checkout("main")
		if got := state(g); got.How != "" || got.Unmerged == 0 {
			t.Errorf("got %+v: the later commit would be lost", got)
		}
	})

	t.Run("an empty change is not a squash", func(t *testing.T) {
		fx, g := branched(t, "a.txt", "b.txt")
		// The branch adds a file and takes it away again, so it changes nothing as a whole.
		fx.Checkout("topic")
		fx.RemoveFile("a.txt")
		fx.RemoveFile("b.txt")
		fx.Commit("take it back")
		fx.Checkout("main")
		if got := state(g); got.How != "" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("finds the target holding the work", func(t *testing.T) {
		fx, g := branched(t, "a.txt")
		fx.Git("branch", "release", "topic")
		got := g.BranchMergeState(ctx, "topic", []string{"main", "release"})
		if got.How != MergedIn || got.Target != "release" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("reports against the first target otherwise", func(t *testing.T) {
		fx, g := branched(t, "a.txt")
		fx.Git("branch", "other", "main")
		got := g.BranchMergeState(ctx, "topic", []string{"main", "other"})
		if got.How != "" || got.Target != "main" || got.Unmerged != 1 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("nothing to compare with", func(t *testing.T) {
		_, g := branched(t, "a.txt")
		if got := g.BranchMergeState(ctx, "topic", nil); got != (BranchMerge{}) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("a target that doesn't exist isn't a merge", func(t *testing.T) {
		_, g := branched(t, "a.txt")
		if got := g.BranchMergeState(ctx, "topic", []string{"nope"}); got.How != "" {
			t.Errorf("got %+v", got)
		}
	})
}
