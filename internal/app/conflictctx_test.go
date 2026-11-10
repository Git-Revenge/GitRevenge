package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestConflictContextOriginAndMergeTool(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("f.txt", "one\ntwo\n", "Add the file")
	fx.Branch("topic")
	fx.CommitFile("f.txt", "one\ntwo theirs\n", "Change two on topic")
	fx.CommitFile("clean.txt", "clean\n", "Add a clean file")
	fx.Checkout("main")
	fx.CommitFile("f.txt", "one\ntwo ours\n", "Change two on main")
	if _, err := fx.TryGit("merge", "topic"); err == nil {
		t.Fatal("expected a merge conflict")
	}
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx, err := a.ConflictContext(tab.Path)
	if err != nil || ctx.MergeTool != "" || len(ctx.Clean) != 1 || ctx.Clean[0] != "clean.txt" {
		t.Fatalf("context = %+v, %v", ctx, err)
	}
	o, err := a.ConflictOrigin(tab.Path, "f.txt", "theirs", 2, 2)
	if err != nil || o.Subject != "Change two on topic" {
		t.Fatalf("origin = %+v, %v", o, err)
	}
	cf, err := a.ConflictFile(tab.Path, "f.txt")
	if err != nil || !strings.Contains(cf.Diff3, "||||||| base\ntwo\n") {
		t.Fatalf("diff3 = %q, %v", cf.Diff3, err)
	}

	// With no tool set, the app says so in words.
	if err := a.OpenMergeTool(tab.Path, "f.txt"); err == nil || !strings.Contains(err.Error(), "No merge tool is set") {
		t.Fatalf("err = %v", err)
	}
	// A tool that fails leaves the file conflicted, and says it wasn't resolved.
	fx.Git("config", "merge.tool", "nope")
	fx.Git("config", "mergetool.nope.cmd", "false")
	fx.Git("config", "mergetool.nope.trustExitCode", "true")
	if err := a.OpenMergeTool(tab.Path, "f.txt"); err == nil || !strings.Contains(err.Error(), "closed without resolving f.txt") {
		t.Fatalf("err = %v", err)
	}
	if st, _ := a.RepoState(tab.Path); len(st.Conflicts) != 1 {
		t.Fatalf("after a failed tool: %+v", st)
	}
	// One that succeeds resolves it.
	fx.Git("config", "mergetool.nope.cmd", `cp -- "$LOCAL" "$MERGED"`)
	if err := a.OpenMergeTool(tab.Path, "f.txt"); err != nil {
		t.Fatal(err)
	}
	if st, _ := a.RepoState(tab.Path); len(st.Conflicts) != 0 {
		t.Fatalf("after the tool resolved it: %+v", st)
	}
}
