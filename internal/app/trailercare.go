package app

import (
	"context"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
	"github.com/git-revenge/gitrevenge/internal/trailerrules"
)

// Trailer care is where a repository says how its trailers should be read: the questions
// the app won't answer for it, what was joined without asking, and every key with where it
// shows and why. Nothing here changes a commit. An answer is a rule in the repository's git
// config, which only says how to read what's already written.

// CareKey is one key in Trailer care's table.
type CareKey struct {
	Key trailerindex.Key `json:"key"`
	// Where is where the key shows: "rows" (on the graph's rows, and in the panel),
	// "panel" or "held".
	Where string `json:"where"`
	// KindSaid and WhereSaid mark what a rule says. Without them the kind and the place
	// are the app's guesses.
	KindSaid  bool `json:"kindSaid"`
	WhereSaid bool `json:"whereSaid"`
	// Also are the keys a rule reads as this one, as the rule writes them: each can be
	// taken back.
	Also []string `json:"also"`
	// Scope is the config its rule is in, "local" or "global", or "" with no rule.
	Scope string `json:"scope"`
}

// TrailerCare is what Trailer care shows.
type TrailerCare struct {
	// Questions are the open ones, the most telling first, and Open how many there are:
	// a long history can raise more than are listed.
	Questions []trailerindex.Question `json:"questions"`
	Open      int                     `json:"open"`
	// People are those written two ways, the ones on the most commits first, and
	// PeopleOpen how many there are. They aren't counted among the questions: a rule
	// can't answer them, and the mailmap, in the People view, can.
	People     []trailerindex.Question `json:"people"`
	PeopleOpen int                     `json:"peopleOpen"`
	// Joined is what was joined without asking.
	Joined trailerindex.Joined `json:"joined"`
	// Keys is every key, most commits first.
	Keys []CareKey `json:"keys"`
	// Status says how far reading the history has got: until it's done, this is so far.
	Status trailerindex.Status `json:"status"`
}

const (
	// careQuestions is how many questions are listed at once. The rest are counted.
	careQuestions = 60
	// carePeople is how many people written two ways are shown: the People view has them all.
	carePeople = 5
)

// trailerCares keeps the questions each open repository's reading raises: finding them
// looks at every pair of keys, and the panel's pill asks how many there are each time it
// draws. They hold for as long as the reading does, which is made again whenever the
// history, the mailmap or a rule changes.
type trailerCares struct {
	mu   sync.Mutex
	kept map[string]keptCare
}

type keptCare struct {
	reading   *trailerindex.Reading
	questions []trailerindex.Question
	open      int
}

// forget drops what's kept for a repository that's closing.
func (cs *trailerCares) forget(path string) {
	cs.mu.Lock()
	delete(cs.kept, path)
	cs.mu.Unlock()
}

// careOf returns the open questions of a repository's reading, and how many there are.
func (a *App) careOf(ctx context.Context, r *session.Repo, reading *trailerindex.Reading) ([]trailerindex.Question, int) {
	cs := &a.trailerCares
	cs.mu.Lock()
	k, ok := cs.kept[r.Path]
	cs.mu.Unlock()
	if ok && k.reading == reading {
		return k.questions, k.open
	}
	rules := a.trailerRules(ctx, r)
	questions, open := reading.Care(func(x, y string) bool { return trailerrules.KeptApart(rules, x, y) }, careQuestions)
	cs.mu.Lock()
	if cs.kept == nil {
		cs.kept = map[string]keptCare{}
	}
	cs.kept[r.Path] = keptCare{reading: reading, questions: questions, open: open}
	cs.mu.Unlock()
	return questions, open
}

// TrailerCare returns what Trailer care shows for the repository at path.
func (a *App) TrailerCare(path string) (TrailerCare, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerCare{Questions: []trailerindex.Question{}, People: []trailerindex.Question{}, Keys: []CareKey{}, Joined: trailerindex.Joined{Spellings: []trailerindex.Key{}}}, err
	}
	return a.trailerCare(context.Background(), r), nil
}

func (a *App) trailerCare(ctx context.Context, r *session.Repo) TrailerCare {
	reading := a.trailerReading(ctx, r)
	rules := a.trailerRules(ctx, r)
	out := TrailerCare{Joined: reading.Joined(), Keys: []CareKey{}, Status: r.Trailers().Status()}
	out.Questions, out.Open = a.careOf(ctx, r, reading)
	out.People, out.PeopleOpen = reading.People(carePeople)
	byID := map[string]trailerrules.Rule{}
	for _, rule := range rules {
		byID[rule.ID] = rule
	}
	for _, k := range reading.Keys() {
		row := CareKey{Key: k, Where: "panel", Also: []string{}}
		if k.Held != "" {
			row.Where = "held"
		}
		if rule, ok := byID[k.ID]; ok {
			row.Scope = rule.Scope
			row.Also = append(row.Also, rule.Also...)
			row.KindSaid = rule.Kind != ""
			switch rule.Show {
			case trailerrules.ShowRows:
				row.Where, row.WhereSaid = "rows", true
			case trailerrules.ShowPanel, trailerrules.ShowHeld:
				row.WhereSaid = true
			}
		}
		out.Keys = append(out.Keys, row)
	}
	return out
}

// ruleChanged says a rule of the repository's was written: what was kept is looked at
// again, and the graph's rows may show other chips.
func (a *App) ruleChanged(r *session.Repo) {
	a.trailerPins.touch(r.Path)
	a.trailerReadings.touch(r.Path)
	a.emit("trailers:rows", r.Path)
}

// keyAsWritten returns how the repository writes a key, for naming it in a rule: its most
// written spelling, or the name as given for a key no commit has.
func keyAsWritten(reading *trailerindex.Reading, key string) string {
	if k, found := reading.Key(key); found {
		// A key read as another by a rule is named by its own spelling, not the other's.
		for _, sp := range k.Spellings {
			if trailerindex.KeyID(sp.Text) == trailerindex.KeyID(key) {
				return sp.Text
			}
		}
		if trailerindex.KeyID(k.Label) == trailerindex.KeyID(key) {
			return k.Label
		}
	}
	return key
}

// AnswerTrailerQuestion answers a question about two keys of the repository at path: with
// join, other is read as key from now on; without, the two are kept apart and the question
// isn't asked again. Either is a rule in the repository's own git config.
func (a *App) AnswerTrailerQuestion(path, key, other string, join bool) (TrailerCare, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerCare{}, err
	}
	ctx := context.Background()
	reading := a.trailerReading(ctx, r)
	name, otherName := keyAsWritten(reading, key), keyAsWritten(reading, other)
	if join {
		err = trailerrules.Join(ctx, r.Git, trailerrules.Local, name, otherName)
	} else {
		err = trailerrules.KeepApart(ctx, r.Git, trailerrules.Local, name, otherName)
	}
	a.ruleChanged(r)
	if err != nil {
		return a.trailerCare(ctx, r), userError("That couldn’t be written to the repository’s git config: " + err.Error())
	}
	return a.trailerCare(ctx, r), nil
}

// UndoTrailerJoin takes back a join in the repository at path: other is read as itself
// again, and the question about the two may be asked again.
func (a *App) UndoTrailerJoin(path, key, other string) (TrailerCare, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerCare{}, err
	}
	ctx := context.Background()
	// The rule may be the repository's or the global one: it's taken back where it is.
	scope := trailerrules.Local
	for _, rule := range a.trailerRules(ctx, r) {
		if rule.ID == trailerindex.KeyID(key) {
			scope = rule.Scope
		}
	}
	err = trailerrules.Unjoin(ctx, r.Git, scope, key, other)
	a.ruleChanged(r)
	if err != nil {
		return a.trailerCare(ctx, r), userError("That couldn’t be written to git config: " + err.Error())
	}
	return a.trailerCare(ctx, r), nil
}

// SetTrailerKind says what the values of a key of the repository at path are: "people",
// "tickets", "commits", "labels", "identifiers" or "text", or "" to leave it to be guessed.
func (a *App) SetTrailerKind(path, key, kind string) (TrailerCare, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerCare{}, err
	}
	ctx := context.Background()
	err = trailerrules.SetKind(ctx, r.Git, trailerrules.Local, keyAsWritten(a.trailerReading(ctx, r), key), kind)
	a.ruleChanged(r)
	if err != nil {
		return a.trailerCare(ctx, r), userError("That couldn’t be written to the repository’s git config: " + err.Error())
	}
	return a.trailerCare(ctx, r), nil
}

// SetTrailerPlace says where a key of the repository at path shows: "rows", "panel" or
// "held", or "" to leave it to be guessed.
func (a *App) SetTrailerPlace(path, key, place string) (TrailerCare, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerCare{}, err
	}
	ctx := context.Background()
	switch place {
	case "", trailerrules.ShowRows, trailerrules.ShowPanel, trailerrules.ShowHeld:
	default:
		return a.trailerCare(ctx, r), userError("A key shows on rows, in the panel, or is held back.")
	}
	err = trailerrules.SetShow(ctx, r.Git, trailerrules.Local, keyAsWritten(a.trailerReading(ctx, r), key), place)
	a.ruleChanged(r)
	if err != nil {
		return a.trailerCare(ctx, r), userError("That couldn’t be written to the repository’s git config: " + err.Error())
	}
	return a.trailerCare(ctx, r), nil
}
