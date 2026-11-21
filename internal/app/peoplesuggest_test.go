package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// suggestRepo has one person under two addresses, one address under two names, and three
// matches that shouldn't be offered: a bot, a no-reply address and a machine's name.
func suggestRepo(t *testing.T) (*App, *gittest.Repo, string) {
	t.Helper()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sarah Lind", "sarah@tidewater.example", "s1.txt", "one")
	commitAs(fx, "Sarah Lind", "sarah@tidewater.example", "s2.txt", "two")
	commitAs(fx, "sarah lind", "slind@users.noreply.example", "s3.txt", "three")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "o1.txt", "four")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "o2.txt", "five")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "o3.txt", "six")
	commitAs(fx, "sam", "sam@tidewater.example", "o4.txt", "seven")
	commitAs(fx, "dependabot[bot]", "bot@users.noreply.example", "d1.txt", "Bump vite")
	commitAs(fx, "dependabot[bot]", "49699333+dependabot[bot]@users.noreply.example", "d2.txt", "Bump svelte")
	commitAs(fx, "Claude Opus", "noreply@models.example", "c1.txt", "eight")
	commitAs(fx, "Claude Fable", "noreply@models.example", "c2.txt", "nine")
	commitAs(fx, "root", "root@build-1.example", "r1.txt", "ten")
	commitAs(fx, "root", "root@build-2.example", "r2.txt", "eleven")
	return a, fx, openForMap(t, a, fx)
}

func keys(list []Suggestion) []string {
	out := []string{}
	for _, s := range list {
		out = append(out, s.Key)
	}
	return out
}

func TestPeopleSuggestionsOfferLikelyMatchesOnly(t *testing.T) {
	t.Parallel()
	a, fx, path := suggestRepo(t)

	got, err := a.PeopleSuggestions(path, MailmapText{})
	if err != nil {
		t.Fatal(err)
	}
	// Most commits first: Sam's four, then Sarah's three.
	if want := []string{"address:sam@tidewater.example", "name:sarah lind"}; !slices.Equal(keys(got.Offers), want) || got.Offered != 2 {
		t.Fatalf("offers = %q (%d), want %q", keys(got.Offers), got.Offered, want)
	}
	sam := got.Offers[0]
	if sam.Kind != "address" || len(sam.People) != 2 || sam.People[0].Name != "Sam Ortiz" || sam.People[0].Commits != 3 || sam.People[1].Name != "sam" {
		t.Fatalf("the shared address = %+v", sam)
	}
	sarah := got.Offers[1]
	if sarah.Kind != "name" || sarah.People[0].Email != "sarah@tidewater.example" || len(sarah.People[1].Identities) != 1 || sarah.People[1].Identities[0].Email != "slind@users.noreply.example" {
		t.Fatalf("the shared name = %+v", sarah)
	}

	// Left out, each with why, and never mixed in with the offers.
	why := map[string]string{}
	for _, s := range got.Shared {
		why[s.Key] = s.Why
	}
	want := map[string]string{
		"name:dependabot[bot]":           "a bot",
		"address:noreply@models.example": "a no-reply address",
		"name:root":                      "a name many machines commit under",
	}
	if got.SharedCount != 3 || len(why) != 3 {
		t.Fatalf("left out = %v (%d)", why, got.SharedCount)
	}
	for k, w := range want {
		if why[k] != w {
			t.Errorf("%s is left out because %q, want %q", k, why[k], w)
		}
	}
	if len(got.Apart) != 0 {
		t.Errorf("nothing was answered yet, and apart = %q", keys(got.Apart))
	}
	// Asking is reading: no mailmap appears.
	if _, err := os.Stat(filepath.Join(fx.Dir, ".mailmap")); err == nil {
		t.Fatal("asking for suggestions created a .mailmap")
	}
}

func TestDifferentPeopleIsRememberedAndCanBeUndone(t *testing.T) {
	t.Parallel()
	a, fx, path := suggestRepo(t)

	if err := a.KeepApart(path, "name:sarah lind", true); err != nil {
		t.Fatal(err)
	}
	got, err := a.PeopleSuggestions(path, MailmapText{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"address:sam@tidewater.example"}; !slices.Equal(keys(got.Offers), want) {
		t.Fatalf("offers = %q, want Sarah's gone", keys(got.Offers))
	}
	if want := []string{"name:sarah lind"}; !slices.Equal(keys(got.Apart), want) || got.ApartCount != 1 {
		t.Fatalf("apart = %q", keys(got.Apart))
	}
	// It lives in the repository's own git config, so it's the repository's, not the app's.
	if out := fx.Git("config", "--local", "--get-all", "gitrevenge.apart"); out == "" {
		t.Fatal("nothing was written to the repository's git config")
	}

	if err := a.KeepApart(path, "name:sarah lind", false); err != nil {
		t.Fatal(err)
	}
	got, err = a.PeopleSuggestions(path, MailmapText{})
	if err != nil || len(got.Offers) != 2 || len(got.Apart) != 0 {
		t.Fatalf("after undoing: offers %q, apart %q, %v", keys(got.Offers), keys(got.Apart), err)
	}
}

func TestPlanSamePersonFromASuggestion(t *testing.T) {
	t.Parallel()
	a, fx, path := suggestRepo(t)
	got, err := a.PeopleSuggestions(path, MailmapText{})
	if err != nil {
		t.Fatal(err)
	}
	sarah := got.Offers[1]
	var members []git.Identity
	for _, p := range sarah.People {
		members = append(members, p.Identities...)
	}

	plan, err := a.PlanSamePerson(path, MailmapText{}, SamePersonRequest{Identities: members, Name: sarah.People[0].Name, Email: sarah.People[0].Email})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>"}; !slices.Equal(plan.Lines, want) || plan.Text != want[0]+"\n" {
		t.Fatalf("plan = %+v", plan)
	}
	// A plan writes nothing.
	if _, err := os.Stat(filepath.Join(fx.Dir, ".mailmap")); err == nil {
		t.Fatal("planning created a .mailmap")
	}

	// Read through the planned text, as the view does before it's saved, they're one person
	// and the suggestion is gone.
	edited := MailmapText{Text: plan.Text, Edited: true}
	after, err := a.PeopleSuggestions(path, edited)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"address:sam@tidewater.example"}; !slices.Equal(keys(after.Offers), want) {
		t.Fatalf("offers after the join = %q, want %q", keys(after.Offers), want)
	}
	list, err := a.People(path, PeopleRequest{Mailmap: edited, Query: "sarah"})
	if err != nil || list.Matching != 1 || list.People[0].Commits != 3 {
		t.Fatalf("people named sarah after the join = %+v, %v", list.People, err)
	}

	// The second join goes into the same text, beside nothing of Sam's, so at the end.
	sam := after.Offers[0]
	members = nil
	for _, p := range sam.People {
		members = append(members, p.Identities...)
	}
	plan, err = a.PlanSamePerson(path, edited, SamePersonRequest{Identities: members, Name: "Sam Ortiz", Email: "sam@tidewater.example"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Sam Ortiz <sam@tidewater.example>"}; !slices.Equal(plan.Lines, want) || !slices.Equal(plan.At, []int{2}) {
		t.Fatalf("the second plan = %+v", plan)
	}

	// Nobody to show them as: said in words, and nothing planned.
	_, err = a.PlanSamePerson(path, edited, SamePersonRequest{Identities: members, Name: " ", Email: "sam@tidewater.example"})
	if err == nil || err.Error() != "Give the name and the address to show them as. Neither can hold < or >." {
		t.Fatalf("with no name: %v", err)
	}
}

// Taking one name apart leaves the others a line joined where they were, and writes nothing.
func TestPlanTakeApartKeepsTheOthers(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "o1.txt", "one")
	commitAs(fx, "sam", "pair@tidewater.example", "o2.txt", "two")
	commitAs(fx, "S. Ortiz", "pair@tidewater.example", "o3.txt", "three")
	path := openForMap(t, a, fx)
	text := "Sam Ortiz <sam@tidewater.example> <pair@tidewater.example>\n"
	edited := MailmapText{Text: text, Edited: true}
	if list, err := a.People(path, PeopleRequest{Mailmap: edited}); err != nil || list.Everyone != 1 {
		t.Fatalf("before: %d people, %v", list.Everyone, err)
	}

	plan, err := a.PlanTakeApart(path, edited, git.Identity{Name: "sam", Email: "pair@tidewater.example"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "Sam Ortiz <sam@tidewater.example> S. Ortiz <pair@tidewater.example>\n"; plan.Text != want || len(plan.Kept) != 1 || plan.Kept[0].Name != "S. Ortiz" || len(plan.Lost) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	// Through the planned text there are two people: Sam, still with the other spelling, and sam.
	list, err := a.People(path, PeopleRequest{Mailmap: MailmapText{Text: plan.Text, Edited: true}})
	if err != nil || list.Everyone != 2 {
		t.Fatalf("after: %d people, %v", list.Everyone, err)
	}
	if sam := person(t, list, "Sam Ortiz"); sam.Commits != 2 || len(sam.Identities) != 2 {
		t.Fatalf("Sam Ortiz after = %+v, want the other spelling still joined", sam)
	}
	if _, err := os.Stat(filepath.Join(fx.Dir, ".mailmap")); err == nil {
		t.Fatal("planning created a .mailmap")
	}

	// Someone nothing joins: the text comes back as it was.
	plan, err = a.PlanTakeApart(path, edited, git.Identity{Name: "Sam Ortiz", Email: "sam@tidewater.example"})
	if err != nil || plan.Text != text || len(plan.Removed) != 0 {
		t.Fatalf("an identity no line joins: %+v, %v", plan, err)
	}
}

func TestSuggestLeavesOutAnAddressUnlikeNamesShare(t *testing.T) {
	who := func(name, email string, commits int) Person {
		return Person{Name: name, Email: email, Commits: commits, Identities: []PersonIdentity{{Name: name, Email: email, Commits: commits}}}
	}
	// Three names with nothing in common on one address: a shared mailbox, not one person.
	got := suggest([]Person{who("Ada Park", "team@x.example", 3), who("Bo Chen", "team@x.example", 2), who("Cy Diaz", "team@x.example", 1)}, nil)
	if len(got.Offers) != 0 || len(got.Shared) != 1 || got.Shared[0].Why != "an address 3 unlike names use" {
		t.Fatalf("unlike names: offers %q, left out %+v", keys(got.Offers), got.Shared)
	}
	// Three spellings of one name on one address: worth asking.
	got = suggest([]Person{who("Ada Park", "ada@x.example", 3), who("ada", "ada@x.example", 2), who("Ada P.", "ada@x.example", 1)}, nil)
	if len(got.Offers) != 1 || len(got.Shared) != 0 {
		t.Fatalf("like names: offers %q, left out %q", keys(got.Offers), keys(got.Shared))
	}
	// Two people only, however unlike: asked, since a changed name looks just like this.
	got = suggest([]Person{who("Ada Park", "ada@x.example", 3), who("Robin Vale", "ada@x.example", 2)}, nil)
	if len(got.Offers) != 1 {
		t.Fatalf("two unlike names: offers %q", keys(got.Offers))
	}
	// GitHub's own per-person no-reply addresses have the person before the @.
	got = suggest([]Person{who("Ada Park", "1234+ada@users.noreply.github.com", 3), who("ada", "1234+ada@users.noreply.github.com", 2)}, nil)
	if len(got.Offers) != 1 {
		t.Fatalf("a personal no-reply address: offers %q, left out %+v", keys(got.Offers), got.Shared)
	}
	// The list is capped, and still says how many there are.
	var many []Person
	for i := range maxSuggestions + 7 {
		name := "Person " + string(rune('A'+i%26)) + string(rune('a'+i/26))
		many = append(many, who(name, name+"@one.example", 2), who(name, name+"@two.example", 1))
	}
	got = suggest(many, nil)
	if len(got.Offers) != maxSuggestions || got.Offered != maxSuggestions+7 {
		t.Fatalf("capped: %d shown of %d", len(got.Offers), got.Offered)
	}
}
