package app

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// The People view asks who a repository's history names and how its mailmap reads them.
// The history is read once and kept until a ref moves; everything else here is worked out
// from that and the mailmap text, which may be an edit that isn't saved yet. So the view
// can ask after every keystroke.

// historyPeople is every identity a history uses, as its commits have them.
type historyPeople struct {
	ids     []git.HistoryIdentity
	commits int
}

// peopleReads keeps the latest read for each repository, and lets callers that ask while
// one is under way wait for it instead of starting another: on a long history a read
// takes seconds, and the view asks several things at once.
type peopleReads struct {
	mu      sync.Mutex
	kept    map[string]*historyPeople // by repository path; key says what it was read at
	keys    map[string]string
	reading map[string]*peopleRead
}

type peopleRead struct {
	done chan struct{}
	res  *historyPeople
	err  error
}

// historyPeople returns who the history of r names, reading it if a ref has moved since
// the last read.
func (a *App) historyPeople(ctx context.Context, r *session.Repo) (*historyPeople, error) {
	state, err := refState(ctx, r.Git)
	if err != nil {
		return nil, err
	}
	p := &a.people
	p.mu.Lock()
	if p.keys[r.Path] == state && p.kept[r.Path] != nil {
		res := p.kept[r.Path]
		p.mu.Unlock()
		return res, nil
	}
	key := r.Path + "\x00" + state
	if w := p.reading[key]; w != nil {
		p.mu.Unlock()
		<-w.done
		return w.res, w.err
	}
	w := &peopleRead{done: make(chan struct{})}
	if p.reading == nil {
		p.reading, p.kept, p.keys = map[string]*peopleRead{}, map[string]*historyPeople{}, map[string]string{}
	}
	p.reading[key] = w
	p.mu.Unlock()

	ids, commits, err := r.Git.HistoryIdentities(ctx)
	if err == nil {
		w.res = &historyPeople{ids: ids, commits: commits}
	}
	w.err = err
	p.mu.Lock()
	delete(p.reading, key)
	if err == nil {
		p.kept[r.Path], p.keys[r.Path] = w.res, state
	}
	p.mu.Unlock()
	close(w.done)
	return w.res, w.err
}

// MailmapText is the mailmap text to read names through.
type MailmapText struct {
	// Text stands in for the work tree's .mailmap when Edited is set: what the view is
	// editing and hasn't saved. Otherwise the saved file is read.
	Text   string `json:"text"`
	Edited bool   `json:"edited"`
}

// PeopleRequest asks for a page of people.
type PeopleRequest struct {
	Mailmap MailmapText `json:"mailmap"`
	// Query narrows the list to people with a name or address containing it, as shown or
	// as any commit has it.
	Query string `json:"query"`
	// Offset and Limit pick the page. A Limit of 0 or less means the default.
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// PersonIdentity is one identity the history uses, under the person it's read as.
type PersonIdentity struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Commits    int    `json:"commits"`
	CoAuthored int    `json:"coAuthored"`
	// Lines are the mailmap lines that read it as that person; none when it's shown as
	// written.
	Lines []git.MailmapRef `json:"lines"`
}

// Person is one person as the mailmap reads the history.
type Person struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	// Commits is how many commits they wrote, merges included, and CoAuthored how many
	// more credit them in a trailer.
	Commits    int `json:"commits"`
	CoAuthored int `json:"coAuthored"`
	// Bot marks someone the bot author patterns match.
	Bot bool `json:"bot"`
	// Identities are every name and address read as this person, most used first. One
	// that's the same as Name and Email is them as written.
	Identities []PersonIdentity `json:"identities"`
}

// PeopleList is a page of people, and what the whole history holds.
type PeopleList struct {
	People []Person `json:"people"`
	// Matching is how many people the query matches: the length of the list being paged.
	Matching int `json:"matching"`
	// People and Bots count everyone, and Identities every name and address before the
	// mailmap. Commits is how many commits were read.
	Everyone   int `json:"everyone"`
	Bots       int `json:"bots"`
	Identities int `json:"identities"`
	Commits    int `json:"commits"`
}

// peoplePage is how many people a page holds when the request doesn't say.
const peoplePage = 200

// People returns who the history of the repository at path names, as the mailmap reads
// them, most commits first.
func (a *App) People(path string, req PeopleRequest) (PeopleList, error) {
	empty := PeopleList{People: []Person{}}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	ctx := context.Background()
	history, err := a.historyPeople(ctx, r)
	if err != nil {
		return empty, err
	}
	m, err := r.Git.ReadMailmap(ctx, req.Mailmap.Text, req.Mailmap.Edited)
	if err != nil {
		return empty, err
	}
	people := groupPeople(history.ids, m, a.branchRules(r.Path).BotAuthor)
	out := PeopleList{People: []Person{}, Everyone: len(people), Identities: len(history.ids), Commits: history.commits}
	for _, p := range people {
		if p.Bot {
			out.Bots++
		}
	}
	if q := strings.ToLower(strings.TrimSpace(req.Query)); q != "" {
		people = slices.DeleteFunc(people, func(p Person) bool { return !personMatches(p, q) })
	}
	out.Matching = len(people)
	limit := req.Limit
	if limit <= 0 {
		limit = peoplePage
	}
	from := min(max(req.Offset, 0), len(people))
	out.People = append(out.People, people[from:min(from+limit, len(people))]...)
	return out, nil
}

// groupPeople reads each identity through the mailmap and gathers the ones read alike,
// most commits first.
func groupPeople(ids []git.HistoryIdentity, m *git.Mailmap, bot func(name, email string) bool) []Person {
	at := map[git.Identity]int{}
	var people []Person
	for _, h := range ids {
		shown, by := m.Resolve(h.Identity)
		i, ok := at[shown]
		if !ok {
			i = len(people)
			at[shown] = i
			people = append(people, Person{Name: shown.Name, Email: shown.Email, Bot: bot(shown.Name, shown.Email)})
		}
		p := &people[i]
		p.Commits += h.Authored
		p.CoAuthored += h.CoAuthored
		if by == nil {
			by = []git.MailmapRef{}
		}
		p.Identities = append(p.Identities, PersonIdentity{Name: h.Name, Email: h.Email, Commits: h.Authored, CoAuthored: h.CoAuthored, Lines: by})
	}
	for i := range people {
		slices.SortStableFunc(people[i].Identities, func(x, y PersonIdentity) int {
			return cmp.Compare(y.Commits+y.CoAuthored, x.Commits+x.CoAuthored)
		})
	}
	slices.SortStableFunc(people, func(x, y Person) int {
		return cmp.Or(cmp.Compare(y.Commits+y.CoAuthored, x.Commits+x.CoAuthored), cmp.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name)))
	})
	return people
}

// personMatches reports whether q, in lower case, is in any of a person's names or addresses.
func personMatches(p Person, q string) bool {
	has := func(s string) bool { return strings.Contains(strings.ToLower(s), q) }
	if has(p.Name) || has(p.Email) {
		return true
	}
	for _, id := range p.Identities {
		if has(id.Name) || has(id.Email) {
			return true
		}
	}
	return false
}

// MailmapSources lists where git reads the repository's mailmap from. The work tree's
// .mailmap is always first, whether or not it exists yet.
func (a *App) MailmapSources(path string) ([]git.MailmapSource, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return []git.MailmapSource{}, err
	}
	return r.Git.MailmapSources(context.Background())
}

// ReadMailmapFile returns a mailmap source's text, "" when there's nothing there.
func (a *App) ReadMailmapFile(path, source string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	return r.Git.ReadMailmapSource(context.Background(), source)
}

// MailmapNotes says what each line of one mailmap source does: what it joins, that it
// matches nobody, that a later line replaces it, or why git skips it. mm is the work
// tree's .mailmap as it's being edited, which decides what the lines of every source still
// do; source is the one whose lines are wanted.
func (a *App) MailmapNotes(path string, mm MailmapText, source string) ([]git.MailmapNote, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return []git.MailmapNote{}, err
	}
	ctx := context.Background()
	history, err := a.historyPeople(ctx, r)
	if err != nil {
		return []git.MailmapNote{}, err
	}
	m, err := r.Git.ReadMailmap(ctx, mm.Text, mm.Edited)
	if err != nil {
		return []git.MailmapNote{}, err
	}
	text := mm.Text
	if source != git.MailmapWorkTree || !mm.Edited {
		if text, err = r.Git.ReadMailmapSource(ctx, source); err != nil {
			return []git.MailmapNote{}, err
		}
	}
	notes := m.Notes(source, text, history.ids)
	if notes == nil {
		notes = []git.MailmapNote{}
	}
	return notes, nil
}

// WhoAnswer is who one identity is read as.
type WhoAnswer struct {
	// Written is the identity as a commit has it, and Shown who the mailmap reads it as.
	Written git.Identity `json:"written"`
	Shown   git.Identity `json:"shown"`
	// Lines are the mailmap lines that decide it; none when it's shown as written.
	Lines []git.MailmapRef `json:"lines"`
	// Commits and CoAuthored are how often the history uses the written identity; both are
	// 0 for one it doesn't use.
	Commits    int `json:"commits"`
	CoAuthored int `json:"coAuthored"`
	// Known is whether the history uses it at all.
	Known bool `json:"known"`
}

// maxWhoAnswers caps the answers to a question that matches many identities.
const maxWhoAnswers = 8

// WhoIs answers "who is this?" for a name, an address, or "Name <address>" as a commit has
// them: who the mailmap reads it as, and by which lines. A name or an address alone is
// looked for among the identities the history uses, most used first. Something the history
// doesn't use is still answered when it has an address, since a mailmap line can be
// written for it.
func (a *App) WhoIs(path string, mm MailmapText, query string) ([]WhoAnswer, error) {
	none := []WhoAnswer{}
	query = strings.TrimSpace(query)
	if query == "" {
		return none, nil
	}
	r, err := a.openRepo(path)
	if err != nil {
		return none, err
	}
	ctx := context.Background()
	history, err := a.historyPeople(ctx, r)
	if err != nil {
		return none, err
	}
	m, err := r.Git.ReadMailmap(ctx, mm.Text, mm.Edited)
	if err != nil {
		return none, err
	}
	answer := func(h git.HistoryIdentity, known bool) WhoAnswer {
		shown, by := m.Resolve(h.Identity)
		if by == nil {
			by = []git.MailmapRef{}
		}
		return WhoAnswer{Written: h.Identity, Shown: shown, Lines: by, Commits: h.Authored, CoAuthored: h.CoAuthored, Known: known}
	}

	var asked git.Identity
	exact := strings.Contains(query, "<")
	switch {
	case exact:
		asked = git.ParseIdentity(query)
	case strings.Contains(query, "@"):
		asked.Email = query
	default:
		asked.Name = query
	}
	same := strings.EqualFold
	var found []git.HistoryIdentity
	for _, h := range history.ids {
		switch {
		case exact && same(h.Name, asked.Name) && same(h.Email, asked.Email),
			!exact && asked.Email != "" && same(h.Email, asked.Email),
			!exact && asked.Name != "" && same(h.Name, asked.Name):
			found = append(found, h)
		}
	}
	slices.SortStableFunc(found, func(x, y git.HistoryIdentity) int {
		return cmp.Compare(y.Authored+y.CoAuthored, x.Authored+x.CoAuthored)
	})
	out := none
	for _, h := range found[:min(len(found), maxWhoAnswers)] {
		out = append(out, answer(h, true))
	}
	// Nothing in the history, but there's an address to look up: a line can still be
	// written for a commit that hasn't arrived, or was asked about by hand.
	if len(out) == 0 && asked.Email != "" {
		out = append(out, answer(git.HistoryIdentity{Identity: asked}, false))
	}
	return out, nil
}

// SetPeopleTab remembers which repository the People tab shows, for the next launch; ""
// records that the tab is closed.
func (a *App) SetPeopleTab(repo string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	err = m.Settings().Update(func(s *settings.Settings) { s.PeopleTab = repo })
	// The repository on screen is the one whose people are showing.
	m.Rewatch()
	return err
}

// SaveMailmap writes the work tree's .mailmap. base is the text it had when editing began;
// if someone else changed the file since, nothing is written.
//
// A repository with no .mailmap gets one only with create set, which the view sends after
// the user has agreed to the new file. Without it nothing is written.
func (a *App) SaveMailmap(path, base, text string, create bool) error {
	// What trailers are read through may have changed: the next reading looks again.
	defer a.trailerReadings.touch("")
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	err = r.Git.WriteMailmap(context.Background(), base, text, create)
	switch {
	case errors.Is(err, git.ErrMailmapChanged):
		return userError("The .mailmap changed on disk since you started editing, so it wasn't saved. Reload it to see the change, then make your edits again.")
	case errors.Is(err, git.ErrNoMailmap):
		return userError("This repository has no .mailmap, and one is only created when you say so. Nothing was written.")
	case errors.Is(err, git.ErrSymlink):
		return userError("This repository's .mailmap is a symbolic link. Git doesn't read a mailmap through a link, so the app won't write through one either: it could change a file outside the repository. Nothing was written. Replace the link with a file of its own to keep a mailmap here.")
	}
	return err
}
