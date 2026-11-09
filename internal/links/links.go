// Package links reads and writes smart link rules: patterns that turn text in commit
// messages, trailers and branch names into links to another site, such as ABC-123 to a
// Jira issue. They are links only; nothing here contacts the linked site.
//
// Rules live in git config, one section each, in the repository's own .git/config or in
// the global config:
//
//	[gitrevenge-link "Jira"]
//		pattern = "\\b(ABC|OPS)-\\d+\\b"
//		url = https://acme.atlassian.net/browse/{0}
//		in = message trailers branches
//		shipped = true
//
// A rule in the repository's config replaces a global rule with the same name. Rules are
// tried in order, the repository's first, each in file order; where two match at the same
// place, the earlier rule wins.
package links

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Section is the git config section that holds one rule per subsection.
const Section = "gitrevenge-link"

// WebKey turns links for web addresses written out in full on or off. It's on unless set
// to false.
const WebKey = "gitrevenge.webLinks"

// Where a rule looks.
const (
	InMessage  = "message"
	InTrailers = "trailers"
	InBranches = "branches"
)

// AllPlaces is where a rule looks when it doesn't say.
var AllPlaces = []string{InMessage, InTrailers, InBranches}

// Scopes a rule is kept in.
const (
	Local  = "local"
	Global = "global"
)

// Rule is one smart link.
type Rule struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	// URL is the address to open, with {0} for the whole match and {1}…{9} for groups.
	URL string   `json:"url"`
	In  []string `json:"in"`
	// Shipped means matches count as references in Insights › Shipped.
	Shipped bool   `json:"shipped"`
	Enabled bool   `json:"enabled"`
	Scope   string `json:"scope"`
	// Shadowed marks a global rule a repository rule of the same name replaces.
	Shadowed bool `json:"shadowed,omitempty"`
	// Problem says why the rule can't be used, such as a pattern that doesn't compile.
	Problem string `json:"problem,omitempty"`
}

// Config is every rule that applies to one repository, and whether web addresses link.
type Config struct {
	Rules []Rule `json:"rules"`
	Web   bool   `json:"web"`
}

// Active returns the rules in use: enabled, not replaced, and valid, in matching order.
func (c Config) Active() []Rule {
	var out []Rule
	for _, r := range c.Rules {
		if r.Enabled && !r.Shadowed && r.Problem == "" {
			out = append(out, r)
		}
	}
	return out
}

// Load reads the rules from a repository's config and the global config. g may be a Repo
// opened outside any repository, which reads only the global rules.
func Load(ctx context.Context, g *git.Repo, inRepo bool) (Config, error) {
	global, err := read(ctx, g, "--global")
	if err != nil {
		return Config{}, err
	}
	var local []Rule
	if inRepo {
		if local, err = read(ctx, g, "--local"); err != nil {
			return Config{}, err
		}
	}
	for i := range global {
		if slices.ContainsFunc(local, func(r Rule) bool { return r.Name == global[i].Name }) {
			global[i].Shadowed = true
		}
	}
	c := Config{Rules: append(local, global...), Web: true}
	if c.Rules == nil {
		c.Rules = []Rule{}
	}
	if v, err := g.ReadString(ctx, "config", "--type=bool", "--get", WebKey); err == nil && v == "false" {
		c.Web = false
	}
	return c, nil
}

// read returns one config file's rules in file order.
func read(ctx context.Context, g *git.Repo, scope string) ([]Rule, error) {
	out, err := g.Read(ctx, "config", scope, "-z", "--get-regexp", `^`+regexp.QuoteMeta(Section)+`\.`)
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
			r = &Rule{Name: name, Enabled: true, Scope: strings.TrimPrefix(scope, "--"), In: AllPlaces}
			byName[name] = r
			rules = append(rules, r)
		}
		switch field {
		case "pattern":
			r.Pattern = value
		case "url":
			r.URL = value
		case "in":
			r.In = places(value)
		case "shipped":
			r.Shipped = boolValue(value)
		case "enabled":
			r.Enabled = boolValue(value)
		}
	}
	res := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if err := Check(*r); err != nil {
			r.Problem = err.Error()
		}
		res = append(res, *r)
	}
	return res, nil
}

func places(v string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(strings.ToLower(v), func(c rune) bool { return c == ' ' || c == ',' }) {
		if slices.Contains(AllPlaces, p) && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func boolValue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "true", "yes", "on", "1":
		return true
	}
	return false
}

var placeholder = regexp.MustCompile(`\{([^}]*)\}`)

// Check says what's wrong with a rule, or nil. Patterns use Go's regular expressions (RE2),
// which have no lookaround or backreferences.
func Check(r Rule) error {
	switch {
	case strings.TrimSpace(r.Name) == "":
		return errors.New("a link needs a name")
	case strings.ContainsAny(r.Name, "\n\x00\"\\"):
		return errors.New("a name can't hold quotes, backslashes or line breaks")
	case r.Pattern == "":
		return errors.New("a link needs a pattern")
	}
	re, err := regexp.Compile(r.Pattern)
	if err != nil {
		return fmt.Errorf("the pattern isn't a regular expression this app can read: %w", err)
	}
	if re.MatchString("") {
		return errors.New("the pattern matches empty text, so it would match everywhere")
	}
	u := strings.TrimSpace(r.URL)
	if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return errors.New("the address must start with https:// or http://")
	}
	if strings.ContainsAny(u, " \t\n") {
		return errors.New("the address can't hold spaces")
	}
	for _, m := range placeholder.FindAllStringSubmatch(u, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 0 || n > re.NumSubexp() {
			return fmt.Errorf("{%s} in the address isn't the match or one of the pattern's groups", m[1])
		}
	}
	return nil
}

// Expand fills an address template with a match's groups. Each value is escaped as for a
// URL component, as the frontend's encodeURIComponent does.
func Expand(template string, groups []string) string {
	return placeholder.ReplaceAllStringFunc(template, func(m string) string {
		n, err := strconv.Atoi(m[1 : len(m)-1])
		if err != nil || n >= len(groups) {
			return m
		}
		return componentEscape(groups[n])
	})
}

// componentEscape escapes everything but A–Z a–z 0–9 - _ . ! ~ * ' ( ), as
// encodeURIComponent does.
func componentEscape(s string) string {
	const keep = "-_.!~*'()"
	var b strings.Builder
	for _, c := range []byte(s) {
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte(keep, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// Match is one place a rule matched.
type Match struct {
	Text string `json:"text"`
	Href string `json:"href"`
	Rule string `json:"rule"`
}

// Find returns the matches of the active rules that look in a place, earliest first; where
// two overlap, the one starting first wins, then the earlier rule.
func (c Config) Find(text, place string) []Match {
	type hit struct {
		start, end, rule int
		groups           []string
	}
	var hits []hit
	active := c.Active()
	for i, r := range active {
		if !slices.Contains(r.In, place) {
			continue
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		for _, loc := range re.FindAllStringSubmatchIndex(text, -1) {
			if loc[1] == loc[0] {
				continue
			}
			g := make([]string, len(loc)/2)
			for k := range g {
				if loc[2*k] >= 0 {
					g[k] = text[loc[2*k]:loc[2*k+1]]
				}
			}
			hits = append(hits, hit{loc[0], loc[1], i, g})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int {
		if a.start != b.start {
			return a.start - b.start
		}
		return a.rule - b.rule
	})
	var out []Match
	end := 0
	for _, h := range hits {
		if h.start < end {
			continue
		}
		end = h.end
		out = append(out, Match{Text: h.groups[0], Href: Expand(active[h.rule].URL, h.groups), Rule: active[h.rule].Name})
	}
	return out
}

// References returns the matches of rules that count for Insights › Shipped, found in a
// message's subject (for rules that look in messages) and its reference trailers (for
// rules that look in trailers). It returns nil when no rule counts for Shipped, so the
// built-in references apply.
func (c Config) References() func(text, place string) []string {
	var shipped Config
	for _, r := range c.Active() {
		if r.Shipped {
			shipped.Rules = append(shipped.Rules, r)
		}
	}
	if len(shipped.Rules) == 0 {
		return nil
	}
	return func(text, place string) []string {
		var out []string
		for _, m := range shipped.Find(text, place) {
			out = append(out, m.Text)
		}
		return out
	}
}

// Patterns lists the rules that count for Shipped, one line each, for Insights exports.
func (c Config) Patterns() []string {
	out := []string{}
	for _, r := range c.Active() {
		if r.Shipped {
			out = append(out, r.Name+" "+r.Pattern)
		}
	}
	return out
}
