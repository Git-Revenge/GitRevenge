package git

import (
	"context"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestReflog(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	shas := fx.Linear(3)
	fx.Git("reset", "--hard", shas[0])
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	got, err := repo.Reflog(context.Background(), "HEAD", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("HEAD reflog has %d entries, want 4: %+v", len(got), got)
	}
	newest := got[0]
	if newest.Selector != "HEAD@{0}" || newest.Commit != shas[0] || !strings.HasPrefix(newest.Action, "reset: moving to") || newest.Time == 0 {
		t.Fatalf("newest entry = %+v", newest)
	}
	if got[1].Commit != shas[2] || !strings.HasPrefix(got[1].Action, "commit") || got[1].Selector != "HEAD@{1}" {
		t.Fatalf("second entry = %+v", got[1])
	}

	branch, err := repo.Reflog(context.Background(), "refs/heads/main", 2)
	if err != nil || len(branch) != 2 || branch[0].Selector != "main@{0}" {
		t.Fatalf("main reflog = %+v, %v", branch, err)
	}
	if none, err := repo.Reflog(context.Background(), "refs/heads/nope", 5); err != nil || len(none) != 0 {
		t.Fatalf("missing ref = %+v, %v", none, err)
	}
}
