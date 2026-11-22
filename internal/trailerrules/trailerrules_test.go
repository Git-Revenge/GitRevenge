package trailerrules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// TestMain keeps the user's own git config out of the tests (see gittest.Main).
func TestMain(m *testing.M) { gittest.Main(m) }

// fixture is a repository, with the global config pointed at a file of the test's own.
func fixture(t *testing.T) (*gittest.Repo, *git.Repo, string) {
	t.Helper()
	bin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	fx := gittest.New(t)
	return fx, git.OpenRepo(git.NewRunner(bin), fx.Dir), global
}

func shown(t *testing.T, g *git.Repo) []string {
	t.Helper()
	rules, err := Load(context.Background(), g, true)
	if err != nil {
		t.Fatal(err)
	}
	return Shown(rules)
}

// A key is shown on rows once a config says so, and the repository's word comes before the
// global one.
func TestShowIsReadFromBothConfigsLocalFirst(t *testing.T) {
	fx, g, global := fixture(t)
	ctx := context.Background()
	if got := shown(t, g); len(got) != 0 {
		t.Fatalf("with nothing said, shown = %v", got)
	}
	if err := SetShow(ctx, g, Local, "Changelog", ShowRows); err != nil {
		t.Fatal(err)
	}
	if err := SetShow(ctx, g, Global, "Closes", ShowRows); err != nil {
		t.Fatal(err)
	}
	if got := shown(t, g); !slices.Equal(got, []string{"changelog", "closes"}) {
		t.Fatalf("shown = %v, want the repository's key then the global one", got)
	}
	// As a person would write it by hand.
	local, err := os.ReadFile(filepath.Join(fx.Dir, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(local), "[gitrevenge-trailer \"Changelog\"]") || !strings.Contains(string(local), "show = rows") {
		t.Fatalf("the repository's config:\n%s", local)
	}
	if text, _ := os.ReadFile(global); !strings.Contains(string(text), "[gitrevenge-trailer \"Closes\"]") {
		t.Fatalf("the global config:\n%s", text)
	}

	// The repository turns off a key the global config shows, without touching that file.
	if err := SetShow(ctx, g, Local, "Closes", ShowNone); err != nil {
		t.Fatal(err)
	}
	if got := shown(t, g); !slices.Equal(got, []string{"changelog"}) {
		t.Fatalf("shown = %v, want Closes turned off here", got)
	}
	if text, _ := os.ReadFile(global); !strings.Contains(string(text), "show = rows") {
		t.Fatalf("the global config was changed:\n%s", text)
	}
	// Saying nothing here leaves it to the global config again.
	if err := SetShow(ctx, g, Local, "Closes", ""); err != nil {
		t.Fatal(err)
	}
	if got := shown(t, g); !slices.Equal(got, []string{"changelog", "closes"}) {
		t.Fatalf("shown = %v, want Closes back", got)
	}
	// Outside a repository only the global rules are read.
	rules, err := Load(ctx, g, false)
	if err != nil || len(rules) != 1 || rules[0].ID != "closes" || rules[0].Scope != Global {
		t.Fatalf("the global rules alone = %+v, %v", rules, err)
	}
}

// A key is one key however it's spelled: the section that's there is the one changed.
func TestSetShowFindsTheKeyByAnySpelling(t *testing.T) {
	fx, g, _ := fixture(t)
	ctx := context.Background()
	if err := SetShow(ctx, g, Local, "Closes", ShowRows); err != nil {
		t.Fatal(err)
	}
	// "closed" is the same key: no second section, and unsetting it clears the first.
	if err := SetShow(ctx, g, Local, "closed", ShowRows); err != nil {
		t.Fatal(err)
	}
	text, _ := os.ReadFile(filepath.Join(fx.Dir, ".git", "config"))
	if n := strings.Count(string(text), "[gitrevenge-trailer"); n != 1 {
		t.Fatalf("%d sections, want one:\n%s", n, text)
	}
	if err := SetShow(ctx, g, Local, "CLOSE", ""); err != nil {
		t.Fatal(err)
	}
	if got := shown(t, g); len(got) != 0 {
		t.Fatalf("shown = %v after unsetting", got)
	}
	// Nothing is left behind, and unsetting what isn't set is fine.
	if text, _ = os.ReadFile(filepath.Join(fx.Dir, ".git", "config")); strings.Contains(string(text), "gitrevenge-trailer") {
		t.Fatalf("an empty section was left:\n%s", text)
	}
	if err := SetShow(ctx, g, Local, "Closes", ""); err != nil {
		t.Fatalf("unsetting what isn't set: %v", err)
	}
}

// What's written by hand is read as it is, and nothing that isn't a trailer key is written.
func TestHandWrittenRulesAndBadKeys(t *testing.T) {
	fx, g, _ := fixture(t)
	ctx := context.Background()
	fx.Git("config", "--local", "gitrevenge-trailer.Reviewed-by.show", "Rows")
	fx.Git("config", "--local", "gitrevenge-trailer.Reviewed-by.other", "kept")
	fx.Git("config", "--local", "gitrevenge-trailer.Signed-off-by.show", "somewhere")
	if got := shown(t, g); !slices.Equal(got, []string{"reviewed-by"}) {
		t.Fatalf("shown = %v, want the one that says rows, whatever the case", got)
	}
	// Unsetting show leaves a section that says something else.
	if err := SetShow(ctx, g, Local, "Reviewed-by", ""); err != nil {
		t.Fatal(err)
	}
	text, _ := os.ReadFile(filepath.Join(fx.Dir, ".git", "config"))
	if !strings.Contains(string(text), "other = kept") {
		t.Fatalf("the rest of the section went with it:\n%s", text)
	}
	for _, bad := range []string{"", "has space", "dot.ted", "new\nline", "-dash"} {
		if err := SetShow(ctx, g, Local, bad, ShowRows); err == nil {
			t.Errorf("%q was written as a key", bad)
		}
	}
	if err := SetShow(ctx, g, "elsewhere", "Closes", ShowRows); err == nil {
		t.Error("an unknown config was written to")
	}
}
