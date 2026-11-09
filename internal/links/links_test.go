package links

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

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

var jira = Rule{Name: "Jira", Pattern: `\b(ABC|OPS)-\d+\b`, URL: "https://acme.atlassian.net/browse/{0}", In: AllPlaces, Shipped: true, Enabled: true, Scope: Global}

func TestSaveLoadAndShadow(t *testing.T) {
	_, g, global := fixture(t)
	ctx := context.Background()
	if err := Save(ctx, g, jira, ""); err != nil {
		t.Fatal(err)
	}
	num := Rule{Name: "Numbered issues", Pattern: `#(\d+)\b`, URL: "https://tracker.example/issues/{1}", In: []string{InTrailers}, Enabled: false, Scope: Local}
	if err := Save(ctx, g, num, ""); err != nil {
		t.Fatal(err)
	}
	local := jira
	local.Scope, local.URL = Local, "https://other.example/browse/{0}"
	if err := Save(ctx, g, local, ""); err != nil {
		t.Fatal(err)
	}
	c, err := Load(ctx, g, true)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range c.Rules {
		names = append(names, r.Scope+":"+r.Name)
	}
	if want := []string{"local:Numbered issues", "local:Jira", "global:Jira"}; !slices.Equal(names, want) {
		t.Fatalf("rules = %v, want %v", names, want)
	}
	if !c.Rules[2].Shadowed || c.Rules[1].Shadowed || c.Rules[0].Enabled || !slices.Equal(c.Rules[0].In, []string{InTrailers}) {
		t.Fatalf("rules = %+v", c.Rules)
	}
	if !c.Web {
		t.Fatal("web links off by default")
	}
	// The local Jira is the one in use.
	if got := c.Active(); len(got) != 1 || got[0].URL != local.URL {
		t.Fatalf("active = %+v", got)
	}
	// A backslash-heavy pattern survives git config's quoting.
	data, _ := os.ReadFile(global)
	if !strings.Contains(string(data), `[gitrevenge-link "Jira"]`) {
		t.Fatalf("global config = %s", data)
	}
	if c.Rules[2].Pattern != jira.Pattern {
		t.Fatalf("pattern read back as %q", c.Rules[2].Pattern)
	}
	// Outside a repository, only global rules.
	c, err = Load(ctx, g, false)
	if err != nil || len(c.Rules) != 1 || c.Rules[0].Shadowed {
		t.Fatalf("global only = %+v, %v", c, err)
	}
}

func TestRenameReorderRemove(t *testing.T) {
	_, g, _ := fixture(t)
	ctx := context.Background()
	a := Rule{Name: "A", Pattern: `A-\d+`, URL: "https://a.example/{0}", Enabled: true, Scope: Global}
	b := Rule{Name: "B", Pattern: `B-\d+`, URL: "https://b.example/{0}", Enabled: true, Scope: Global}
	for _, r := range []Rule{a, b} {
		if err := Save(ctx, g, r, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := Save(ctx, g, a, ""); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("duplicate save err = %v", err)
	}
	renamed := a
	renamed.Name = "Alpha"
	if err := Save(ctx, g, renamed, "A"); err != nil {
		t.Fatal(err)
	}
	if err := Reorder(ctx, g, Global, []string{"B", "Alpha"}); err != nil {
		t.Fatal(err)
	}
	c, _ := Load(ctx, g, true)
	if len(c.Rules) != 2 || c.Rules[0].Name != "B" || c.Rules[1].Name != "Alpha" {
		t.Fatalf("after rename and reorder = %+v", c.Rules)
	}
	if err := Remove(ctx, g, Global, "B"); err != nil {
		t.Fatal(err)
	}
	if c, _ := Load(ctx, g, true); len(c.Rules) != 1 {
		t.Fatalf("after remove = %+v", c.Rules)
	}
	if err := SetWeb(ctx, g, false); err != nil {
		t.Fatal(err)
	}
	if c, _ := Load(ctx, g, true); c.Web {
		t.Fatal("web links still on")
	}
	if err := SetWeb(ctx, g, true); err != nil {
		t.Fatal(err)
	}
	if c, _ := Load(ctx, g, true); !c.Web {
		t.Fatal("web links still off")
	}
}

func TestCheck(t *testing.T) {
	for _, c := range []struct {
		r    Rule
		want string
	}{
		{jira, ""},
		{Rule{Name: "", Pattern: "x", URL: "https://x"}, "needs a name"},
		{Rule{Name: "a", Pattern: "(", URL: "https://x"}, "regular expression"},
		{Rule{Name: "a", Pattern: "x*", URL: "https://x"}, "empty text"},
		{Rule{Name: "a", Pattern: "x", URL: "javascript:alert(1)"}, "https://"},
		{Rule{Name: "a", Pattern: "x", URL: "https://x/{1}"}, "{1}"},
		{Rule{Name: "a", Pattern: "x(y)", URL: "https://x/{1}"}, ""},
		{Rule{Name: "a\"b", Pattern: "x", URL: "https://x"}, "quotes"},
	} {
		err := Check(c.r)
		if (c.want == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), c.want) {
			t.Errorf("Check(%+v) = %v, want %q", c.r, err, c.want)
		}
	}
}

func TestFind(t *testing.T) {
	num := Rule{Name: "Num", Pattern: `#(\d+)\b`, URL: "https://t.example/i/{1}", In: []string{InTrailers}, Enabled: true}
	wide := Rule{Name: "Keys", Pattern: `\b[A-Z]+-\d+\b`, URL: "https://k.example/{0}", In: AllPlaces, Enabled: true}
	off := jira
	off.Enabled = false
	c := Config{Rules: []Rule{jira, wide, num, off}}
	got := c.Find("Fix ABC-12 and XYZ-9, see #4", InMessage)
	want := []Match{
		{Text: "ABC-12", Href: "https://acme.atlassian.net/browse/ABC-12", Rule: "Jira"},
		{Text: "XYZ-9", Href: "https://k.example/XYZ-9", Rule: "Keys"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("message matches = %+v", got)
	}
	if got := c.Find("#4 and #5", InTrailers); len(got) != 2 || got[1].Href != "https://t.example/i/5" {
		t.Fatalf("trailer matches = %+v", got)
	}
	if got := Expand("https://x/{1}?q={0}", []string{"a b/c", "é"}); got != "https://x/%C3%A9?q=a%20b%2Fc" {
		t.Fatalf("expand = %s", got)
	}
}

func TestReferencesAndPatterns(t *testing.T) {
	c := Config{Rules: []Rule{jira}}
	refs := c.References()
	if refs == nil {
		t.Fatal("no references function with a Shipped rule")
	}
	if got := refs("feat: ABC-1 (OPS-2)", InMessage); !slices.Equal(got, []string{"ABC-1", "OPS-2"}) {
		t.Fatalf("refs = %v", got)
	}
	if got := c.Patterns(); !slices.Equal(got, []string{`Jira \b(ABC|OPS)-\d+\b`}) {
		t.Fatalf("patterns = %v", got)
	}
	noShip := jira
	noShip.Shipped = false
	if (Config{Rules: []Rule{noShip}}).References() != nil {
		t.Fatal("references without a Shipped rule")
	}
}
