package weblinks

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/links"
)

func cfg(rules ...links.ForgeRule) links.Config { return links.Config{Forges: rules} }

func site(t *testing.T, url string, c links.Config) Site {
	t.Helper()
	s, ok := For(url, c)
	if !ok {
		t.Fatalf("For(%q) found no site", url)
	}
	return s
}

func TestFor(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"https://github.com/o/r.git":                       "https://github.com/o/r",
		"git@github.com:o/r.git":                           "https://github.com/o/r",
		"ssh://git@ssh.github.com:443/o/r.git":             "https://github.com/o/r",
		"https://user:token@github.com/o/r/":               "https://github.com/o/r",
		"git@gitlab.com:group/sub/proj.git":                "https://gitlab.com/group/sub/proj",
		"ssh://git@altssh.gitlab.com:443/g/p.git":          "https://gitlab.com/g/p",
		"https://gitea.com/o/r":                            "https://gitea.com/o/r",
		"git@codeberg.org:o/r.git":                         "https://codeberg.org/o/r",
		"git@bitbucket.org:team/r.git":                     "https://bitbucket.org/team/r",
		"https://org@dev.azure.com/org/proj/_git/repo":     "https://dev.azure.com/org/proj/_git/repo",
		"git@ssh.dev.azure.com:v3/org/proj/repo":           "https://dev.azure.com/org/proj/_git/repo",
		"https://dev.azure.com/org/My%20Project/_git/repo": "https://dev.azure.com/org/My%20Project/_git/repo",
		"org@vs-ssh.visualstudio.com:v3/org/proj/repo":     "https://org.visualstudio.com/proj/_git/repo",
		"https://github.com/o/r with space.git":            "https://github.com/o/r%20with%20space",
		"git@github.com:o/r.git ":                          "https://github.com/o/r",
	}
	for in, want := range cases {
		if got := site(t, in, links.Config{}).Repo; got != want {
			t.Errorf("For(%q).Repo = %q, want %q", in, got, want)
		}
	}
}

func TestNoSiteWithoutAForge(t *testing.T) {
	t.Parallel()
	for _, u := range []string{
		"https://gitlab.example.com/team/app.git", // never guessed from its name
		"git@git.example.com:team/app.git",
		"/srv/git/app.git",
		`C:\repos\app`,
		"file:///srv/git/app.git",
		"https://github.com",
	} {
		if s, ok := For(u, links.Config{}); ok {
			t.Errorf("For(%q) = %+v, want no site", u, s)
		}
	}
}

func TestForgeRules(t *testing.T) {
	t.Parallel()
	c := cfg(
		links.ForgeRule{Host: "gitlab.example.com", Forge: git.ForgeGitLab},
		links.ForgeRule{Host: "git.acme.com", Forge: git.ForgeGitea, Web: "https://code.acme.com"},
		links.ForgeRule{Host: "github-work", Forge: git.ForgeGitHub, Web: "https://github.com"},
		links.ForgeRule{Host: "bb.acme.com", Forge: git.ForgeBitbucketServer},
		links.ForgeRule{Host: "sub.acme.com", Forge: git.ForgeGitLab, Web: "https://sub.acme.com/gitlab"},
		links.ForgeRule{Host: "off.acme.com", Forge: git.ForgeGitLab, Shadowed: true},
		links.ForgeRule{Host: "bad.acme.com", Forge: git.ForgeGitLab, Problem: "no"},
	)
	cases := map[string]string{
		"https://gitlab.example.com:8443/team/app.git":   "https://gitlab.example.com:8443/team/app",
		"git@gitlab.example.com:team/app.git":            "https://gitlab.example.com/team/app",
		"ssh://git@gitlab.example.com:2222/team/app.git": "https://gitlab.example.com/team/app",
		"http://gitlab.example.com/team/app.git":         "http://gitlab.example.com/team/app",
		"git@git.acme.com:o/r.git":                       "https://code.acme.com/o/r",
		"github-work:o/r.git":                            "https://github.com/o/r",
		"https://bb.acme.com/scm/proj/repo.git":          "https://bb.acme.com/projects/PROJ/repos/repo",
		"ssh://git@bb.acme.com:7999/proj/repo.git":       "https://bb.acme.com/projects/PROJ/repos/repo",
		"ssh://git@bb.acme.com:7999/~ris/repo.git":       "https://bb.acme.com/users/ris/repos/repo",
		"https://sub.acme.com/gitlab/group/app.git":      "https://sub.acme.com/gitlab/group/app",
		"git@sub.acme.com:group/app.git":                 "https://sub.acme.com/gitlab/group/app",
	}
	for in, want := range cases {
		if got := site(t, in, c).Repo; got != want {
			t.Errorf("For(%q).Repo = %q, want %q", in, got, want)
		}
	}
	for _, u := range []string{"git@off.acme.com:o/r.git", "git@bad.acme.com:o/r.git", "https://bb.acme.com/scm/only.git"} {
		if _, ok := For(u, c); ok {
			t.Errorf("For(%q) has a site", u)
		}
	}
}

func TestBranchPages(t *testing.T) {
	t.Parallel()
	c := cfg(links.ForgeRule{Host: "bb.acme.com", Forge: git.ForgeBitbucketServer}, links.ForgeRule{Host: "git.acme.com", Forge: git.ForgeForgejo})
	const b = "feature/a b"
	cases := []struct {
		url, branches, branch, compare string
	}{
		{"git@github.com:o/r.git", "https://github.com/o/r/branches", "https://github.com/o/r/tree/feature/a%20b",
			"https://github.com/o/r/compare/main...feature/a%20b"},
		{"git@gitlab.com:g/p.git", "https://gitlab.com/g/p/-/branches", "https://gitlab.com/g/p/-/tree/feature/a%20b",
			"https://gitlab.com/g/p/-/compare/main...feature/a%20b"},
		{"https://gitea.com/o/r", "https://gitea.com/o/r/branches", "https://gitea.com/o/r/src/branch/feature/a%20b",
			"https://gitea.com/o/r/compare/main...feature/a%20b"},
		{"git@git.acme.com:o/r.git", "https://git.acme.com/o/r/branches", "https://git.acme.com/o/r/src/branch/feature/a%20b",
			"https://git.acme.com/o/r/compare/main...feature/a%20b"},
		{"git@codeberg.org:o/r.git", "https://codeberg.org/o/r/branches", "https://codeberg.org/o/r/src/branch/feature/a%20b",
			"https://codeberg.org/o/r/compare/main...feature/a%20b"},
		{"git@bitbucket.org:t/r.git", "https://bitbucket.org/t/r/branches", "https://bitbucket.org/t/r/src/feature/a%20b",
			"https://bitbucket.org/t/r/branches/compare/feature/a%20b%0Dmain"},
		{"https://bb.acme.com/scm/proj/repo.git", "https://bb.acme.com/projects/PROJ/repos/repo/branches",
			"https://bb.acme.com/projects/PROJ/repos/repo/browse?at=refs%2Fheads%2Ffeature%2Fa+b",
			"https://bb.acme.com/projects/PROJ/repos/repo/compare/commits?sourceBranch=refs%2Fheads%2Ffeature%2Fa+b&targetBranch=refs%2Fheads%2Fmain"},
		{"git@ssh.dev.azure.com:v3/org/proj/repo", "https://dev.azure.com/org/proj/_git/repo/branches",
			"https://dev.azure.com/org/proj/_git/repo?version=GBfeature%2Fa+b",
			"https://dev.azure.com/org/proj/_git/repo/branchCompare?baseVersion=GBmain&targetVersion=GBfeature%2Fa+b"},
	}
	for _, tc := range cases {
		s := site(t, tc.url, c)
		base := Base{Site: s, Branch: "main"}
		for _, got := range []struct{ name, got, want string }{
			{"Branches", s.Branches(), tc.branches},
			{"Branch", s.Branch(b), tc.branch},
			{"Compare", s.Compare(b, base), tc.compare},
		} {
			if got.got != got.want {
				t.Errorf("%s: %s = %q, want %q", tc.url, got.name, got.got, got.want)
			}
		}
	}
}

func TestNoBaseKnown(t *testing.T) {
	t.Parallel()
	gh := site(t, "git@github.com:o/r.git", links.Config{})
	if got := gh.Compare("x", Base{Site: gh}); got != "" {
		t.Errorf("compare with no base = %q", got)
	}
}

func TestForks(t *testing.T) {
	t.Parallel()
	fork := site(t, "git@github.com:ris/GitRevenge.git", links.Config{})
	up := site(t, "https://github.com/Git-Revenge/gitrevenge.git", links.Config{})
	other := site(t, "https://github.com/someone/else.git", links.Config{})
	lab := site(t, "git@gitlab.com:ris/GitRevenge.git", links.Config{})
	if !fork.ForkOf(up) || fork.ForkOf(other) || fork.ForkOf(fork) || fork.ForkOf(lab) {
		t.Fatal("ForkOf is wrong")
	}
	base := Base{Site: up, Branch: "main"}
	if got, want := fork.Compare("fix/x", base), "https://github.com/Git-Revenge/gitrevenge/compare/main...ris:fix/x"; got != want {
		t.Errorf("compare across forks = %q, want %q", got, want)
	}
	labUp := site(t, "git@gitlab.com:team/GitRevenge.git", links.Config{})
	if got := lab.Compare("x", Base{Site: labUp, Branch: "main"}); got != "" {
		t.Errorf("GitLab can't compare across forks by address, got %q", got)
	}
}

func TestIssues(t *testing.T) {
	t.Parallel()
	c := cfg(links.ForgeRule{Host: "gitlab.example.com", Forge: git.ForgeGitLab})
	cases := map[string]string{
		"git@github.com:o/r.git":              "https://github.com/o/r/issues/{n}",
		"git@gitlab.example.com:team/app.git": "https://gitlab.example.com/team/app/-/issues/{n}",
		"git@codeberg.org:o/r.git":            "https://codeberg.org/o/r/issues/{n}",
		"git@bitbucket.org:t/r.git":           "",
	}
	for in, want := range cases {
		if got := site(t, in, c).Issues(); got != want {
			t.Errorf("Issues(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClone(t *testing.T) {
	t.Parallel()
	c := cfg(links.ForgeRule{Host: "gitlab.example.com", Forge: git.ForgeGitLab}, links.ForgeRule{Host: "bb.acme.com", Forge: git.ForgeBitbucketServer})
	cases := map[string][2]string{
		"git@github.com:o/r.git":                       {"https://github.com/o/r.git", "git@github.com:o/r.git"},
		"https://github.com/o/r":                       {"https://github.com/o/r.git", "git@github.com:o/r.git"},
		"ssh://git@ssh.github.com:443/o/r.git":         {"https://github.com/o/r.git", "git@github.com:o/r.git"},
		"https://gitlab.com/g/sub/p.git":               {"https://gitlab.com/g/sub/p.git", "git@gitlab.com:g/sub/p.git"},
		"git@codeberg.org:o/r.git":                     {"https://codeberg.org/o/r.git", "git@codeberg.org:o/r.git"},
		"https://org@dev.azure.com/org/proj/_git/repo": {"https://dev.azure.com/org/proj/_git/repo", "git@ssh.dev.azure.com:v3/org/proj/repo"},
		"git@gitlab.example.com:team/app.git":          {"https://gitlab.example.com/team/app.git", ""},
		"https://bb.acme.com/scm/proj/repo.git":        {"", ""},
	}
	for in, want := range cases {
		https, ssh := site(t, in, c).Clone()
		if https != want[0] || ssh != want[1] {
			t.Errorf("Clone(%q) = %q, %q, want %q, %q", in, https, ssh, want[0], want[1])
		}
	}
}
