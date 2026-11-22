package app

import (
	"context"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/fsutil"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
	"github.com/git-revenge/gitrevenge/internal/trailerrules"
)

// A pinned key is a trailer key whose values show on the graph's rows, so "Changelog: fixed"
// can be read down the history without opening each commit. Nothing is pinned until someone
// pins it. A pin is kept in git config, the repository's own or the global one:
//
//	[gitrevenge-trailer "Changelog"]
//	    show = rows

// TrailerPin is one pinned key.
type TrailerPin struct {
	// Key is the key's ID and Label how the repository writes it.
	Key   string `json:"key"`
	Label string `json:"label"`
	// Kind is what most of the key's values are, or "" for a key no commit here has.
	Kind string `json:"kind"`
	// Scope is the config that pins it: "local", the repository's own, or "global".
	Scope string `json:"scope"`
}

// trailerPins keeps what each open repository's config says about its trailer keys, so that
// handing out graph rows doesn't run git each time. It holds until the app hears a config
// changed (see touch).
type trailerPins struct {
	mu   sync.Mutex
	kept map[string][]trailerrules.Rule
	// refreshing marks a repository whose reading is being worked out for its rows' chips.
	refreshing map[string]bool
}

// touch forgets what's kept for the repository at path, or for every repository with "".
func (ps *trailerPins) touch(path string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for p := range ps.kept {
		if path == "" || fsutil.SamePath(p, path) {
			delete(ps.kept, p)
		}
	}
}

// trailerRules returns what the repository's config says about its trailer keys.
func (a *App) trailerRules(ctx context.Context, r *session.Repo) []trailerrules.Rule {
	ps := &a.trailerPins
	ps.mu.Lock()
	rules, ok := ps.kept[r.Path]
	ps.mu.Unlock()
	if ok {
		return rules
	}
	rules, err := trailerrules.Load(ctx, r.Git, true)
	if err != nil {
		// A config that can't be read pins nothing, and is asked again next time.
		return nil
	}
	ps.mu.Lock()
	if ps.kept == nil {
		ps.kept = map[string][]trailerrules.Rule{}
	}
	ps.kept[r.Path] = rules
	ps.mu.Unlock()
	return rules
}

// TrailerPins returns the keys pinned to the graph's rows of the repository at path.
func (a *App) TrailerPins(path string) ([]TrailerPin, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return []TrailerPin{}, err
	}
	return a.trailerPinList(context.Background(), r), nil
}

func (a *App) trailerPinList(ctx context.Context, r *session.Repo) []TrailerPin {
	pins := []TrailerPin{}
	var reading *trailerindex.Reading
	for _, rule := range a.trailerRules(ctx, r) {
		if rule.Show != trailerrules.ShowRows {
			continue
		}
		if reading == nil {
			reading = a.trailerReading(ctx, r)
		}
		pin := TrailerPin{Key: rule.ID, Label: rule.Key, Scope: rule.Scope}
		if k, found := reading.Key(rule.ID); found {
			pin.Label, pin.Kind = k.Label, string(k.Kind)
		}
		pins = append(pins, pin)
	}
	return pins
}

// SetTrailerPin pins a key to the graph's rows of the repository at path, or unpins it. key
// is the key's ID or any spelling of it. A pin is written to the repository's own config.
// Unpinning a key the global config pins turns it off for this repository only, and leaves
// the global config as it is.
func (a *App) SetTrailerPin(path, key string, on bool) ([]TrailerPin, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return []TrailerPin{}, err
	}
	ctx := context.Background()
	// The key as the repository writes it, when it does: that's what the config will say.
	name := key
	if k, found := a.trailerReading(ctx, r).Key(key); found {
		name = k.Label
	}
	id := trailerindex.KeyID(key)
	show := trailerrules.ShowRows
	if !on {
		show = ""
		global, err := trailerrules.Load(ctx, r.Git, false)
		if err != nil {
			return a.trailerPinList(ctx, r), err
		}
		for _, rule := range global {
			if rule.ID == id && rule.Show == trailerrules.ShowRows {
				show = trailerrules.ShowNone
			}
		}
	}
	err = trailerrules.SetShow(ctx, r.Git, trailerrules.Local, name, show)
	a.trailerPins.touch(r.Path)
	if err != nil {
		return a.trailerPinList(ctx, r), err
	}
	a.emit("trailers:rows", r.Path)
	return a.trailerPinList(ctx, r), nil
}

// rowChips returns the chips to show on some commits' rows: the values each has for the
// pinned keys, by commit. It never waits for a reading: with none to hand, or one that's
// out of date, the rows go out as they can and "trailers:rows" says when to ask again.
func (a *App) rowChips(r *session.Repo, ids []string) map[string][]trailerindex.Chip {
	keys := trailerrules.Shown(a.trailerRules(context.Background(), r))
	if len(keys) == 0 {
		return nil
	}
	index := r.Trailers()
	reading, fresh := a.trailerReadings.peek(r.Path, index.Gen())
	// While the history is still being read the index changes every moment: the reading is
	// worked out again when the read ends, not for every batch.
	if !fresh && !index.Status().Reading {
		a.refreshChips(r.Path)
	}
	if reading == nil {
		return nil
	}
	var chips map[string][]trailerindex.Chip
	for _, id := range ids {
		if c := reading.Chips(id, keys); len(c) > 0 {
			if chips == nil {
				chips = map[string][]trailerindex.Chip{}
			}
			chips[id] = c
		}
	}
	return chips
}

// refreshChips works the repository's reading out again, away from whoever asked for rows,
// and then says the rows' chips may have changed. One at a time for a repository.
func (a *App) refreshChips(path string) {
	ps := &a.trailerPins
	ps.mu.Lock()
	if ps.refreshing[path] {
		ps.mu.Unlock()
		return
	}
	if ps.refreshing == nil {
		ps.refreshing = map[string]bool{}
	}
	ps.refreshing[path] = true
	ps.mu.Unlock()
	go func() {
		defer func() {
			ps.mu.Lock()
			delete(ps.refreshing, path)
			ps.mu.Unlock()
		}()
		r, err := a.openRepo(path)
		if err != nil {
			return
		}
		before, _ := a.trailerReadings.peek(r.Path, 0)
		// Most of what the app hears about a repository changes nothing its trailers are read
		// through: the rows are asked for again only when the reading is another one.
		if a.trailerReading(context.Background(), r) != before {
			a.emit("trailers:rows", r.Path)
		}
	}()
}

// pinsKept reports whether the repository at path is known to have a key pinned, without
// asking git: for whoever can't wait.
func (ps *trailerPins) pinsKept(path string) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return len(trailerrules.Shown(ps.kept[path])) > 0
}
