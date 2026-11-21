package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/links"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
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
	bots := a.branchRules(r.Path)
	linked := a.linkConfig(ctx, r.Git)
	_, total := r.GraphStatus()
	sig, _ := json.Marshal([]any{index.Gen(), r.Git.MailmapState(ctx).Key(), bots.BotAuthorPatterns(), linked.Active(), total})
	if k.reading != nil && k.key == string(sig) {
		return k.reading
	}

	rules := trailerindex.Rules{
		Total: total,
		Commits: func(ids []string) map[string]string {
			full, err := r.Git.CommitIDs(ctx, ids)
			if err != nil {
				return nil
			}
			return full
		},
	}
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
