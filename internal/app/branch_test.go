package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestBranchActions(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(2)
	tab, _ := a.OpenRepo(fx.Dir)
	do := func(action string, args ...string) error { return a.BranchAction(tab.Path, action, args) }

	if err := do("create", "feature", "", "true"); err != nil {
		t.Fatal(err)
	}
	if err := do("create", "feature"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate err = %v", err)
	}
	if err := do("create", "bad name"); err == nil || !strings.Contains(err.Error(), "isn't allowed") {
		t.Fatalf("invalid err = %v", err)
	}
	fx.CommitFile("f.txt", "f", "feature work")
	if err := do("checkout", "main"); err != nil {
		t.Fatal(err)
	}
	if err := do("delete", "feature"); err == nil || !strings.Contains(err.Error(), "aren't merged") {
		t.Fatalf("unmerged err = %v", err)
	}
	if err := do("delete", "feature", "true"); err != nil {
		t.Fatal(err)
	}
	if err := do("tag", "v1", "", "notes"); err != nil {
		t.Fatal(err)
	}
	if err := do("untag", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := do("explode"); err == nil {
		t.Fatal("unknown action accepted")
	}
}

func TestGitMessageStripsPrefixes(t *testing.T) {
	t.Parallel()
	got := gitMessage("error: Your local changes would be overwritten:\n\tmain.go\nfatal: Aborting\n").Error()
	if got != "Your local changes would be overwritten:\n\tmain.go\nAborting" {
		t.Fatalf("got %q", got)
	}
}
