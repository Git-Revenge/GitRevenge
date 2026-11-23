package app

import (
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/links"
)

func TestWebLinksForRemotesAndBranches(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Commit("one")
	fx.Git("remote", "add", "origin", "git@github.com:ris/GitRevenge.git")
	fx.Git("remote", "add", "upstream", "https://github.com/Git-Revenge/GitRevenge.git")
	fx.Git("remote", "add", "work", "git@git.acme.com:team/app.git")
	fx.Git("remote", "add", "nas", "git@nas.local:srv/app.git")
	fx.Git("remote", "add", "folder", t.TempDir())
	for _, ref := range []string{"refs/remotes/origin/main", "refs/remotes/origin/fix/a+b#1", "refs/remotes/upstream/trunk"} {
		fx.Git("update-ref", ref, "HEAD")
	}
	fx.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	fx.Git("symbolic-ref", "refs/remotes/upstream/HEAD", "refs/remotes/upstream/trunk")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}

	names := func() []string {
		ws, err := a.RemoteWebLinks(tab.Path)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, w := range ws {
			out = append(out, w.Remote)
		}
		return out
	}
	if got := names(); !slices.Equal(got, []string{"origin", "upstream"}) {
		t.Fatalf("remotes with web links = %v", got)
	}
	if got, _ := a.UnknownHosts(tab.Path); !slices.Equal(got, []string{"nas.local", "git.acme.com"}) {
		t.Fatalf("unknown hosts = %v", got)
	}

	// Saying what git.acme.com is gives it links, and takes it off the unknown list.
	if err := a.SaveForgeRule(tab.Path, links.ForgeRule{Host: "git.acme.com", Forge: "gitlab", Scope: links.Local}, ""); err != nil {
		t.Fatal(err)
	}
	if got := names(); !slices.Equal(got, []string{"origin", "upstream", "work"}) {
		t.Fatalf("after the rule: %v", got)
	}
	if got, _ := a.UnknownHosts(tab.Path); !slices.Equal(got, []string{"nas.local"}) {
		t.Fatalf("unknown hosts after the rule = %v", got)
	}

	ws, _ := a.RemoteWebLinks(tab.Path)
	origin := ws[0]
	if origin.Repo != "https://github.com/ris/GitRevenge" || origin.DefaultBranch != "main" ||
		!slices.Equal(origin.Bases, []WebBase{{"origin", "main"}, {"upstream", "trunk"}}) {
		t.Fatalf("origin = %+v", origin)
	}

	b, err := a.BranchWebLinks(tab.Path, "origin", "fix/a+b#1")
	if err != nil {
		t.Fatal(err)
	}
	if b.Branch != "https://github.com/ris/GitRevenge/tree/fix/a+b%231" || len(b.Bases) != 2 {
		t.Fatalf("branch links = %+v", b)
	}
	if got, want := b.Bases[1].Compare, "https://github.com/Git-Revenge/GitRevenge/compare/trunk...ris:fix/a+b%231"; got != want {
		t.Fatalf("compare with upstream = %q, want %q", got, want)
	}
	// The default branch isn't compared with itself.
	if b, _ := a.BranchWebLinks(tab.Path, "origin", "main"); len(b.Bases) != 1 || b.Bases[0].Remote != "upstream" {
		t.Fatalf("main's bases = %+v", b.Bases)
	}
	if b, _ := a.BranchWebLinks(tab.Path, "nas", "main"); b.Branch != "" || len(b.Bases) != 0 {
		t.Fatalf("a remote without a forge has links: %+v", b)
	}
	if u, _ := a.IssueURL(tab.Path); u != "https://github.com/ris/GitRevenge/issues/{n}" {
		t.Fatalf("issue URL = %q", u)
	}
}
