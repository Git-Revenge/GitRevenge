package app

import (
	"slices"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func kindOf(p RepoPreferences, k branchkind.Kind) KindPatterns {
	for _, kp := range p.Kinds {
		if kp.Kind == string(k) {
			return kp
		}
	}
	return KindPatterns{}
}

func TestRepoPreferencesDefaults(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	p, err := a.RepoPreferences(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Kinds) != len(branchkind.Order) || p.CleanupOnPull || p.SyncNotes || len(p.Hide) != 0 {
		t.Fatalf("defaults = %+v", p)
	}
	docs := kindOf(p, branchkind.Docs)
	if docs.Custom || !slices.Equal(docs.Patterns, branchkind.Defaults[branchkind.Docs]) {
		t.Fatalf("docs = %+v", docs)
	}
	if env := kindOf(p, branchkind.Environment); env.Patterns == nil {
		t.Fatal("environment patterns should be an empty list, not null")
	}
}

func TestRepoPreferencesRoundTrip(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	repo, other := t.TempDir(), t.TempDir()
	p, _ := a.RepoPreferences(repo)
	for i := range p.Kinds {
		switch branchkind.Kind(p.Kinds[i].Kind) {
		case branchkind.Environment:
			p.Kinds[i].Patterns, p.Kinds[i].Custom = []string{" staging ", "production", "staging", ""}, true
		case branchkind.Docs:
			p.Kinds[i].Patterns, p.Kinds[i].Custom = []string{}, true
		case branchkind.Trunk:
			// Custom but equal to the defaults: stored as defaults.
			p.Kinds[i].Custom = true
		}
	}
	p.Hide = []string{"wip/*"}
	p.CleanupOnPull, p.SyncNotes = true, true
	if err := a.SetRepoPreferences(repo, p); err != nil {
		t.Fatal(err)
	}

	got, _ := a.RepoPreferences(repo)
	if env := kindOf(got, branchkind.Environment); !env.Custom || !slices.Equal(env.Patterns, []string{"staging", "production"}) {
		t.Fatalf("environment = %+v", env)
	}
	if docs := kindOf(got, branchkind.Docs); !docs.Custom || len(docs.Patterns) != 0 {
		t.Fatalf("docs = %+v", docs)
	}
	if trunk := kindOf(got, branchkind.Trunk); trunk.Custom {
		t.Fatalf("trunk equal to its defaults should be stored as defaults: %+v", trunk)
	}
	if !got.CleanupOnPull || !got.SyncNotes || !slices.Equal(got.Hide, []string{"wip/*"}) {
		t.Fatalf("got %+v", got)
	}

	r := a.branchRules(repo)
	if r.Kind("production") != branchkind.Environment || r.Kind("gh-pages") != branchkind.None || !r.Hidden("wip/x") {
		t.Fatal("rules don't follow the saved settings")
	}
	if a.branchRules(other).Kind("gh-pages") != branchkind.Docs {
		t.Fatal("another repository should keep the defaults")
	}

	// Back to nothing custom: the entry is dropped.
	reset, _ := a.RepoPreferences(repo)
	for i := range reset.Kinds {
		reset.Kinds[i].Custom = false
	}
	reset.Hide, reset.CleanupOnPull, reset.SyncNotes = nil, false, false
	if err := a.SetRepoPreferences(repo, reset); err != nil {
		t.Fatal(err)
	}
	m, _ := a.sessions()
	if n := len(m.Settings().Get().Repos); n != 0 {
		t.Fatalf("%d repo entries left after resetting", n)
	}
}

func TestRepoPreferencesValidation(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	repo := t.TempDir()
	if err := a.SetRepoPreferences(repo, RepoPreferences{Kinds: []KindPatterns{{Kind: "nightly", Custom: true}}}); err == nil {
		t.Error("accepted an unknown kind")
	}
	if err := a.SetRepoPreferences(repo, RepoPreferences{Hide: []string{"has space"}}); err == nil {
		t.Error("accepted a pattern with a space")
	}
	if err := a.SetRepoPreferences("", RepoPreferences{}); err == nil {
		t.Error("accepted no repository")
	}
}

func TestMarkBranchKind(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	repo := t.TempDir()
	if err := a.MarkBranchKind(repo, "production", "environment"); err != nil {
		t.Fatal(err)
	}
	if err := a.MarkBranchKind(repo, "website", "docs"); err != nil {
		t.Fatal(err)
	}
	r := a.branchRules(repo)
	if r.Kind("production") != branchkind.Environment || r.Kind("website") != branchkind.Docs || r.Kind("gh-pages") != branchkind.Docs {
		t.Fatal("docs should keep its defaults plus the new name")
	}
	// Moving it to another kind takes it out of the first.
	if err := a.MarkBranchKind(repo, "website", "release"); err != nil {
		t.Fatal(err)
	}
	if a.branchRules(repo).Kind("website") != branchkind.Release {
		t.Fatal("website should now be a release branch")
	}
	p, _ := a.RepoPreferences(repo)
	if docs := kindOf(p, branchkind.Docs); docs.Custom {
		t.Fatalf("docs is back to its defaults, so it shouldn't be custom: %+v", docs)
	}
	// Ordinary: the exact name goes; a default pattern still claiming it is reported.
	if err := a.MarkBranchKind(repo, "production", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.MarkBranchKind(repo, "release/2.0", ""); err == nil {
		t.Fatal("release/2.0 matches release/*, so it can't become ordinary by name")
	}
	if err := a.MarkBranchKind(repo, "x", "nightly"); err == nil {
		t.Fatal("accepted an unknown kind")
	}
}

// #1059: a path focus keeps any branch focus, a branch focus keeps any path, and clearing
// ends both.
func TestGraphPathFocus(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("src/alerts/sms.js", "x\n", "sms")
	fx.Branch("topic")
	fx.Checkout("main")
	path := openForMap(t, a, fx)

	if err := a.SetGraphPath(path, `src\alerts\`); err != nil {
		t.Fatal(err)
	}
	if f, _ := a.GraphFocus(path); f.Path != "src/alerts/" || f.Branch != "" {
		t.Fatalf("focus = %+v", f)
	}
	if err := a.SetGraphFocus(path, "topic", false); err != nil {
		t.Fatal(err)
	}
	if f, _ := a.GraphFocus(path); f.Path != "src/alerts/" || f.Branch != "topic" {
		t.Fatalf("with a branch: %+v", f)
	}
	// Stopping the branch focus keeps the path.
	if err := a.SetGraphFocus(path, "", false); err != nil {
		t.Fatal(err)
	}
	if f, _ := a.GraphFocus(path); f.Path != "src/alerts/" || f.Branch != "" {
		t.Fatalf("branch stopped: %+v", f)
	}
	if err := a.SetGraphPath(path, "src/alerts/sms.js"); err != nil {
		t.Fatal(err)
	}
	if f, _ := a.GraphFocus(path); f.Path != "src/alerts/sms.js" {
		t.Fatalf("a file: %+v", f)
	}
	if err := a.ClearGraphFocus(path); err != nil {
		t.Fatal(err)
	}
	if f, _ := a.GraphFocus(path); f.Path != "" || f.Branch != "" {
		t.Fatalf("cleared: %+v", f)
	}
	for _, bad := range []string{"../elsewhere", "src/../../x", "a//b"} {
		if err := a.SetGraphPath(path, bad); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
}

// #1060: the palette's box offers the folders and files at HEAD, and the count of the whole
// history arrives under a path focus.
func TestFocusSuggestionsAndTheWholeCount(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.CommitFile("src/alerts/sms.js", "x\n", "sms")
	fx.CommitFile("README.md", "r\n", "readme")
	path := openForMap(t, a, fx)

	got, err := a.FocusSuggestions(path, "src/al")
	if err != nil || !slices.Equal(got, []string{"src/alerts/", "src/alerts/sms.js"}) {
		t.Fatalf("suggestions = %v, %v", got, err)
	}
	// A new commit is listed: the paths are kept for one commit only.
	fx.CommitFile("src/alerts/gusts.js", "g\n", "gusts")
	if got, _ := a.FocusSuggestions(path, "gusts"); !slices.Equal(got, []string{"src/alerts/gusts.js"}) {
		t.Errorf("after a commit: %v", got)
	}

	if n, _ := a.GraphOutOf(path); n != 0 {
		t.Errorf("out of %d with no path focus", n)
	}
	if err := a.SetGraphPath(path, "src/alerts/"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if n, _ := a.GraphOutOf(path); n == 3 {
			return
		}
	}
	n, _ := a.GraphOutOf(path)
	t.Errorf("out of %d, want 3", n)
}

// #1061: a branch whose tip changed nothing under the path is labelled on the row where its
// history last did, after the refs that are there.
func TestGraphRowsLabelABranchWhereItLastTouchedThePath(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	c1 := fx.CommitFile("src/alerts/sms.js", "x\n", "sms")
	fx.Tag("v1", "v1")
	fx.CommitFile("README.md", "r\n", "readme")
	path := openForMap(t, a, fx)
	if err := a.SetGraphPath(path, "src/alerts/"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		p, err := a.GraphRows(path, 0, 10)
		if err != nil || p.Loading || len(p.Rows) != 1 {
			continue
		}
		refs := p.Refs[c1]
		if len(refs) == 2 && refs[0].Short == "v1" && refs[1].Short == "main" && refs[1].Later == 1 {
			return
		}
	}
	p, _ := a.GraphRows(path, 0, 10)
	t.Errorf("refs at sms: %+v", p.Refs[c1])
}
