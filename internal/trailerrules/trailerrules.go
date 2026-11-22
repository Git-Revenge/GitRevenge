// Package trailerrules reads and writes what a repository says about its trailer keys, kept
// in git config so it travels with the user's other settings and can be written by hand:
//
//	[gitrevenge-trailer "Reviewed-by"]
//	    also = Rviewed-by
//	    kind = people
//	    show = panel
//
// A rule in a repository's own config replaces one for the same key in the global config.
// Keys are matched as the trailer index joins them: whatever the letter case, and with
// "Closed" and "Close" as "Closes". Nothing here changes a commit: a rule only says how to
// read what's already written.
package trailerrules

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// Section is the config section the rules live in, one subsection for each key.
const Section = "gitrevenge-trailer"

// The two config files a rule can be in.
const (
	Local  = "local"
	Global = "global"
)

// What "show" can say: where a key is offered. Anything else says nothing.
const (
	// ShowRows shows the key's values on the graph's rows, and offers it in the panel.
	ShowRows = "rows"
	// ShowPanel offers the key in the panel whatever the app would have guessed.
	ShowPanel = "panel"
	// ShowHeld holds the key back from the panel.
	ShowHeld = "held"
	// ShowNone is how a repository turns off what the global config shows on rows.
	ShowNone = "none"
)

// kinds is what "kind" can say, and what the trailer index calls each.
var kinds = map[string]trailerindex.Kind{
	"people":      trailerindex.KindPerson,
	"tickets":     trailerindex.KindTicket,
	"commits":     trailerindex.KindCommit,
	"labels":      trailerindex.KindLabel,
	"identifiers": trailerindex.KindIdentifier,
	"text":        trailerindex.KindText,
}

// KindName returns what a rule calls a kind of value, or "" for one it has no word for.
func KindName(k trailerindex.Kind) string {
	for name, kind := range kinds {
		if kind == k {
			return name
		}
	}
	return ""
}

// Rule is what one config file says about one key.
type Rule struct {
	// Key is the key as the section names it, and ID what the trailer index calls it.
	Key string `json:"key"`
	ID  string `json:"id"`
	// Also are other keys read as this one, as the rule writes them.
	Also []string `json:"also"`
	// Kind is what the key's values are: "people", "tickets", "commits", "labels",
	// "identifiers" or "text"; "" when the rule doesn't say.
	Kind string `json:"kind"`
	// Show is where the key is offered, or "" when the rule doesn't say.
	Show string `json:"show"`
	// Apart are keys this one was asked about joining with and is kept apart from, so the
	// question isn't asked twice.
	Apart []string `json:"apart"`
	// Scope is the config the rule is in: "local" or "global".
	Scope string `json:"scope"`
}

// Load reads the rules in force: the repository's own, then the global ones for keys the
// repository says nothing about. g may be a Repo opened outside any repository, which
// reads only the global rules.
func Load(ctx context.Context, g *git.Repo, inRepo bool) ([]Rule, error) {
	global, err := read(ctx, g, Global)
	if err != nil {
		return nil, err
	}
	rules := []Rule{}
	said := map[string]bool{}
	if inRepo {
		local, err := read(ctx, g, Local)
		if err != nil {
			return nil, err
		}
		for _, r := range local {
			if !said[r.ID] {
				said[r.ID] = true
				rules = append(rules, r)
			}
		}
	}
	for _, r := range global {
		if !said[r.ID] {
			said[r.ID] = true
			rules = append(rules, r)
		}
	}
	return rules, nil
}

// Shown returns the IDs of the keys shown on the graph's rows, in the order the rules name them.
func Shown(rules []Rule) []string {
	out := []string{}
	for _, r := range rules {
		if r.Show == ShowRows {
			out = append(out, r.ID)
		}
	}
	return out
}

// Apply writes what the rules say into what a reading of the trailers takes: which keys
// are read as others, how a named key is written, what its values are, and where it's
// offered. A key joined to another gives up what a rule of its own says.
func Apply(rules []Rule, to *trailerindex.Rules) {
	to.Join, to.Labels, to.Kinds, to.Show = map[string]string{}, map[string]string{}, map[string]trailerindex.Kind{}, map[string]string{}
	for _, r := range rules {
		for _, other := range r.Also {
			if id := trailerindex.KeyID(other); id != r.ID {
				// The first rule to claim a key has it.
				if _, taken := to.Join[id]; !taken {
					to.Join[id] = r.ID
				}
			}
		}
	}
	for _, r := range rules {
		if _, joined := to.Join[r.ID]; joined {
			continue
		}
		to.Labels[r.ID] = r.Key
		if kind, ok := kinds[r.Kind]; ok {
			to.Kinds[r.ID] = kind
		}
		switch r.Show {
		case ShowRows, ShowPanel:
			to.Show[r.ID] = trailerindex.ShowPanel
		case ShowHeld:
			to.Show[r.ID] = trailerindex.ShowHeld
		}
	}
}

// KeptApart reports whether the rules say two keys, by ID, are kept apart: a question
// about joining them that was answered no.
func KeptApart(rules []Rule, a, b string) bool {
	for _, r := range rules {
		other := ""
		switch r.ID {
		case a:
			other = b
		case b:
			other = a
		default:
			continue
		}
		if slices.ContainsFunc(r.Apart, func(k string) bool { return trailerindex.KeyID(k) == other }) {
			return true
		}
	}
	return false
}

func scopeFlag(scope string) (string, error) {
	switch scope {
	case Local:
		return "--local", nil
	case Global:
		return "--global", nil
	}
	return "", fmt.Errorf("unknown config %q", scope)
}

// read returns one config file's rules in file order.
func read(ctx context.Context, g *git.Repo, scope string) ([]Rule, error) {
	flag, err := scopeFlag(scope)
	if err != nil {
		return nil, err
	}
	out, err := g.Read(ctx, "config", flag, "-z", "--get-regexp", `^`+regexp.QuoteMeta(Section)+`\.`)
	var gerr *git.Error
	// Exit 1: no such keys. A missing global file reads the same way.
	if errors.As(err, &gerr) && gerr.ExitCode == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rules []*Rule
	byName := map[string]*Rule{}
	for entry := range strings.SplitSeq(string(out), "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
		if key == "" || first < 0 || first == last {
			continue
		}
		name, field := key[first+1:last], strings.ToLower(key[last+1:])
		r := byName[name]
		if r == nil {
			r = &Rule{Key: name, ID: trailerindex.KeyID(name), Also: []string{}, Apart: []string{}, Scope: scope}
			byName[name] = r
			rules = append(rules, r)
		}
		value = strings.TrimSpace(value)
		switch field {
		case "show":
			r.Show = strings.ToLower(value)
		case "kind":
			r.Kind = strings.ToLower(value)
		case "also":
			if value != "" && !slices.Contains(r.Also, value) {
				r.Also = append(r.Also, value)
			}
		case "apart":
			if value != "" && !slices.Contains(r.Apart, value) {
				r.Apart = append(r.Apart, value)
			}
		}
	}
	res := make([]Rule, 0, len(rules))
	for _, r := range rules {
		res = append(res, *r)
	}
	return res, nil
}

// keyName is how a trailer key may be written as a section's name: what a trailer key can
// be, so that nothing typed elsewhere ends up as a config key.
var keyName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*$`)

// section finds the section to write a key's rule to in one config file: the one that
// already names the key, in any spelling, or one named as the key is given.
func section(ctx context.Context, g *git.Repo, scope, key string) (flag, name string, err error) {
	if flag, err = scopeFlag(scope); err != nil {
		return "", "", err
	}
	if !keyName.MatchString(key) {
		return "", "", fmt.Errorf("%q can't be a trailer key", key)
	}
	existing, err := read(ctx, g, scope)
	if err != nil {
		return "", "", err
	}
	id := trailerindex.KeyID(key)
	for _, r := range existing {
		if r.ID == id {
			key = r.Key
			break
		}
	}
	return flag, Section + "." + key, nil
}

// set says one thing in a key's rule, or with "" takes it away.
func set(ctx context.Context, g *git.Repo, scope, key, field, value string) error {
	flag, name, err := section(ctx, g, scope, key)
	if err != nil {
		return err
	}
	if value != "" {
		_, err = g.Write(ctx, git.Cmd{Args: []string{"config", flag, "--replace-all", name + "." + field, value}})
		return err
	}
	return unset(ctx, g, flag, name, field)
}

// unset takes a field out of a key's rule; with a value, only the line that says it.
func unset(ctx context.Context, g *git.Repo, flag, name, field string, value ...string) error {
	args := []string{"config", flag}
	if len(value) > 0 {
		args = append(args, "--fixed-value", "--unset-all", name+"."+field, value[0])
	} else {
		args = append(args, "--unset-all", name+"."+field)
	}
	_, err := g.Write(ctx, git.Cmd{Args: args})
	var gerr *git.Error
	// Exit 5: it wasn't set, which is what was asked for.
	if errors.As(err, &gerr) && gerr.ExitCode == 5 {
		return nil
	}
	if err != nil {
		return err
	}
	// git leaves the empty section behind: take it away if nothing else is said in it.
	rest, err := g.Read(ctx, "config", flag, "--get-regexp", `^`+regexp.QuoteMeta(name)+`\.`)
	if errors.As(err, &gerr) && gerr.ExitCode == 1 || err == nil && strings.TrimSpace(string(rest)) == "" {
		_, _ = g.Write(ctx, git.Cmd{Args: []string{"config", flag, "--remove-section", name}})
	}
	return nil
}

// add says one more of a thing a rule can say several of.
func add(ctx context.Context, g *git.Repo, scope, key, field, value string) error {
	if !keyName.MatchString(value) {
		return fmt.Errorf("%q can't be a trailer key", value)
	}
	flag, name, err := section(ctx, g, scope, key)
	if err != nil {
		return err
	}
	// Said already, in any spelling: nothing to add.
	rules, err := read(ctx, g, scope)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if Section+"."+r.Key != name {
			continue
		}
		had := r.Also
		if field == "apart" {
			had = r.Apart
		}
		if slices.ContainsFunc(had, func(k string) bool { return trailerindex.KeyID(k) == trailerindex.KeyID(value) }) {
			return nil
		}
	}
	_, err = g.Write(ctx, git.Cmd{Args: []string{"config", flag, "--add", name + "." + field, value}})
	return err
}

// remove takes back one of a thing a rule says several of, however it was spelled.
func remove(ctx context.Context, g *git.Repo, scope, key, field, value string) error {
	flag, name, err := section(ctx, g, scope, key)
	if err != nil {
		return err
	}
	rules, err := read(ctx, g, scope)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if Section+"."+r.Key != name {
			continue
		}
		had := r.Also
		if field == "apart" {
			had = r.Apart
		}
		for _, k := range had {
			if trailerindex.KeyID(k) == trailerindex.KeyID(value) {
				if err := unset(ctx, g, flag, name, field, k); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// SetShow says in one config file where a key is offered: ShowRows, ShowPanel, ShowHeld or
// ShowNone, or "" to say nothing there, which leaves it to the other file. key is the key
// as it should be written; a section that already names the same key, in any spelling, is
// the one changed.
func SetShow(ctx context.Context, g *git.Repo, scope, key, show string) error {
	switch show {
	case "", ShowRows, ShowPanel, ShowHeld, ShowNone:
	default:
		return fmt.Errorf("a key can't be shown %q", show)
	}
	return set(ctx, g, scope, key, "show", show)
}

// SetKind says in one config file what a key's values are, by one of the words a rule
// uses for a kind, or with "" leaves it to be guessed.
func SetKind(ctx context.Context, g *git.Repo, scope, key, kind string) error {
	if _, ok := kinds[kind]; !ok && kind != "" {
		return fmt.Errorf("%q isn't a kind of value", kind)
	}
	return set(ctx, g, scope, key, "kind", kind)
}

// Join reads another key as this one from now on: "also = other" in the key's rule. Asked
// and answered, so the two are no longer kept apart either.
func Join(ctx context.Context, g *git.Repo, scope, key, other string) error {
	if trailerindex.KeyID(key) == trailerindex.KeyID(other) {
		return errors.New("those are one key already")
	}
	if err := remove(ctx, g, scope, key, "apart", other); err != nil {
		return err
	}
	return add(ctx, g, scope, key, "also", other)
}

// Unjoin undoes a join: the other key is read as itself again.
func Unjoin(ctx context.Context, g *git.Repo, scope, key, other string) error {
	return remove(ctx, g, scope, key, "also", other)
}

// KeepApart remembers that two keys were asked about and aren't to be joined: "apart =
// other" in the key's rule.
func KeepApart(ctx context.Context, g *git.Repo, scope, key, other string) error {
	if trailerindex.KeyID(key) == trailerindex.KeyID(other) {
		return errors.New("those are one key already")
	}
	return add(ctx, g, scope, key, "apart", other)
}

// ForgetApart takes back a KeepApart, so the question may be asked again.
func ForgetApart(ctx context.Context, g *git.Repo, scope, key, other string) error {
	return remove(ctx, g, scope, key, "apart", other)
}
