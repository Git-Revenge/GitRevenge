package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/links"
)

// isolateGlobal points the global git config at an empty file of the test's own.
func isolateGlobal(t *testing.T) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", p)
}

func TestLinkRulesDriveShippedReferences(t *testing.T) {
	isolateGlobal(t)
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "Start")
	fx.CommitFile("a.txt", "2\n", "Fix the upload (ABC-12) #4")
	a := newTestApp(t)
	tab, _ := a.OpenRepo(fx.Dir)

	// No rule counts for Shipped: the built-in references.
	res, err := a.InsightsShipped(tab.Path, InsightsInputs{Window: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Items[0].Refs; !slices.Equal(got, []string{"ABC-12", "#4"}) || len(res.Window.References) != 0 {
		t.Fatalf("built-in refs = %v, inputs %v", got, res.Window.References)
	}

	jira := links.Rule{Name: "Jira", Pattern: `\bABC-\d+\b`, URL: "https://acme.example/browse/{0}", In: links.AllPlaces, Shipped: true, Enabled: true, Scope: links.Local}
	if err := a.SaveLinkRule(tab.Path, jira, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveLinkRule(tab.Path, jira, ""); err == nil {
		t.Fatal("saved a second Jira in the same config")
	}
	cfg, err := a.LinkRules(tab.Path)
	if err != nil || len(cfg.Rules) != 1 || cfg.Rules[0].Scope != links.Local || !cfg.Web {
		t.Fatalf("rules = %+v, %v", cfg, err)
	}
	// With a Shipped rule, only its matches are references, and the rule is an input.
	res, err = a.InsightsShipped(tab.Path, InsightsInputs{Window: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Items[0].Refs; !slices.Equal(got, []string{"ABC-12"}) || !slices.Equal(res.Window.References, []string{`Jira \bABC-\d+\b`}) {
		t.Fatalf("rule refs = %v, inputs %v", got, res.Window.References)
	}

	try, err := a.TryLinkRule(tab.Path, jira)
	if err != nil || try.Scanned != 2 || try.Commits != 1 || len(try.Matches) != 1 || try.Matches[0].Href != "https://acme.example/browse/ABC-12" {
		t.Fatalf("try = %+v, %v", try, err)
	}
	bad := jira
	bad.Pattern = "("
	if try, _ := a.TryLinkRule(tab.Path, bad); try.Problem == "" {
		t.Fatal("a broken pattern tried without a problem")
	}

	// Global rules can be kept without a repository.
	global := jira
	global.Scope, global.Name = links.Global, "Shared Jira"
	if err := a.SaveLinkRule("", global, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveLinkRule("", jira, ""); err == nil {
		t.Fatal("saved a repository rule with no repository")
	}
	if cfg, _ := a.LinkRules(""); len(cfg.Rules) != 1 || cfg.Rules[0].Name != "Shared Jira" {
		t.Fatalf("global rules = %+v", cfg.Rules)
	}

	// Import replaces a rule of the same name.
	file := filepath.Join(t.TempDir(), "links.json")
	if err := os.WriteFile(file, []byte(`[{"name":"Jira","pattern":"\\bABC-\\d+\\b","url":"https://new.example/{0}","in":["message"],"shipped":false,"enabled":true}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := a.ImportLinkRules(tab.Path, file, links.Local); err != nil || n != 1 {
		t.Fatalf("import = %d, %v", n, err)
	}
	cfg, _ = a.LinkRules(tab.Path)
	if cfg.Rules[0].URL != "https://new.example/{0}" || cfg.Rules[0].Shipped {
		t.Fatalf("after import = %+v", cfg.Rules[0])
	}
}
