package app

import (
	"os"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestGroupCRUD(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("src/a.txt", "a", "base")

	if err := a.GroupAction("create", []string{"work"}); err != nil {
		t.Fatal(err)
	}
	if err := a.GroupAction("create", []string{"work"}); err == nil {
		t.Fatal("duplicate group accepted")
	}
	// Adding a subfolder stores the repository root.
	if err := a.GroupAction("add", []string{"work", fx.Path("src")}); err != nil {
		t.Fatal(err)
	}
	if err := a.GroupAction("add", []string{"work", t.TempDir()}); err == nil || !strings.Contains(err.Error(), "isn't a git repository") {
		t.Fatalf("non-repo err = %v", err)
	}
	if err := a.GroupAction("rename", []string{"work", "client"}); err != nil {
		t.Fatal(err)
	}
	groups, _ := a.Groups()
	if len(groups) != 1 || groups[0].Name != "client" || len(groups[0].Paths) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	if err := a.GroupAction("remove", []string{"client", groups[0].Paths[0]}); err != nil {
		t.Fatal(err)
	}
	if err := a.GroupAction("delete", []string{"client"}); err != nil {
		t.Fatal(err)
	}
	if groups, _ := a.Groups(); len(groups) != 0 {
		t.Fatalf("groups after delete = %+v", groups)
	}
}

func TestGroupDashboardAndBatch(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)

	// Repo 1: behind its remote (fast-forwardable).
	behind, remote := gittest.WithRemote(t, 1)
	other := gittest.Clone(t, remote)
	other.CommitFile("o.txt", "o", "upstream work")
	other.Git("push", "-q", "origin", "main")
	// Repo 2: diverged from its remote (can't fast-forward).
	diverged, remote2 := gittest.WithRemote(t, 1)
	other2 := gittest.Clone(t, remote2)
	other2.CommitFile("x.txt", "x", "theirs")
	other2.Git("push", "-q", "origin", "main")
	diverged.CommitFile("y.txt", "y", "mine")
	diverged.WriteFile("dirty.txt", "local")
	// Repo 3: folder no longer exists.
	gone := gittest.New(t)

	_ = a.GroupAction("create", []string{"g"})
	for _, p := range []string{behind.Dir, diverged.Dir, gone.Dir} {
		if err := a.GroupAction("add", []string{"g", p}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(gone.Dir); err != nil {
		t.Fatal(err)
	}

	results, err := a.GroupBatch("g", "fetch")
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].OK || !results[1].OK || results[2].OK {
		t.Fatalf("fetch results = %+v", results)
	}

	// A repo without remotes is skipped, not failed.
	_ = a.GroupAction("create", []string{"local"})
	_ = a.GroupAction("add", []string{"local", gittest.New(t).Dir})
	if res, _ := a.GroupBatch("local", "pull"); !res[0].OK || !res[0].Skipped {
		t.Fatalf("no-remote repo = %+v", res[0])
	}

	rows, err := a.GroupDashboard("g")
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Behind != 1 || rows[0].Branch != "main" || rows[0].LastFetch == 0 {
		t.Fatalf("behind row = %+v", rows[0])
	}
	if rows[1].Ahead != 1 || rows[1].Behind != 1 || rows[1].Changes != 1 {
		t.Fatalf("diverged row = %+v", rows[1])
	}
	if rows[2].Exists || rows[2].Error == "" {
		t.Fatalf("missing row = %+v", rows[2])
	}

	results, err = a.GroupBatch("g", "pull")
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].OK || behind.Head() != other.Head() {
		t.Fatalf("fast-forwardable repo should pull: %+v", results[0])
	}
	if results[1].OK || !strings.Contains(results[1].Message, "fast-forward") {
		t.Fatalf("diverged repo should be reported, not merged: %+v", results[1])
	}
	if strings.Contains(diverged.Git("log", "-1", "--format=%s"), "Merge") {
		t.Fatal("batch pull must never create a merge")
	}
}
