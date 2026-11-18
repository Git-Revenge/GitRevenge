package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// One folder, two spellings: the real one, which git reports, and one through a link, as a
// saved path may have it (on macOS /var is a link to /private/var; on Windows a folder has a
// short name too). Trust given under either holds for both.
func TestTrustHoldsHoweverThePathIsSpelled(t *testing.T) {
	t.Parallel()
	actual := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(actual, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "by-another-name")
	if err := os.Symlink(actual, linked); err != nil {
		t.Skipf("can't make a symbolic link here: %v", err)
	}

	var saved Settings
	saved.Trust(linked)
	if !saved.Trusts(actual) || !saved.Trusts(filepath.Join(actual, ".git"), actual) {
		t.Errorf("trusted as %v, but not by its real path %s", saved.TrustedRepos, actual)
	}

	var reported Settings
	reported.Trust(actual, filepath.Join(actual, ".git"))
	if !reported.Trusts(linked) {
		t.Errorf("trusted as %v, but not through the link %s", reported.TrustedRepos, linked)
	}
	// Trusting it again under the other spelling adds nothing.
	reported.Trust(linked)
	if len(reported.TrustedRepos) != 2 {
		t.Errorf("listed twice: %v", reported.TrustedRepos)
	}

	other := filepath.Join(t.TempDir(), "other")
	if reported.Trusts(other) || reported.Trusts("") {
		t.Error("trusts a folder it was never told about")
	}
}

// The settings a version before trust left behind: everything they name is trusted, once.
func TestSeedTrust(t *testing.T) {
	t.Parallel()
	s := Settings{
		Recent:   []RecentRepo{{Path: "/work/a"}},
		OpenTabs: []string{"/work/a", "/work/b"},
		Groups:   []Group{{Name: "g", Paths: []string{"/work/c"}}},
		Repos:    []RepoSettings{{Path: "/work/d"}},
	}
	s.SeedTrust()
	for _, p := range []string{"/work/a", "/work/b", "/work/c", "/work/d"} {
		if !s.Trusts(p) {
			t.Errorf("%s isn't trusted: %v", p, s.TrustedRepos)
		}
	}
	if len(s.TrustedRepos) != 4 || !s.TrustSeeded {
		t.Errorf("seeded %v, done %v", s.TrustedRepos, s.TrustSeeded)
	}
	// Only once: a repository added to a group later isn't trusted by a second pass.
	s.Groups[0].Paths = append(s.Groups[0].Paths, "/work/e")
	s.SeedTrust()
	if s.Trusts("/work/e") {
		t.Error("seeded a second time")
	}
}
