package links

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
)

func TestForgeRules(t *testing.T) {
	_, g, global := fixture(t)
	ctx := context.Background()
	lab := ForgeRule{Host: "Git.Acme.com", Forge: git.ForgeGitLab, Scope: Global}
	if err := SaveForge(ctx, g, lab, ""); err != nil {
		t.Fatal(err)
	}
	local := ForgeRule{Host: "git.acme.com", Forge: git.ForgeGitea, Web: "https://code.acme.com/", Scope: Local}
	if err := SaveForge(ctx, g, local, ""); err != nil {
		t.Fatal(err)
	}
	c, err := Load(ctx, g, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Forges) != 2 || c.Forges[0].Scope != Local || !c.Forges[1].Shadowed {
		t.Fatalf("forges = %+v", c.Forges)
	}
	f, ok := c.ForgeFor("GIT.acme.com")
	if !ok || f.Forge != git.ForgeGitea || f.Web != "https://code.acme.com" {
		t.Fatalf("ForgeFor = %+v, %v", f, ok)
	}
	data, _ := os.ReadFile(global)
	if !strings.Contains(string(data), `[gitrevenge-forge "git.acme.com"]`) || !strings.Contains(string(data), "type = gitlab") {
		t.Fatalf("global config:\n%s", data)
	}

	// Renaming the host moves the rule; clearing web unsets it.
	moved := ForgeRule{Host: "code.acme.com", Forge: git.ForgeGitea, Scope: Local}
	if err := SaveForge(ctx, g, moved, "git.acme.com"); err != nil {
		t.Fatal(err)
	}
	c, _ = Load(ctx, g, true)
	if _, ok := c.ForgeFor("code.acme.com"); !ok {
		t.Fatalf("moved rule missing: %+v", c.Forges)
	}
	if f, _ := c.ForgeFor("git.acme.com"); f.Forge != git.ForgeGitLab || f.Scope != Global {
		t.Fatalf("the global rule should be back in force: %+v", f)
	}

	if err := RemoveForge(ctx, g, Local, "code.acme.com"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveForge(ctx, g, Local, "never.acme.com"); err != nil {
		t.Fatalf("removing a missing rule: %v", err)
	}
	c, _ = Load(ctx, g, false)
	if len(c.Forges) != 1 {
		t.Fatalf("forges = %+v", c.Forges)
	}
}

func TestCheckForge(t *testing.T) {
	t.Parallel()
	bad := []ForgeRule{
		{Host: "", Forge: git.ForgeGitLab},
		{Host: "https://git.acme.com", Forge: git.ForgeGitLab},
		{Host: "git.acme.com:8443", Forge: git.ForgeGitLab},
		{Host: "git.acme.com", Forge: "sourcehut"},
		{Host: "git.acme.com", Forge: git.ForgeCodeberg},
		{Host: "git.acme.com", Forge: git.ForgeGitLab, Web: "ftp://git.acme.com"},
		{Host: "git.acme.com", Forge: git.ForgeGitLab, Web: "https://git.acme.com/{0}"},
	}
	for _, f := range bad {
		if CheckForge(f) == nil {
			t.Errorf("CheckForge(%+v) passed", f)
		}
	}
	if err := CheckForge(ForgeRule{Host: "github-work", Forge: git.ForgeGitHub, Web: "https://github.com"}); err != nil {
		t.Error(err)
	}
}

func TestAProblemRuleIsKeptButNotUsed(t *testing.T) {
	fx, g, _ := fixture(t)
	fx.Git("config", "gitrevenge-forge.git.acme.com.type", "sourcehut")
	c, err := Load(context.Background(), g, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Forges) != 1 || c.Forges[0].Problem == "" {
		t.Fatalf("forges = %+v", c.Forges)
	}
	if _, ok := c.ForgeFor("git.acme.com"); ok {
		t.Fatal("a rule with a problem is in force")
	}
}
