// Package trailerrules reads and writes what a repository says about its trailer keys, kept
// in git config so it travels with the user's other settings and can be written by hand:
//
//	[gitrevenge-trailer "Changelog"]
//	    show = rows
//
// A rule in a repository's own config comes before one for the same key in the global
// config. Keys are matched as the trailer index joins them: whatever the letter case, and
// with "Closed" and "Close" as "Closes".
package trailerrules

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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

// What "show" can say. Anything else, "none" among it, shows the key nowhere, which is how
// a repository turns off a key the global config shows.
const (
	ShowRows = "rows"
	ShowNone = "none"
)

// Rule is what one config file says about one key.
type Rule struct {
	// Key is the key as the section names it, and ID what the trailer index calls it.
	Key string `json:"key"`
	ID  string `json:"id"`
	// Show is where the key's values are shown beside a commit: "rows" for the graph's
	// rows, or "" when the rule doesn't say.
	Show string `json:"show"`
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
			r = &Rule{Key: name, ID: trailerindex.KeyID(name), Scope: scope}
			byName[name] = r
			rules = append(rules, r)
		}
		if field == "show" {
			r.Show = strings.ToLower(strings.TrimSpace(value))
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

// SetShow says in one config file where a key is shown: ShowRows, ShowNone, or "" to say
// nothing there, which leaves it to the other file. key is the key as it should be
// written; a section that already names the same key, in any spelling, is the one changed.
func SetShow(ctx context.Context, g *git.Repo, scope, key, show string) error {
	flag, err := scopeFlag(scope)
	if err != nil {
		return err
	}
	if !keyName.MatchString(key) {
		return fmt.Errorf("%q can't be a trailer key", key)
	}
	existing, err := read(ctx, g, scope)
	if err != nil {
		return err
	}
	id := trailerindex.KeyID(key)
	for _, r := range existing {
		if r.ID == id {
			key = r.Key
			break
		}
	}
	name := Section + "." + key + ".show"
	if show != "" {
		_, err = g.Write(ctx, git.Cmd{Args: []string{"config", flag, "--replace-all", name, show}})
		return err
	}
	_, err = g.Write(ctx, git.Cmd{Args: []string{"config", flag, "--unset-all", name}})
	var gerr *git.Error
	// Exit 5: it wasn't set, which is what was asked for.
	if errors.As(err, &gerr) && gerr.ExitCode == 5 {
		return nil
	}
	if err != nil {
		return err
	}
	// git leaves the empty section behind: take it away if nothing else is said in it.
	rest, err := g.Read(ctx, "config", flag, "--get-regexp", `^`+regexp.QuoteMeta(Section+"."+key)+`\.`)
	if errors.As(err, &gerr) && gerr.ExitCode == 1 || err == nil && strings.TrimSpace(string(rest)) == "" {
		_, _ = g.Write(ctx, git.Cmd{Args: []string{"config", flag, "--remove-section", Section + "." + key}})
	}
	return nil
}
