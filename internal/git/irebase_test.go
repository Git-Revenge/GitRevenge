package git

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// fourCommits returns a repo with base + a, b, c and the plan for a..c.
func fourCommits(t *testing.T) (*gittest.Repo, *Repo, string, []RebaseItem) {
	t.Helper()
	fx := gittest.New(t)
	base := fx.CommitFile("base.txt", "base\n", "base")
	fx.CommitFile("a.txt", "a\n", "add a")
	fx.CommitFile("b.txt", "b\n", "add b\n\nwith a body")
	fx.CommitFile("c.txt", "c\n", "add c")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	plan, err := repo.RebasePlan(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	return fx, repo, base, plan
}

func subjects(fx *gittest.Repo, n int) string {
	return fx.Git("log", "--format=%s", "-n", string(rune('0'+n)))
}

func TestRebasePlan(t *testing.T) {
	t.Parallel()
	_, _, _, plan := fourCommits(t)
	if len(plan) != 3 || plan[0].Subject != "add a" || plan[2].Subject != "add c" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan[1].Message != "add b\n\nwith a body" || plan[1].Action != "pick" {
		t.Fatalf("plan[1] = %+v", plan[1])
	}
}

func TestRebaseReorderDropReword(t *testing.T) {
	t.Parallel()
	fx, repo, base, plan := fourCommits(t)
	plan[0], plan[2] = plan[2], plan[0] // c, b, a
	plan[1].Action = "drop"             // drop b
	plan[2].Action = "reword"
	plan[2].Message = "add a (reworded)\n\n#42 keeps hash lines"

	if err := repo.InteractiveRebase(context.Background(), base, plan); err != nil {
		t.Fatal(err)
	}
	if got := subjects(fx, 3); got != "add a (reworded)\nadd c\nbase" {
		t.Fatalf("history = %q", got)
	}
	if msg := fx.Git("log", "-1", "--format=%B"); !strings.Contains(msg, "#42 keeps hash lines") {
		t.Fatalf("message = %q", msg)
	}
	if _, err := fx.TryGit("cat-file", "-e", "HEAD:b.txt"); err == nil {
		t.Fatal("dropped commit's file should be gone")
	}
}

func TestRebaseSquashAndFixup(t *testing.T) {
	t.Parallel()
	fx, repo, base, plan := fourCommits(t)
	plan[1].Action = "squash"
	plan[1].Message = "add a and b"
	plan[2].Action = "fixup"

	if err := repo.InteractiveRebase(context.Background(), base, plan); err != nil {
		t.Fatal(err)
	}
	if got := subjects(fx, 2); got != "add a and b\nbase" {
		t.Fatalf("history = %q", got)
	}
	for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
		if _, err := fx.TryGit("cat-file", "-e", "HEAD:"+f); err != nil {
			t.Fatalf("%s missing after squash", f)
		}
	}
}

func TestRebaseValidation(t *testing.T) {
	t.Parallel()
	_, repo, base, plan := fourCommits(t)
	ctx := context.Background()
	bad := append([]RebaseItem(nil), plan...)
	bad[0].Action = "squash"
	bad[0].Message = "x"
	if err := repo.InteractiveRebase(ctx, base, bad); err == nil {
		t.Fatal("squash first should be refused")
	}
	bad = append([]RebaseItem(nil), plan...)
	bad[0].Action = "reword"
	bad[0].Message = "  "
	if err := repo.InteractiveRebase(ctx, base, bad); err == nil {
		t.Fatal("reword without a message should be refused")
	}
	all := append([]RebaseItem(nil), plan...)
	for i := range all {
		all[i].Action = "drop"
	}
	if err := repo.InteractiveRebase(ctx, base, all); err == nil {
		t.Fatal("dropping everything should be refused")
	}
}

func TestRebasePlanRefusesMerges(t *testing.T) {
	t.Parallel()
	fx := gittest.WithMergeHistory(t)
	root := fx.Git("rev-list", "--max-parents=0", "HEAD")
	_, err := OpenRepo(newTestRunner(t), fx.Dir).RebasePlan(context.Background(), root)
	if !errors.Is(err, ErrMergesInRange) {
		t.Fatalf("err = %v", err)
	}
}

func TestRebaseConflictStopsInProgress(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	base := fx.CommitFile("f.txt", "1\n", "base")
	fx.CommitFile("f.txt", "2\n", "two")
	fx.CommitFile("f.txt", "3\n", "three")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	plan, _ := repo.RebasePlan(context.Background(), base)
	plan[0], plan[1] = plan[1], plan[0] // swapping dependent edits conflicts

	if err := repo.InteractiveRebase(context.Background(), base, plan); !errors.Is(err, ErrConflicts) {
		t.Fatalf("err = %v", err)
	}
	if op, _, _ := DetectOperation(fx.Path(".git")); op != Rebase {
		t.Fatalf("operation = %q", op)
	}
}
