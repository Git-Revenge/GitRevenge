package git

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestParseWorktrees(t *testing.T) {
	t.Parallel()
	out := "worktree /src/app\nHEAD aaa\nbranch refs/heads/main\n\nworktree /src/app-fix\nHEAD bbb\ndetached\nlocked reason\n\nworktree /tmp/gone\nHEAD ccc\nbranch refs/heads/old\nprunable gitdir file points to non-existent location\n"
	got := parseWorktrees(out, "/src/app-fix")
	if len(got) != 3 {
		t.Fatalf("got %d worktrees", len(got))
	}
	if w := got[0]; !w.Main || w.Branch != "main" || w.Current {
		t.Errorf("main = %+v", w)
	}
	if w := got[1]; w.Main || w.Branch != "" || !w.Locked || !w.Current || w.Head != "bbb" {
		t.Errorf("linked = %+v", w)
	}
	if w := got[2]; !w.Prunable || w.Branch != "old" {
		t.Errorf("prunable = %+v", w)
	}
}

func TestAddAndRemoveWorktree(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	wt := filepath.Join(t.TempDir(), "feature")

	if err := repo.AddWorktree(ctx, wt, "", "feature"); err != nil {
		t.Fatal(err)
	}
	list, err := repo.Worktrees(ctx)
	if err != nil || len(list) != 2 || list[1].Branch != "feature" || !list[0].Current {
		t.Fatalf("after add = %+v, %v", list, err)
	}
	if err := repo.RemoveWorktree(ctx, list[1].Path, false); err != nil {
		t.Fatal(err)
	}
	if list, _ := repo.Worktrees(ctx); len(list) != 1 {
		t.Fatalf("after remove = %+v", list)
	}
}
