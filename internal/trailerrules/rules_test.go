package trailerrules

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

func rule(t *testing.T, g *git.Repo, id string) (Rule, bool) {
	t.Helper()
	rules, err := Load(context.Background(), g, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		if r.ID == id {
			return r, true
		}
	}
	return Rule{}, false
}

// A rule as the issue writes it is read whole, and written back the same way.
func TestARuleSaysAlsoKindAndShow(t *testing.T) {
	fx, g, _ := fixture(t)
	ctx := context.Background()
	if err := Join(ctx, g, Local, "Reviewed-by", "Rviewed-by"); err != nil {
		t.Fatal(err)
	}
	if err := SetKind(ctx, g, Local, "Reviewed-by", "people"); err != nil {
		t.Fatal(err)
	}
	if err := SetShow(ctx, g, Local, "Reviewed-by", ShowPanel); err != nil {
		t.Fatal(err)
	}
	text, _ := os.ReadFile(filepath.Join(fx.Dir, ".git", "config"))
	for _, want := range []string{"[gitrevenge-trailer \"Reviewed-by\"]", "also = Rviewed-by", "kind = people", "show = panel"} {
		if !strings.Contains(string(text), want) {
			t.Fatalf("the config lacks %q:\n%s", want, text)
		}
	}
	if n := strings.Count(string(text), "[gitrevenge-trailer"); n != 1 {
		t.Fatalf("%d sections, want the one:\n%s", n, text)
	}
	r, found := rule(t, g, "reviewed-by")
	if !found || !slices.Equal(r.Also, []string{"Rviewed-by"}) || r.Kind != "people" || r.Show != ShowPanel || r.Scope != Local {
		t.Fatalf("the rule as read = %+v, %v", r, found)
	}

	// What a reading takes from it.
	var to trailerindex.Rules
	rules, _ := Load(ctx, g, true)
	Apply(rules, &to)
	if to.Join["rviewed-by"] != "reviewed-by" || to.Labels["reviewed-by"] != "Reviewed-by" || to.Kinds["reviewed-by"] != trailerindex.KindPerson || to.Show["reviewed-by"] != trailerindex.ShowPanel {
		t.Fatalf("applied = %+v", to)
	}

	// A second key joined, the same one again, and one taken back.
	if err := Join(ctx, g, Local, "reviewed-by", "Review-by"); err != nil {
		t.Fatal(err)
	}
	if err := Join(ctx, g, Local, "Reviewed-by", "RVIEWED-BY"); err != nil {
		t.Fatal(err)
	}
	if r, _ = rule(t, g, "reviewed-by"); !slices.Equal(r.Also, []string{"Rviewed-by", "Review-by"}) {
		t.Fatalf("also = %v, want the two, each once", r.Also)
	}
	if err := Unjoin(ctx, g, Local, "Reviewed-by", "rviewed-by"); err != nil {
		t.Fatal(err)
	}
	if r, _ = rule(t, g, "reviewed-by"); !slices.Equal(r.Also, []string{"Review-by"}) || r.Kind != "people" {
		t.Fatalf("after taking one back = %+v", r)
	}

	// Each thing taken away in turn leaves nothing behind.
	for _, undo := range []func() error{
		func() error { return Unjoin(ctx, g, Local, "Reviewed-by", "Review-by") },
		func() error { return SetKind(ctx, g, Local, "Reviewed-by", "") },
		func() error { return SetShow(ctx, g, Local, "Reviewed-by", "") },
	} {
		if err := undo(); err != nil {
			t.Fatal(err)
		}
	}
	if text, _ = os.ReadFile(filepath.Join(fx.Dir, ".git", "config")); strings.Contains(string(text), "gitrevenge-trailer") {
		t.Fatalf("something was left behind:\n%s", text)
	}
}

// Keep apart is remembered, either way round, and joining after all forgets it.
func TestKeepApartIsRemembered(t *testing.T) {
	_, g, _ := fixture(t)
	ctx := context.Background()
	rules, _ := Load(ctx, g, true)
	if KeptApart(rules, "ticket", "refs") {
		t.Fatal("kept apart with nothing said")
	}
	if err := KeepApart(ctx, g, Local, "Ticket", "Refs"); err != nil {
		t.Fatal(err)
	}
	rules, _ = Load(ctx, g, true)
	if !KeptApart(rules, "ticket", "refs") || !KeptApart(rules, "refs", "ticket") {
		t.Fatalf("kept apart isn't remembered both ways: %+v", rules)
	}
	if KeptApart(rules, "ticket", "closes") {
		t.Fatal("kept apart from a key nobody asked about")
	}
	// By another ending it's the same answer: "Ref" is Refs.
	if err := KeepApart(ctx, g, Local, "ticket", "Ref"); err != nil {
		t.Fatal(err)
	}
	if r, _ := rule(t, g, "ticket"); len(r.Apart) != 1 {
		t.Fatalf("apart = %v, want it said once", r.Apart)
	}
	// Joined after all: no longer apart.
	if err := Join(ctx, g, Local, "Ticket", "Refs"); err != nil {
		t.Fatal(err)
	}
	rules, _ = Load(ctx, g, true)
	if KeptApart(rules, "ticket", "refs") {
		t.Fatalf("still kept apart after joining: %+v", rules)
	}
	if r, _ := rule(t, g, "ticket"); !slices.Equal(r.Also, []string{"Refs"}) {
		t.Fatalf("also = %v", r.Also)
	}
	if err := ForgetApart(ctx, g, Local, "Ticket", "Refs"); err != nil {
		t.Fatalf("forgetting what isn't there: %v", err)
	}
}

// What can't be said isn't written: a kind or a place there's no word for, a key joined
// to itself, a name that isn't a key.
func TestRulesRefuseWhatTheyCannotSay(t *testing.T) {
	fx, g, _ := fixture(t)
	ctx := context.Background()
	for name, err := range map[string]error{
		"a kind there is no word for":  SetKind(ctx, g, Local, "Closes", "tikets"),
		"a place there is no word for": SetShow(ctx, g, Local, "Closes", "everywhere"),
		"a key joined to itself":       Join(ctx, g, Local, "Closes", "closed"),
		"a key apart from itself":      KeepApart(ctx, g, Local, "Closes", "Close"),
		"a name that is not a key":     Join(ctx, g, Local, "Closes", "two words"),
		"a section that is not a key":  Join(ctx, g, Local, "two words", "Closes"),
	} {
		if err == nil {
			t.Errorf("%s was written", name)
		}
	}
	if text, _ := os.ReadFile(filepath.Join(fx.Dir, ".git", "config")); strings.Contains(string(text), "gitrevenge-trailer") {
		t.Fatalf("something was written:\n%s", text)
	}
	// Every kind a rule has a word for has a word back.
	for name, kind := range kinds {
		if KindName(kind) != name {
			t.Errorf("KindName(%q) = %q, want %q", kind, KindName(kind), name)
		}
	}
}

// A repository's rule for a key replaces the global one whole, and a key joined to another
// gives up what a rule of its own says.
func TestLocalReplacesGlobalAndAJoinedKeyYields(t *testing.T) {
	_, g, _ := fixture(t)
	ctx := context.Background()
	if err := SetKind(ctx, g, Global, "Ticket", "tickets"); err != nil {
		t.Fatal(err)
	}
	if err := SetShow(ctx, g, Global, "Ticket", ShowPanel); err != nil {
		t.Fatal(err)
	}
	if err := SetShow(ctx, g, Local, "Ticket", ShowHeld); err != nil {
		t.Fatal(err)
	}
	r, _ := rule(t, g, "ticket")
	if r.Scope != Local || r.Show != ShowHeld || r.Kind != "" {
		t.Fatalf("the rule in force = %+v, want the repository's, whole", r)
	}

	// Fixes joined to Closes: a rule that Fixes still has says nothing any more.
	if err := Join(ctx, g, Local, "Closes", "Fixes"); err != nil {
		t.Fatal(err)
	}
	if err := SetShow(ctx, g, Local, "Fixes", ShowHeld); err != nil {
		t.Fatal(err)
	}
	rules, _ := Load(ctx, g, true)
	var to trailerindex.Rules
	Apply(rules, &to)
	if to.Join["fixes"] != "closes" {
		t.Fatalf("join = %v", to.Join)
	}
	if _, said := to.Show["fixes"]; said {
		t.Fatalf("a joined key still says where it shows: %v", to.Show)
	}
	if to.Show["ticket"] != trailerindex.ShowHeld {
		t.Fatalf("show = %v", to.Show)
	}
}
