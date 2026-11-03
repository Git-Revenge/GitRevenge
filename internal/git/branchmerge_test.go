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

func TestMergedBranches(t *testing.T) {
	ctx := context.Background()
	fx, g := branched(t, "a1.txt", "a2.txt")
	fx.Git("branch", "bottom", "topic")
	fx.Checkout("topic")
	fx.Git("branch", "-m", "top")
	fx.CommitFile("b1.txt", "b1\n", "add b1")
	fx.CommitFile("b2.txt", "b2\n", "add b2")
	fx.Checkout("main")
	fx.Git("merge", "-q", "--squash", "top")
	fx.Commit("squash the stack")
	fx.Git("checkout", "-q", "-b", "unmerged", "main")
	fx.CommitFile("own.txt", "own\n", "unmerged work")
	fx.Git("branch", "unmerged-below")
	fx.CommitFile("own2.txt", "own2\n", "more unmerged work")
	fx.Checkout("main")

	got := g.MergedBranches(ctx, []string{"bottom", "top", "unmerged", "unmerged-below"}, []string{"main"})
	for _, name := range []string{"bottom", "top"} {
		if got[name].How == "" || got[name].Target != "main" {
			t.Errorf("%s: got %+v", name, got[name])
		}
	}
	for _, name := range []string{"unmerged", "unmerged-below"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s read as merged: %+v", name, got[name])
		}
	}
	if len(g.MergedBranches(ctx, []string{"top"}, nil)) != 0 {
		t.Error("something merged with no targets")
	}
}

func TestMergedBranchesStopsWhenOutOfTime(t *testing.T) {
	_, g := branched(t, "a.txt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := g.MergedBranches(ctx, []string{"topic"}, []string{"main"}); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
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

	// #461: a stack, each branch of several commits, squash-merged a branch at a time.
	t.Run("a stack squash-merged a branch at a time", func(t *testing.T) {
		fx, g := branched(t, "a1.txt", "a2.txt")
		fx.Git("branch", "bottom", "topic")
		fx.Checkout("topic")
		fx.Git("branch", "-m", "middle")
		fx.CommitFile("b1.txt", "b1\n", "add b1")
		fx.CommitFile("b2.txt", "b2\n", "add b2")
		fx.Branch("top")
		fx.CommitFile("c1.txt", "c1\n", "add c1")
		fx.CommitFile("c2.txt", "c2\n", "add c2")
		fx.Checkout("main")
		for _, b := range []string{"bottom", "middle", "top"} {
			fx.Git("merge", "-q", "--squash", b)
			fx.Commit("squash " + b)
		}
		for _, b := range []string{"bottom", "middle", "top"} {
			if got := g.BranchMergeState(ctx, b, []string{"main"}); got.How == "" || got.Unmerged != 0 {
				t.Errorf("%s: got %+v", b, got)
			}
		}
	})

	t.Run("a lower branch counts through the merged branch above it, even if it would conflict", func(t *testing.T) {
		fx, g := branched(t, "a.txt")
		fx.Checkout("topic")
		fx.CommitFile("shared.txt", "first draft\n", "start shared")
		fx.Git("branch", "bottom")
		fx.CommitFile("shared.txt", "final\n", "finish shared")
		fx.Checkout("main")
		fx.Git("merge", "-q", "--squash", "topic")
		fx.Commit("squash the top")
		// Alone, the lower branch adds a different shared.txt, so merging it would conflict.
		if _, err := fx.TryGit("merge-tree", "--write-tree", "main", "bottom"); err == nil {
			t.Fatal("the fixture should conflict")
		}
		if got := g.BranchMergeState(ctx, "bottom", []string{"main"}); got.How == "" || got.Unmerged != 0 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("work on top of a merged stack still counts as unmerged", func(t *testing.T) {
		fx, g := branched(t, "a.txt", "b.txt")
		fx.Git("merge", "-q", "--squash", "topic")
		fx.Commit("squashed")
		fx.Git("checkout", "-q", "-b", "above", "topic")
		fx.CommitFile("more.txt", "more\n", "more work")
		fx.Checkout("main")
		if got := g.BranchMergeState(ctx, "above", []string{"main"}); got.How != "" {
			t.Errorf("got %+v: more.txt would be lost", got)
		}
	})

	t.Run("a branch under an unmerged one stays unmerged", func(t *testing.T) {
		fx, g := branched(t, "a.txt", "b.txt")
		fx.Git("branch", "above", "topic")
		if got := g.BranchMergeState(ctx, "topic", []string{"main"}); got.How != "" {
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

func TestBranchesTogether(t *testing.T) {
	ctx := context.Background()
	fx, g := branched(t, "a.txt")
	commit := fx.Git("rev-parse", "main")[:40]
	if err := g.CreateBranchesAt(ctx, map[string]string{"v1.2": commit, "fix/x": commit}); err != nil {
		t.Fatal(err)
	}
	// Names holding dots and slashes, whose remote branch needn't exist.
	want := BranchTracking{Remote: "origin", Merge: "refs/heads/v1.2"}
	if err := g.SetBranchTracking(ctx, "v1.2", want); err != nil {
		t.Fatal(err)
	}
	if err := g.SetBranchTracking(ctx, "fix/x", BranchTracking{}); err != nil {
		t.Fatal(err)
	}
	all := g.BranchTrackings(ctx)
	if all["v1.2"] != want {
		t.Errorf("got %+v in %+v", all["v1.2"], all)
	}
	if _, ok := all["fix/x"]; ok {
		t.Errorf("fix/x has tracking: %+v", all)
	}

	// All or none: one that exists already stops the others.
	if err := g.CreateBranchesAt(ctx, map[string]string{"new": commit, "v1.2": commit}); err == nil {
		t.Error("recreating a branch should fail")
	}
	names, err := g.BranchNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if names["new"] || !names["v1.2"] || !names["fix/x"] {
		t.Errorf("got %v", names)
	}

	// The checked-out branch stays, and the rest still go.
	if err := g.DeleteBranches(ctx, []string{"main", "v1.2", "fix/x"}); err == nil {
		t.Error("deleting the checked-out branch should say so")
	}
	names, _ = g.BranchNames(ctx)
	if !names["main"] || names["v1.2"] || names["fix/x"] {
		t.Errorf("got %v", names)
	}
}
