package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/git-revenge/gitrevenge/internal/fsutil"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
	"github.com/git-revenge/gitrevenge/internal/links"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
	"github.com/git-revenge/gitrevenge/internal/trailerrules"
)

// Every commit's trailers are read once, in the background, and kept in a file of the
// app's own (see trailerindex): what Smart filters asks is answered from that, never by
// reading the history again. This file is the app's side of it: where the files go, and
// telling the window how far a read has got.

// trailerDir is where trailer indexes are kept: the operating system's cache folder. With
// the settings somewhere of a test's choosing, they go beside those instead, so a test
// never writes to the real cache.
func (a *App) trailerDir() string {
	if a.settingsPath != "" {
		return filepath.Join(filepath.Dir(a.settingsPath), "trailers")
	}
	return trailerindex.Dir()
}

// TrailerProgress is the payload of the "trailers:progress" event, and what TrailerStatus
// returns: how far reading a repository's trailers has got.
type TrailerProgress struct {
	Path string `json:"path"`
	trailerindex.Status
}

func (a *App) emitTrailerProgress(path string, st trailerindex.Status) {
	// A filter keeps more once more of the history is read, or a new commit's trailers are:
	// when a read ends, what the filter shows is worked out again. Not from here, though:
	// this is the reader's own goroutine, and closing a repository waits for its reader
	// while holding the lock that looking the repository up needs.
	if !st.Reading && a.trailerFilters.get(path) != nil {
		go func() {
			if r, err := a.openRepo(path); err == nil {
				a.applyTrailerFilter(r)
			}
		}()
	}
	// The same goes for the chips of pinned keys on the graph's rows.
	if !st.Reading && a.trailerPins.pinsKept(path) {
		a.refreshChips(path)
	}
	a.emit("trailers:progress", TrailerProgress{Path: path, Status: st})
}

// trailerReadings keeps each open repository's reading of its trailers (see
// trailerindex.Reading): the keys joined, the values told apart by kind. A reading holds
// while the index and everything it was read through stay as they were, which key says.
type trailerReadings struct {
	mu   sync.Mutex
	kept map[string]*keptReading
}

type keptReading struct {
	// making lets one reading be worked out at a time for a repository: the panel asks for
	// its keys and their values at once, and on a long history a reading takes seconds.
	making  sync.Mutex
	key     string
	reading *trailerindex.Reading
	// last is the same reading, for whoever only wants what's already there (see peek).
	last atomic.Pointer[trailerindex.Reading]
	// stale is set when the app hears the repository changed.
	stale atomic.Bool
	// checked is when the rules were last looked at, and gen the index's state then.
	// Looking takes three git commands, and the sidebar's filter asks on every keystroke,
	// so a look is good for a moment, unless the repository is known to have changed.
	checked time.Time
	gen     uint64
	// history is how many commits the history had when that was last known.
	history int
	// ids are the abbreviated commit ids readings have asked about, each with the full id of
	// the commit it names, or "" for one that names none. Placing them is most of a reading's
	// time on a long history (4.5 of 6.6 seconds on the Linux kernel), and an id that named a
	// commit still does, so each is asked once. One that named none is asked again once the
	// index has read new commits (idsGen), since the commit may have come with them (#897).
	ids    map[string]string
	idsGen uint64
}

// commitIDs answers what ask (git.CommitIDs) would, asking it only about ids it hasn't been
// asked about before, as of the index at gen. It runs while the reading is worked out, with
// making held.
func (k *keptReading) commitIDs(gen uint64, ids []string, ask func([]string) (map[string]string, error)) map[string]string {
	if k.ids == nil || k.idsGen != gen {
		kept := make(map[string]string, len(k.ids))
		for id, full := range k.ids {
			if full != "" {
				kept[id] = full
			}
		}
		k.ids, k.idsGen = kept, gen
	}
	var fresh []string
	for _, id := range ids {
		if _, asked := k.ids[id]; !asked {
			fresh = append(fresh, id)
		}
	}
	if len(fresh) > 0 {
		// A question git couldn't answer is asked again next time, and its ids read as text
		// meanwhile, as they did before anything was kept.
		if full, err := ask(fresh); err == nil {
			for _, id := range fresh {
				k.ids[id] = full[id]
			}
		}
	}
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		if full := k.ids[id]; full != "" {
			out[id] = full
		}
	}
	return out
}

// rulesGoodFor is how long a look at a repository's mailmap, bot patterns and smart links
// is trusted without looking again. A change the app hears of ends it at once (see touch).
const rulesGoodFor = 2 * time.Second

// touch says the repository at path may have changed what its trailers are read through,
// so the next reading looks at the rules again. An empty path says so of every repository,
// as a change to the global git config does.
func (rs *trailerReadings) touch(path string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for p, k := range rs.kept {
		if path == "" || fsutil.SamePath(p, path) {
			k.stale.Store(true)
		}
	}
}

// peek returns the reading last worked out for the repository at path, without working one
// out, and whether it still stands for an index at gen with nothing heard to have changed.
func (rs *trailerReadings) peek(path string, gen uint64) (*trailerindex.Reading, bool) {
	rs.mu.Lock()
	k := rs.kept[path]
	rs.mu.Unlock()
	if k == nil {
		return nil, false
	}
	reading := k.last.Load()
	return reading, reading != nil && reading.Gen() == gen && !k.stale.Load()
}

// trailerReading returns the repository's trailers as something to filter by, working it
// out if the index, the mailmap, the bot patterns, the smart links or the size of the
// history have changed since the last time.
func (a *App) trailerReading(ctx context.Context, r *session.Repo) *trailerindex.Reading {
	rs := &a.trailerReadings
	rs.mu.Lock()
	if rs.kept == nil {
		rs.kept = map[string]*keptReading{}
	}
	k := rs.kept[r.Path]
	if k == nil {
		k = &keptReading{}
		rs.kept[r.Path] = k
	}
	rs.mu.Unlock()
	k.making.Lock()
	defer k.making.Unlock()

	index := r.Trailers()
	if k.reading != nil && k.gen == index.Gen() && !k.stale.Load() && time.Since(k.checked) < rulesGoodFor {
		return k.reading
	}
	k.stale.Store(false)
	k.gen = index.Gen()
	// The moment counts from when the look is over: a first reading can take seconds.
	defer func() { k.checked = time.Now() }()
	bots := a.branchRules(r.Path)
	linked := a.linkConfig(ctx, r.Git)
	// How long the history is, for telling a key that's on nearly all of it. Under a smart
	// filter the graph holds only what the filter keeps, so its size isn't the history's:
	// the key a filter is on would be on every commit there is, and be held back. The
	// graph counts the history without its filter, and until it has, it's as long as it was.
	_, total := r.GraphStatus()
	if on, _, whole := r.Filtered(); on {
		if total = whole; total == 0 {
			total = k.history
		}
	}
	if total > 0 {
		k.history = total
	}
	// What the repository's own rules say about its keys, in its git config.
	said := a.trailerRules(ctx, r)
	sig, _ := json.Marshal([]any{index.Gen(), r.Git.MailmapState(ctx).Key(), bots.BotAuthorPatterns(), linked.Active(), total, said})
	if k.reading != nil && k.key == string(sig) {
		return k.reading
	}

	gen := index.Gen()
	ask := func(ids []string) (map[string]string, error) { return r.Git.CommitIDs(ctx, ids) }
	rules := trailerindex.Rules{
		Total:   total,
		Commits: func(ids []string) map[string]string { return k.commitIDs(gen, ids, ask) },
	}
	trailerrules.Apply(said, &rules)
	// Who a trailer names is who the graph would show: the mailmap git reads.
	if m, err := r.Git.ReadMailmap(ctx, "", false); err == nil {
		rules.Person = func(name, email string) (string, string, bool) {
			shown, _ := m.Resolve(git.Identity{Name: name, Email: email})
			return shown.Name, shown.Email, bots.BotAuthor(shown.Name, shown.Email)
		}
	}
	if whole := linked.Whole(links.InTrailers); whole != nil {
		rules.Tickets = func(value string) []trailerindex.Ticket {
			found := whole(value)
			if found == nil {
				return nil
			}
			out := make([]trailerindex.Ticket, len(found))
			for i, m := range found {
				out[i] = trailerindex.Ticket{Text: m.Text, Href: m.Href}
			}
			return out
		}
	}
	k.reading, k.key = index.Reading(rules), string(sig)
	k.last.Store(k.reading)
	return k.reading
}

// TrailerKeys is every trailer key of a repository, as something to filter by, with how
// far reading its history has got: until that's done the counts are of what's read so far.
type TrailerKeys struct {
	Keys   []trailerindex.Key  `json:"keys"`
	Status trailerindex.Status `json:"status"`
}

// TrailerKeys returns the trailer keys of the repository at path, most commits first:
// spellings joined, each with its kind and its counts, and the ones not worth offering as
// a filter marked with why.
func (a *App) TrailerKeys(path string) (TrailerKeys, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerKeys{Keys: []trailerindex.Key{}}, err
	}
	reading := a.trailerReading(context.Background(), r)
	return TrailerKeys{Keys: reading.Keys(), Status: r.Trailers().Status()}, nil
}

// A reference by number, which has to match whole: "#1" isn't in "#12".
var wholeNumber = regexp.MustCompile(`^#\d+$`)

// trailerSearch answers a search written "Key: value" from the trailer index, when Key is
// one the repository's commits write, under any spelling of it. ok is false when the search
// isn't one for the index: it isn't written that way, the key is nobody's, or the history is
// still being read and the index would answer for only part of it.
//
// What a repository writes decides what's a trailer search, not a list of keys: "Changelog:
// fixed" is one where commits have a Changelog trailer. A search like "fix: crash" stays a
// search of subjects unless some commit has a trailer spelled "Fix".
//
// Values are matched as they're shown, so a person is found under the name the mailmap
// gives them, whichever name the commit used.
func (a *App) trailerSearch(ctx context.Context, r *session.Repo, g *graph.Graph, query string) (graph.SearchResult, bool) {
	// A leading "-" asks for the commits that don't match.
	q := strings.TrimSpace(query)
	not := strings.HasPrefix(q, "-")
	key, value, ok := git.SplitTrailerQuery(strings.TrimPrefix(q, "-"))
	if !ok || !r.Trailers().Status().Done {
		return graph.SearchResult{}, false
	}
	// "has:reviewed-by" asks for any value of a key.
	has := strings.EqualFold(key, "has") && value != ""
	if has {
		key, value = value, ""
	}
	reading := a.trailerReading(ctx, r)
	k, found := reading.Key(key)
	if !found {
		return graph.SearchResult{}, false
	}
	// "closed: #14" finds Closes and Close too, since endings are one key. But a key typed in
	// a spelling no commit uses may not be meant as one: "fix: crash" is the start of a
	// subject, even where commits write Fixes. Said outright, with has: or a "-", it is one.
	written := slices.ContainsFunc(k.Spellings, func(s trailerindex.Spelling) bool { return strings.EqualFold(s.Text, key) })
	sure := has || not || (written && k.Held != trailerindex.HeldRare)
	var match func(trailerindex.Value) bool
	switch {
	case value == "":
	case wholeNumber.MatchString(value):
		whole := regexp.MustCompile(`(^|\D)` + regexp.QuoteMeta(value) + `(\D|$)`)
		match = func(v trailerindex.Value) bool { return whole.MatchString(v.Label) }
	default:
		lower := strings.ToLower(value)
		match = func(v trailerindex.Value) bool { return trailerindex.ContainsFold(v.Label, lower) }
	}
	ids := reading.CommitsWhere(k.ID, match)
	if not {
		return rowsBut(g, ids, maxSearchRows), true
	}
	res := rowsOf(g, ids, maxSearchRows)
	if sure {
		return res, true
	}
	// A key on a commit or two is likely a sentence with a colon in, and the search may be
	// for words in a subject after all, as "docs: the plan" is. Both are looked for.
	text := g.Search(query, maxSearchRows)
	extra := 0
	for _, row := range res.Rows {
		if _, there := slices.BinarySearch(text.Rows, row); !there {
			text.Rows = append(text.Rows, row)
			extra++
		}
	}
	slices.Sort(text.Rows)
	text.Total += extra
	if len(text.Rows) > maxSearchRows {
		text.Rows = text.Rows[:maxSearchRows]
	}
	return text, true
}

// TrailerPanelKey is one key in the Smart filters panel, with the first of its values.
type TrailerPanelKey struct {
	Key trailerindex.Key `json:"key"`
	// Top are the values to show under it, people before bots, and More how many more it
	// has, or with a query how many more match.
	Top  []trailerindex.Value `json:"top"`
	More int                  `json:"more"`
}

// TrailerPanel is what the sidebar's Smart filters panel shows.
type TrailerPanel struct {
	// Keys are the keys offered as filters, most commits first: all of them, or with a
	// query the ones it matches by their name or by a value. MoreKeys is how many more
	// there are than the panel is given at once.
	Keys     []TrailerPanelKey `json:"keys"`
	MoreKeys int               `json:"moreKeys"`
	// Held are the keys not offered, each with why.
	Held []trailerindex.Key `json:"held"`
	// Matches is how many keys and values the query matches, for the rail's count; 0 with
	// no query.
	Matches int `json:"matches"`
	// Status says how far reading the history has got: until it's done, counts are of
	// what's read so far.
	Status trailerindex.Status `json:"status"`
	// Care is how many questions about the repository's trailers are open, for the pill
	// that leads to Trailer care.
	Care int `json:"care"`
}

const (
	// trailerPanelKeys is how many keys the panel is given at once. The Linux kernel
	// offers nine hundred; past the first couple of hundred the filter box is the way in.
	trailerPanelKeys = 200
	// trailerPanelValues is how many values show under a key when the request doesn't say.
	trailerPanelValues = 6
)

// TrailerPanel returns what the Smart filters panel shows for the repository at path. query
// is the sidebar's filter: it narrows the keys to those whose name contains it, and the
// rest to their values that do, whatever the case. perKey is how many values to give for
// each key; 0 or less means the default.
func (a *App) TrailerPanel(path, query string, perKey int) (TrailerPanel, error) {
	out := TrailerPanel{Keys: []TrailerPanelKey{}, Held: []trailerindex.Key{}}
	r, err := a.openRepo(path)
	if err != nil {
		return out, err
	}
	reading := a.trailerReading(context.Background(), r)
	out.Status = r.Trailers().Status()
	_, out.Care = a.careOf(context.Background(), r, reading)
	if perKey <= 0 {
		perKey = trailerPanelValues
	}
	q := strings.ToLower(strings.TrimSpace(query))
	for _, k := range reading.Keys() {
		if k.Held != "" {
			out.Held = append(out.Held, k)
			continue
		}
		shown, matches, ok := panelKey(reading, k, q, perKey)
		if !ok {
			continue
		}
		out.Matches += matches
		if len(out.Keys) >= trailerPanelKeys {
			out.MoreKeys++
			continue
		}
		out.Keys = append(out.Keys, shown)
	}
	return out, nil
}

// panelKey is one key as the panel shows it under a filter q, in lower case: with the
// first n of its values when its name matches or there's no filter, and otherwise with the
// first n of its values that match. matches counts what the filter matched, the key's name
// and its values, and ok is false when it matched nothing of the key.
func panelKey(reading *trailerindex.Reading, k trailerindex.Key, q string, n int) (shown TrailerPanelKey, matches int, ok bool) {
	byValue := func(v trailerindex.Value) bool { return trailerindex.ContainsFold(v.Label, q) }
	byName := q == "" || trailerindex.ContainsFold(k.Label, q) || slices.ContainsFunc(k.Spellings, func(s trailerindex.Spelling) bool { return trailerindex.ContainsFold(s.Text, q) })
	if byName {
		top, all := reading.Top(k.ID, n, nil)
		if q != "" {
			// Its values count too where they match, though all of them show.
			_, hits := reading.Top(k.ID, 0, byValue)
			matches = 1 + hits
		}
		return TrailerPanelKey{Key: k, Top: top, More: all - len(top)}, matches, true
	}
	top, all := reading.Top(k.ID, n, byValue)
	return TrailerPanelKey{Key: k, Top: top, More: all - len(top)}, all, all > 0
}

// TrailerMore returns one key of the Smart filters panel with its first n values, for
// "24 more…". key is the key's ID and query the sidebar's filter, as in TrailerPanel.
func (a *App) TrailerMore(path, key, query string, n int) (TrailerPanelKey, error) {
	empty := TrailerPanelKey{Top: []trailerindex.Value{}}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	reading := a.trailerReading(context.Background(), r)
	k, found := reading.Key(key)
	if !found {
		return empty, userError("No commit has that trailer any more.")
	}
	shown, _, _ := panelKey(reading, k, strings.ToLower(strings.TrimSpace(query)), max(n, 1))
	return shown, nil
}

// TrailerValues is a page of one key's values.
type TrailerValues struct {
	Values []trailerindex.Value `json:"values"`
	// Matching is how many values the query matches: the length of the list being paged.
	Matching int `json:"matching"`
}

// trailerValuesPage is how many values a page holds when the request doesn't say.
const trailerValuesPage = 100

// TrailerValues returns the values of one trailer key of the repository at path, most
// commits first. key is a key's ID or any spelling of it. query narrows the list to values
// whose text contains it, whatever the case. A limit of 0 or less means the default.
func (a *App) TrailerValues(path, key, query string, offset, limit int) (TrailerValues, error) {
	empty := TrailerValues{Values: []trailerindex.Value{}}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	values := a.trailerReading(context.Background(), r).Values(key)
	if q := strings.ToLower(strings.TrimSpace(query)); q != "" {
		values = slices.DeleteFunc(values, func(v trailerindex.Value) bool { return !strings.Contains(strings.ToLower(v.Label), q) })
	}
	if limit <= 0 {
		limit = trailerValuesPage
	}
	from := min(max(offset, 0), len(values))
	return TrailerValues{Values: append([]trailerindex.Value{}, values[from:min(from+limit, len(values))]...), Matching: len(values)}, nil
}

// TrailerStatus says how far reading the trailers of the repository at path has got, for a
// window that wasn't listening when the read began.
func (a *App) TrailerStatus(path string) (TrailerProgress, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerProgress{Path: path}, err
	}
	return TrailerProgress{Path: r.Path, Status: r.Trailers().Status()}, nil
}
