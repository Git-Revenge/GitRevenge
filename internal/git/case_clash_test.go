package git

import (
	"context"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// clashing makes a repository whose index holds two names differing only by case, whatever
// the filesystem under the test can do: update-index needs no such file on disk.
func clashing(t *testing.T) (*gittest.Repo, *Repo) {
	t.Helper()
	fx := gittest.New(t)
	fx.WriteFile("sub/plain.txt", "one\n")
	fx.WriteFile("sub/Clash.txt", "one\n")
	fx.Git("add", ".")
	blob := fx.Git("rev-parse", ":sub/Clash.txt")
	fx.Git("update-index", "--add", "--cacheinfo", "100644,"+trimNL(blob)+",sub/clash.txt")
	// Not fx.Commit: its add -A would drop the name that isn't on disk on a case-sensitive
	// filesystem.
	fx.Git("commit", "-m", "two names")
	return fx, OpenRepo(newTestRunner(t), fx.Dir)
}

func trimNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func TestMarkCaseClashes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	status := func(files ...FileStatus) *Status { return &Status{Files: files} }

	t.Run("flags a changed file with a case twin", func(t *testing.T) {
		fx, g := clashing(t)
		fx.Git("config", "core.ignorecase", "true")
		st := status(FileStatus{Path: "sub/Clash.txt", Index: ".", WorkTree: "M"}, FileStatus{Path: "sub/plain.txt", Index: ".", WorkTree: "M"})
		g.MarkCaseClashes(ctx, st)
		if !st.Files[0].CaseClash {
			t.Error("the clashing file wasn't flagged")
		}
		if st.Files[1].CaseClash {
			t.Error("the plain file was flagged")
		}
	})

	t.Run("leaves it alone where the filesystem tells them apart", func(t *testing.T) {
		fx, g := clashing(t)
		fx.Git("config", "core.ignorecase", "false")
		st := status(FileStatus{Path: "sub/Clash.txt", Index: ".", WorkTree: "M"})
		g.MarkCaseClashes(ctx, st)
		if st.Files[0].CaseClash {
			t.Error("flagged with core.ignorecase off")
		}
	})

	t.Run("ignores an untracked file", func(t *testing.T) {
		fx, g := clashing(t)
		fx.Git("config", "core.ignorecase", "true")
		st := status(FileStatus{Path: "sub/CLASH.TXT", Index: "?", WorkTree: "?", Untracked: true})
		g.MarkCaseClashes(ctx, st)
		if st.Files[0].CaseClash {
			t.Error("flagged an untracked file")
		}
	})

	// The index lists a conflicted file once per stage, which isn't two names.
	t.Run("leaves a conflicted file alone", func(t *testing.T) {
		fx := gittest.New(t)
		fx.Git("config", "core.ignorecase", "true")
		fx.CommitFile("sub/file.txt", "base\n", "base")
		trunk := trimNL(fx.Git("branch", "--show-current"))
		fx.Branch("topic")
		fx.CommitFile("sub/file.txt", "topic\n", "topic")
		fx.Checkout(trunk)
		fx.CommitFile("sub/file.txt", "trunk\n", "trunk")
		if err := fx.Merge("topic"); err == nil {
			t.Fatal("the merge didn't stop on a conflict")
		}
		g := OpenRepo(newTestRunner(t), fx.Dir)
		st, err := g.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(st.Files) == 0 {
			t.Fatal("the status has no files")
		}
		g.MarkCaseClashes(ctx, &st)
		for _, f := range st.Files {
			if f.CaseClash {
				t.Errorf("%s was flagged", f.Path)
			}
		}
	})

	t.Run("has nothing to do with no changes", func(t *testing.T) {
		fx, g := clashing(t)
		fx.Git("config", "core.ignorecase", "true")
		st := status()
		g.MarkCaseClashes(ctx, st)
		if len(st.Files) != 0 {
			t.Error("files appeared")
		}
	})
}
