package git

import (
	"context"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestBranchCaseTwin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "first")
	head := fx.Head()
	// update-ref writes each name as given, whatever the filesystem would make of a branch.
	for _, ref := range []string{"refs/heads/foo", "refs/heads/feature/x", "refs/remotes/origin/Bar", "refs/remotes/origin/HEAD"} {
		fx.Git("update-ref", ref, head)
	}
	g := OpenRepo(newTestRunner(t), fx.Dir)

	cases := []struct{ name, except, want string }{
		{"Foo", "", "foo"},
		{"FOO", "", "foo"},
		{"foo", "", ""}, // the same name: git itself says it exists
		{"Feature/y", "", "feature/"},
		{"feature/Y", "", ""},
		{"feature/X", "", "feature/x"},
		{"bar", "", "origin/Bar"},
		{"head", "", ""},   // origin/HEAD isn't a branch
		{"Foo", "foo", ""}, // renaming foo to Foo
		{"brand-new", "", ""},
		{"foo/bar", "", ""},
	}
	for _, c := range cases {
		got, err := g.BranchCaseTwin(ctx, c.name, c.except)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("BranchCaseTwin(%q, except %q) = %q, want %q", c.name, c.except, got, c.want)
		}
	}
}

func TestMarkCaseTwins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	status := func(files ...FileStatus) *Status { return &Status{Files: files} }
	untracked := func(p string) FileStatus { return FileStatus{Path: p, Index: "?", WorkTree: "?", Untracked: true} }
	// The new files needn't be on disk: only the index and HEAD are looked at.
	tracked := func(t *testing.T) (*gittest.Repo, *Repo) {
		fx := gittest.New(t)
		fx.CommitFile("sub/clash.txt", "one\n", "first")
		fx.Git("config", "core.ignorecase", "false")
		return fx, OpenRepo(newTestRunner(t), fx.Dir)
	}

	t.Run("flags new files that clash by case", func(t *testing.T) {
		_, g := tracked(t)
		st := status(untracked("sub/Clash.txt"), untracked("SUB/other.txt"), untracked("sub/another.txt"), untracked("new/clash.txt"))
		g.MarkCaseClashes(ctx, st)
		for i, want := range []bool{true, true, false, false} {
			if st.Files[i].CaseTwin != want {
				t.Errorf("%s: CaseTwin = %v, want %v", st.Files[i].Path, st.Files[i].CaseTwin, want)
			}
			if st.Files[i].CaseClash {
				t.Errorf("%s: flagged as a clash on a case-sensitive filesystem", st.Files[i].Path)
			}
		}
	})

	t.Run("flags a staged twin but not the file it twins", func(t *testing.T) {
		fx, g := tracked(t)
		blob := fx.Git("rev-parse", ":sub/clash.txt")
		fx.Git("update-index", "--add", "--cacheinfo", "100644,"+trimNL(blob)+",sub/CLASH.txt")
		st := status(FileStatus{Path: "sub/CLASH.txt", Index: "A", WorkTree: "."}, FileStatus{Path: "sub/clash.txt", Index: ".", WorkTree: "M"})
		g.MarkCaseClashes(ctx, st)
		if !st.Files[0].CaseTwin {
			t.Error("the staged twin wasn't flagged")
		}
		if st.Files[1].CaseTwin {
			t.Error("the tracked file was flagged")
		}
	})

	t.Run("leaves it to the clash check where the filesystem folds case", func(t *testing.T) {
		fx, g := tracked(t)
		fx.Git("config", "core.ignorecase", "true")
		st := status(untracked("sub/Clash.txt"))
		g.MarkCaseClashes(ctx, st)
		if st.Files[0].CaseTwin {
			t.Error("flagged with core.ignorecase on")
		}
	})

	t.Run("works before the first commit", func(t *testing.T) {
		fx := gittest.New(t)
		fx.Git("config", "core.ignorecase", "false")
		fx.WriteFile("a.txt", "a\n")
		fx.Git("add", "a.txt")
		g := OpenRepo(newTestRunner(t), fx.Dir)
		st := status(untracked("A.txt"), FileStatus{Path: "a.txt", Index: "A", WorkTree: "."})
		g.MarkCaseClashes(ctx, st)
		// Neither is tracked yet, but they're twins of each other.
		if !st.Files[0].CaseTwin || !st.Files[1].CaseTwin {
			t.Errorf("not both flagged: %+v", st.Files)
		}
	})
}
