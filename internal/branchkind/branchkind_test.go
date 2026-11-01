package branchkind

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"main", "main", true},
		{"main", "main2", false},
		{"main", "mai", false},
		{"release/*", "release/1.4", true},
		{"release/*", "release/1.4/rc", true},
		{"release/*", "release", false},
		{"release/*", "releases/1.4", false},
		{"dependabot/*", "dependabot/npm_and_yarn/vite-7.1", true},
		{"*-wip", "feature/thing-wip", true},
		{"*", "anything/at/all", true},
		{"v?.x", "v1.x", true},
		{"v?.x", "v10.x", false},
		{"a?b", "a/b", true},
		{"*/hotfix", "team/hotfix", true},
		{"**", "", true},
		{"a*b*c", "aXbYbZc", true},
		{"a*b*c", "aXbYbZ", false},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.name); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestDefaults(t *testing.T) {
	r := New(nil, nil)
	cases := map[string]Kind{
		"main":                          Trunk,
		"develop":                       Trunk,
		"release/1.4":                   Release,
		"hotfix/crash":                  Release,
		"gh-pages":                      Docs,
		"dependabot/go_modules/x/y-1.2": Bot,
		"renovate/vite":                 Bot,
		"production":                    None,
		"feature/main":                  Feature,
		"features/x":                    Feature,
		"feat/deep/name":                Feature,
		"feature":                       None,
		"my-feature":                    None,
	}
	for name, want := range cases {
		if got := r.Kind(name); got != want {
			t.Errorf("Kind(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestOverrides(t *testing.T) {
	r := New(map[Kind][]string{
		Environment: {"staging", "production"},
		Docs:        {}, // matches nothing, so gh-pages becomes ordinary
		Trunk:       {"trunk-only"},
	}, []string{"wip/*", "gh-pages"})

	if r.Kind("production") != Environment || r.Kind("staging") != Environment {
		t.Fatal("environment overrides not applied")
	}
	if r.Kind("gh-pages") != None {
		t.Fatalf("gh-pages = %q with Docs emptied", r.Kind("gh-pages"))
	}
	if r.Kind("main") != None || r.Kind("trunk-only") != Trunk {
		t.Fatal("trunk override should replace the defaults, not add to them")
	}
	if r.Kind("release/2.0") != Release {
		t.Fatal("kinds without an override keep their defaults")
	}
	if !r.Hidden("wip/try") || !r.Hidden("gh-pages") || r.Hidden("main") {
		t.Fatal("hide patterns")
	}
}

func TestFirstKindWins(t *testing.T) {
	r := New(map[Kind][]string{Release: {"main"}}, nil)
	if r.Kind("main") != Trunk {
		t.Fatalf("main = %q; Trunk comes before Release", r.Kind("main"))
	}
}

func TestStrip(t *testing.T) {
	remotes := []string{"origin", "team", "team/mirror"}
	cases := map[string]string{
		"origin/release/1.4":   "release/1.4",
		"team/mirror/gh-pages": "gh-pages",
		"team/main":            "main",
		"upstream/main":        "upstream/main",
		"origin":               "origin",
		"originals/main":       "originals/main",
	}
	for in, want := range cases {
		if got := Strip(in, remotes); got != want {
			t.Errorf("Strip(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckPattern(t *testing.T) {
	for _, ok := range []string{"main", "release/*", "dependabot/*", "v?.x", "team-*/wip"} {
		if err := CheckPattern(ok); err != nil {
			t.Errorf("CheckPattern(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "has space", "a..b", "/lead", "trail/", "a//b", "x~1", "x^", "a:b", "back\\slash", "[abc]"} {
		if err := CheckPattern(bad); err == nil {
			t.Errorf("CheckPattern(%q) accepted", bad)
		}
	}
}
