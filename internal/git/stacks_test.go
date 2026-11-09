package git

import (
	"context"
	"maps"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestStackParents(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "base")
	fx.Branch("one")
	fx.CommitFile("one.txt", "1", "one")
	fx.Branch("two")
	fx.CommitFile("two.txt", "1", "two")
	fx.Branch("three")
	fx.CommitFile("three.txt", "1", "three")
	fx.Git("branch", "side", "one") // on one's tip: not stacked
	fx.Checkout("main")
	fx.Branch("solo")
	fx.CommitFile("solo.txt", "1", "solo")
	fx.Checkout("main")
	fx.Git("branch", "merged", "main")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	got, err := repo.StackParents(context.Background(), "main", func(n string) bool { return n == "main" })
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"two": "one", "three": "two"}
	if !maps.Equal(got, want) {
		t.Fatalf("parents = %v, want %v", got, want)
	}

	// Once one merges into main, two sits directly on main: no longer stacked on one.
	fx.Git("merge", "-q", "--ff-only", "one")
	got, _ = repo.StackParents(context.Background(), "main", func(n string) bool { return n == "main" })
	if want := map[string]string{"three": "two"}; !maps.Equal(got, want) {
		t.Fatalf("after merging one: parents = %v, want %v", got, want)
	}
}
