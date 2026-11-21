// Package branchkind classifies branches into the built-in kinds (Trunk, Release,
// Environment, Docs, Bot, Feature) and decides which are hidden, by glob patterns kept per
// repository in local settings (ADR 0009).
package branchkind

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Kind is a branch's kind. The zero value is an ordinary branch.
type Kind string

// The built-in kinds, in the order they are tried: the first whose patterns match wins.
const (
	None        Kind = ""
	Trunk       Kind = "trunk"
	Release     Kind = "release"
	Environment Kind = "environment"
	Docs        Kind = "docs"
	Bot         Kind = "bot"
	Feature     Kind = "feature"
)

// Order lists the kinds in matching order.
var Order = []Kind{Trunk, Release, Environment, Docs, Bot, Feature}

// Defaults are the patterns each kind uses until a repository overrides them. Environment
// has none: staging and production are too often ordinary branches.
var Defaults = map[Kind][]string{
	Trunk:       {"main", "master", "develop", "trunk"},
	Release:     {"release/*", "releases/*", "hotfix/*"},
	Environment: {},
	Docs:        {"gh-pages", "gh-docs", "docs-site"},
	Bot:         {"dependabot/*", "renovate/*"},
	Feature:     {"feature/*", "features/*", "feat/*"},
}

// Valid reports whether k is one of the built-in kinds.
func Valid(k Kind) bool { return slices.Contains(Order, k) }

// Rules are one repository's kind patterns and hide patterns.
type Rules struct {
	kinds map[Kind][]string
	hide  []string
	// hideKinds hides every branch of these kinds (the graph's Docs and Bots toggles).
	hideKinds map[Kind]bool
	// botAuthors are the repository's own patterns for authors that are bots, on top of
	// the built-in one.
	botAuthors []string
}

// BuiltInBotAuthor is the author pattern that always marks a bot: a GitHub App's name, such
// as dependabot[bot].
const BuiltInBotAuthor = "*[bot]"

// WithBotAuthors returns r with these patterns marking authors as bots, besides the
// built-in one.
func (r Rules) WithBotAuthors(patterns []string) Rules {
	r.botAuthors = patterns
	return r
}

// BotAuthorPatterns returns the author patterns in use, the built-in one first, for printing
// as an input (ADR 0008) and keying cached results.
func (r Rules) BotAuthorPatterns() []string {
	out := []string{BuiltInBotAuthor}
	for _, p := range r.botAuthors {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// BotAuthor reports whether an author is a bot: the name or the email matches one of the
// author patterns. Case doesn't count, since email addresses don't have any.
func (r Rules) BotAuthor(name, email string) bool {
	name, email = strings.ToLower(name), strings.ToLower(email)
	for _, p := range r.BotAuthorPatterns() {
		p = strings.ToLower(p)
		if Match(p, name) || (email != "" && Match(p, email)) {
			return true
		}
	}
	return false
}

// CheckAuthorPattern returns an error describing why pattern can't match an author's name or
// email, or nil. Unlike a branch pattern it may hold spaces and brackets: "Release Bot",
// "*[bot]".
func CheckAuthorPattern(pattern string) error {
	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("a pattern is empty")
	}
	for _, c := range pattern {
		if unicode.IsControl(c) {
			return fmt.Errorf("%q has a character names and emails can't contain", pattern)
		}
	}
	return nil
}

// New returns rules from a repository's overrides: a kind missing from overrides uses its
// defaults, and a kind present with an empty list matches nothing.
func New(overrides map[Kind][]string, hide []string) Rules {
	r := Rules{kinds: map[Kind][]string{}, hide: hide}
	for _, k := range Order {
		if p, ok := overrides[k]; ok {
			r.kinds[k] = p
		} else {
			r.kinds[k] = Defaults[k]
		}
	}
	return r
}

// HidingKinds returns r with every branch of the given kinds hidden as well.
func (r Rules) HidingKinds(kinds []Kind) Rules {
	if len(kinds) == 0 {
		return r
	}
	r.hideKinds = map[Kind]bool{}
	for _, k := range kinds {
		if k != None {
			r.hideKinds[k] = true
		}
	}
	return r
}

// KindHidden reports whether branches of kind k are hidden.
func (r Rules) KindHidden(k Kind) bool { return r.hideKinds[k] }

// Patterns returns the patterns kind k uses under these rules.
func (r Rules) Patterns(k Kind) []string { return r.kinds[k] }

// KindPatterns returns every kind's patterns in use, keyed by kind, for printing as an
// input (ADR 0008) and keying cached results.
func (r Rules) KindPatterns() map[string][]string {
	out := make(map[string][]string, len(Order))
	for _, k := range Order {
		out[string(k)] = slices.Clone(r.kinds[k])
		if out[string(k)] == nil {
			out[string(k)] = []string{}
		}
	}
	return out
}

// Hide returns the hide patterns, including the patterns of hidden kinds, for leaving
// branches out of git log.
func (r Rules) Hide() []string {
	if len(r.hideKinds) == 0 {
		return r.hide
	}
	out := slices.Clone(r.hide)
	for _, k := range Order {
		if r.hideKinds[k] {
			out = append(out, r.kinds[k]...)
		}
	}
	return out
}

// Kind classifies a branch by its name without any remote prefix (see Strip).
func (r Rules) Kind(name string) Kind {
	for _, k := range Order {
		for _, p := range r.kinds[k] {
			if Match(p, name) {
				return k
			}
		}
	}
	return None
}

// Hidden reports whether a branch, named without any remote prefix, matches a hide
// pattern.
func (r Rules) Hidden(name string) bool {
	for _, p := range r.hide {
		if Match(p, name) {
			return true
		}
	}
	return len(r.hideKinds) > 0 && r.hideKinds[r.Kind(name)]
}

// Strip removes a remote's name from a remote-tracking branch, so origin/release/1.4
// classifies as release/1.4. Names that don't start with one of remotes are returned as
// they are. The longest matching remote wins, for remotes whose names contain a slash.
func Strip(name string, remotes []string) string {
	best := ""
	for _, rm := range remotes {
		if len(rm) > len(best) && strings.HasPrefix(name, rm+"/") {
			best = rm
		}
	}
	if best == "" {
		return name
	}
	return name[len(best)+1:]
}

// Match reports whether name matches pattern. `*` matches any run of characters, slashes
// included, so dependabot/* matches dependabot/npm_and_yarn/vite-7.1, and `?` matches any
// one character. Everything else matches itself. This is git's wildmatch without
// WM_PATHNAME, which `git log --exclude` uses, so hiding a branch here and in git agree.
func Match(pattern, name string) bool {
	p, n := 0, 0
	star, mark := -1, 0
	for n < len(name) {
		switch {
		case p < len(pattern) && pattern[p] == '*':
			star, mark = p, n
			p++
		case p < len(pattern) && (pattern[p] == name[n] || pattern[p] == '?'):
			p++
			n++
		case star >= 0:
			// Let the last star take one more character and try again.
			mark++
			p, n = star+1, mark
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}

// CheckPattern returns an error describing why pattern can't match a branch name, or nil.
func CheckPattern(pattern string) error {
	switch {
	case pattern == "":
		return fmt.Errorf("a pattern is empty")
	case strings.Contains(pattern, ".."), strings.Contains(pattern, "//"), strings.HasPrefix(pattern, "/"), strings.HasSuffix(pattern, "/"):
		return fmt.Errorf("%q can't match a branch name", pattern)
	}
	for _, c := range pattern {
		if unicode.IsSpace(c) || unicode.IsControl(c) || strings.ContainsRune("~^:\\[", c) {
			return fmt.Errorf("%q has %q, which branch names can't contain", pattern, c)
		}
	}
	return nil
}
