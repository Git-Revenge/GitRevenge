package app

import (
	"cmp"
	"context"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// The People view offers people who look like one person: the same name under another
// address, or the same address under another name. They are offers and nothing more.
// Nothing is joined unless someone says so, and "different people" is remembered.

// SuggestedPerson is one of the people a suggestion is about.
type SuggestedPerson struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Commits    int    `json:"commits"`
	CoAuthored int    `json:"coAuthored"`
	Bot        bool   `json:"bot"`
	// Identities are the names and addresses the history has for them, which a line
	// joining them has to cover.
	Identities []git.Identity `json:"identities"`
}

// Suggestion is two or more people who share a name or an address.
type Suggestion struct {
	// Key names it, for remembering an answer: "name:" and the name, or "address:" and the
	// address, in lower case.
	Key string `json:"key"`
	// Kind is "name" when they share a name, "address" when they share an address.
	Kind string `json:"kind"`
	// People are the ones who share it, most commits first.
	People []SuggestedPerson `json:"people"`
	// Why says why it isn't offered, for one that's left out.
	Why string `json:"why,omitempty"`
}

// Suggestions is what the People view offers, and what it leaves out.
type Suggestions struct {
	// Offers are the likely matches, most commits first, and Offered how many there are in
	// all: the list is capped.
	Offers  []Suggestion `json:"offers"`
	Offered int          `json:"offered"`
	// Shared are matches that aren't offered: bots, and addresses or names that many
	// different senders use. They can still be joined by hand.
	Shared      []Suggestion `json:"shared"`
	SharedCount int          `json:"sharedCount"`
	// Apart are matches someone answered "different people" to.
	Apart      []Suggestion `json:"apart"`
	ApartCount int          `json:"apartCount"`
}

// maxSuggestions caps each list. The Linux kernel still has 6,049 names under more than one
// address after its mailmap.
const maxSuggestions = 50

// PeopleSuggestions returns who looks like the same person in the repository at path, as
// the mailmap reads the history now.
func (a *App) PeopleSuggestions(path string, mm MailmapText) (Suggestions, error) {
	empty := Suggestions{Offers: []Suggestion{}, Shared: []Suggestion{}, Apart: []Suggestion{}}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	ctx := context.Background()
	history, err := a.historyPeople(ctx, r)
	if err != nil {
		return empty, err
	}
	m, err := r.Git.ReadMailmap(ctx, mm.Text, mm.Edited)
	if err != nil {
		return empty, err
	}
	people := groupPeople(history.ids, m, a.branchRules(r.Path).BotAuthor)
	return suggest(people, r.Git.KeptApart(ctx)), nil
}

// suggest finds the people who share a name or an address, and sorts them into what's
// offered, what's left out and what was answered already.
func suggest(people []Person, apart []string) Suggestions {
	byName, byAddress := map[string][]int{}, map[string][]int{}
	var names, addresses []string
	for i, p := range people {
		if n := strings.ToLower(strings.TrimSpace(p.Name)); n != "" {
			if byName[n] == nil {
				names = append(names, n)
			}
			byName[n] = append(byName[n], i)
		}
		if e := strings.ToLower(strings.TrimSpace(p.Email)); e != "" {
			if byAddress[e] == nil {
				addresses = append(addresses, e)
			}
			byAddress[e] = append(byAddress[e], i)
		}
	}
	var all []Suggestion
	add := func(kind, value string, members []int) {
		if len(members) < 2 {
			return
		}
		s := Suggestion{Key: kind + ":" + value, Kind: kind}
		for _, i := range members {
			p := people[i]
			sp := SuggestedPerson{Name: p.Name, Email: p.Email, Commits: p.Commits, CoAuthored: p.CoAuthored, Bot: p.Bot}
			for _, id := range p.Identities {
				sp.Identities = append(sp.Identities, git.Identity{Name: id.Name, Email: id.Email})
			}
			s.People = append(s.People, sp)
		}
		slices.SortStableFunc(s.People, func(x, y SuggestedPerson) int {
			return cmp.Compare(y.Commits+y.CoAuthored, x.Commits+x.CoAuthored)
		})
		s.Why = whyNotOffered(s)
		all = append(all, s)
	}
	for _, n := range names {
		add("name", n, byName[n])
	}
	for _, e := range addresses {
		add("address", e, byAddress[e])
	}
	weight := func(s Suggestion) int {
		n := 0
		for _, p := range s.People {
			n += p.Commits + p.CoAuthored
		}
		return n
	}
	slices.SortStableFunc(all, func(x, y Suggestion) int { return cmp.Or(cmp.Compare(weight(y), weight(x)), cmp.Compare(x.Key, y.Key)) })

	out := Suggestions{Offers: []Suggestion{}, Shared: []Suggestion{}, Apart: []Suggestion{}}
	put := func(list *[]Suggestion, count *int, s Suggestion) {
		*count++
		if len(*list) < maxSuggestions {
			*list = append(*list, s)
		}
	}
	for _, s := range all {
		switch {
		case slices.Contains(apart, s.Key):
			s.Why = ""
			put(&out.Apart, &out.ApartCount, s)
		case s.Why != "":
			put(&out.Shared, &out.SharedCount, s)
		default:
			put(&out.Offers, &out.Offered, s)
		}
	}
	return out
}

var (
	// noReply matches the part of an address before the @ that a service sends from, not a
	// person. GitHub's own per-person addresses ("1234+name@users.noreply.github.com") have
	// the person before the @ and don't match.
	noReply = regexp.MustCompile(`(?i)^(no[-_.]?reply|do[-_.]?not[-_.]?reply|notifications?|mailer-daemon|postmaster|bot|actions?)$`)
	// machineNames are names a fresh machine or a tool commits under, which different
	// people share by accident.
	machineNames = []string{"root", "admin", "administrator", "user", "unknown", "ubuntu", "debian", "developer", "dev", "test", "tester", "vagrant", "pi", "ec2-user", "your name", "no author", "(no author)", "author"}
	nameWord     = regexp.MustCompile(`[\p{L}\p{N}]{2,}`)
)

// whyNotOffered says why a match isn't put to the user as likely the same person, or ""
// when it is. These can still be joined by hand: the reasons are guesses about what's
// worth asking, not rules about who someone is.
func whyNotOffered(s Suggestion) string {
	for _, p := range s.People {
		if p.Bot {
			return "a bot"
		}
	}
	value := strings.TrimPrefix(strings.TrimPrefix(s.Key, "name:"), "address:")
	if s.Kind == "name" {
		if slices.Contains(machineNames, value) {
			return "a name many machines commit under"
		}
		return ""
	}
	if local, _, _ := strings.Cut(value, "@"); noReply.MatchString(local) {
		return "a no-reply address"
	}
	// Three or more names on one address, with no word they all share: a shared mailbox.
	if len(s.People) >= 3 {
		var common map[string]bool
		for _, p := range s.People {
			words := map[string]bool{}
			for _, w := range nameWord.FindAllString(strings.ToLower(p.Name), -1) {
				if common == nil || common[w] {
					words[w] = true
				}
			}
			common = words
		}
		if len(common) == 0 {
			return "an address " + strconv.Itoa(len(s.People)) + " unlike names use"
		}
	}
	return ""
}

// KeepApart remembers, in the repository's git config, that a suggestion is of different
// people, so it isn't offered again. With apart false it forgets that.
func (a *App) KeepApart(path, key string, apart bool) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return r.Git.KeepApart(context.Background(), key, apart)
}

// SamePersonRequest says which identities are one person, and how to show them.
type SamePersonRequest struct {
	// Identities are the names and addresses the history has for the people being joined.
	Identities []git.Identity `json:"identities"`
	// Name and Email are who they're shown as.
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PlanSamePerson works out the mailmap lines that make several identities read as one
// person, and the .mailmap text with them in place. It writes nothing: the view shows the
// lines, and on a yes takes the text as its edit, which is saved like any other. mm is the
// .mailmap as it's being edited.
func (a *App) PlanSamePerson(path string, mm MailmapText, req SamePersonRequest) (git.JoinPlan, error) {
	empty := git.JoinPlan{Lines: []string{}, At: []int{}, Named: []git.Identity{}, Blocked: []git.Identity{}}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	ctx := context.Background()
	history, err := a.historyPeople(ctx, r)
	if err != nil {
		return empty, err
	}
	text := mm.Text
	if !mm.Edited {
		if text, err = r.Git.ReadMailmapSource(ctx, git.MailmapWorkTree); err != nil {
			return empty, err
		}
	}
	read, err := r.Git.MailmapReader(ctx)
	if err != nil {
		return empty, err
	}
	plan, err := git.PlanJoin(text, read, history.ids, req.Identities, git.Identity{Name: strings.TrimSpace(req.Name), Email: strings.TrimSpace(req.Email)})
	if errors.Is(err, git.ErrJoinNeedsIdentity) {
		return empty, userError("Give the name and the address to show them as. Neither can hold < or >.")
	}
	if err != nil {
		return empty, err
	}
	return plan, nil
}

// PlanTakeApart works out the .mailmap text that takes one identity away from the person
// it's joined to, and nobody else: a line that joined others as well is replaced by lines
// that keep them. It writes nothing; the view takes the text as its edit. mm is the
// .mailmap as it's being edited.
func (a *App) PlanTakeApart(path string, mm MailmapText, who git.Identity) (git.ApartPlan, error) {
	empty := git.ApartPlan{Removed: []int{}, Added: []string{}, Kept: []git.Identity{}, Lost: []git.Identity{}}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	ctx := context.Background()
	history, err := a.historyPeople(ctx, r)
	if err != nil {
		return empty, err
	}
	text := mm.Text
	if !mm.Edited {
		if text, err = r.Git.ReadMailmapSource(ctx, git.MailmapWorkTree); err != nil {
			return empty, err
		}
	}
	read, err := r.Git.MailmapReader(ctx)
	if err != nil {
		return empty, err
	}
	return git.PlanApart(text, read, history.ids, who), nil
}
