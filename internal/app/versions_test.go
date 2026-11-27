package app

import (
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// rebasedRepo is a repository whose topic branch was rebased with autosquash onto a main that
// moved on: topic@{1} is the version before. A fixup is folded in and main gained a commit.
func rebasedRepo(t *testing.T) (fx *gittest.Repo, before string) {
	t.Helper()
	body := func(prefix string, change map[int]string) string {
		var b strings.Builder
		for i := 1; i <= 12; i++ {
			if s, ok := change[i]; ok {
				b.WriteString(s + "\n")
			} else {
				b.WriteString(prefix + " line " + strings.Repeat("y", i%5) + string(rune('a'+i)) + "\n")
			}
		}
		return b.String()
	}
	fx = gittest.New(t)
	fx.CommitFile("a.txt", body("a", nil), "base")
	fx.Git("checkout", "-q", "-b", "topic")
	fx.CommitFile("cache.txt", body("cache", nil), "Add the cache")
	fx.CommitFile("cache.txt", body("cache", map[int]string{4: "only a reading"}), "fixup! Add the cache")
	fx.CommitFile("fall.txt", body("fall", nil), "Fall back to the cache")
	before = fx.Head()
	fx.Git("checkout", "-q", "main")
	fx.CommitFile("b.txt", body("b", nil), "Main moves on")
	fx.Git("checkout", "-q", "topic")
	fx.Git("-c", "sequence.editor=true", "rebase", "-q", "-i", "--autosquash", "main")
	return fx, before
}

func TestCompareVersionsAfterARebase(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, before := rebasedRepo(t)
	path := openForMap(t, a, fx)

	// No remote: the branch's base is given as main, and each version finds where it left it.
	oldBase, err := a.VersionBase(path, "topic", "main", before)
	if err != nil {
		t.Fatal(err)
	}
	newBase, err := a.VersionBase(path, "topic", "main", "topic")
	if err != nil {
		t.Fatal(err)
	}
	if oldBase == newBase || len(oldBase) != 40 || newBase != fx.Git("rev-parse", "main") {
		t.Fatalf("bases %q and %q: each version starts where it left main", oldBase, newBase)
	}
	n, err := a.CountVersion(path, oldBase, before)
	if err != nil || n.Commits != 3 || n.Merges != 0 {
		t.Fatalf("old count = %+v, %v", n, err)
	}

	got, err := a.CompareVersions(path, git.RangeSide{Base: oldBase, Tip: before}, git.RangeSide{Base: newBase, Tip: "topic"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.OldCount.Commits != 3 || got.NewCount.Commits != 2 || got.New.Tip != fx.Head() {
		t.Fatalf("compared = %+v", got)
	}
	kinds := map[string]string{}
	for _, p := range got.Pairs {
		kinds[p.Subject] = p.Kind
	}
	// Main's new commit isn't in either version, so nothing reads as new.
	want := map[string]string{"Add the cache": git.PairChanged, "fixup! Add the cache": git.PairFolded, "Fall back to the cache": git.PairSame}
	if len(kinds) != len(want) {
		t.Fatalf("pairs = %v\n%s", kinds, got.Text)
	}
	for s, k := range want {
		if kinds[s] != k {
			t.Errorf("%q: %q, want %q\n%s", s, kinds[s], k, got.Text)
		}
	}
	if !strings.HasPrefix(got.Command, "git range-diff "+oldBase[:7]+".."+before[:7]+" ") || !strings.Contains(got.Text, "Add the cache") {
		t.Errorf("command %q", got.Command)
	}
}

func TestCompareVersionsSaysWhatIsntThere(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, before := rebasedRepo(t)
	path := openForMap(t, a, fx)
	_, err := a.CompareVersions(path, git.RangeSide{Base: "main", Tip: "nowhere"}, git.RangeSide{Base: "main", Tip: "topic"}, false)
	if err == nil || !strings.Contains(err.Error(), "“nowhere” isn't a branch, tag or commit here") {
		t.Fatalf("err = %v", err)
	}
	if _, err := a.CountVersion(path, "", before); err == nil {
		t.Fatal("a version needs a start")
	}
}

func TestVersionBaseWithoutABase(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, _ := rebasedRepo(t)
	path := openForMap(t, a, fx)
	// No stack parent and no remote: nothing to go by.
	base, err := a.VersionBase(path, "topic", "", "topic")
	if err != nil || base != "" {
		t.Fatalf("base = %q, %v", base, err)
	}
}
