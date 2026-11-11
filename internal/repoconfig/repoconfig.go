// Package repoconfig keeps a repository's own Git Revenge settings in git config, so they
// follow a clone's .git/config and the user's dotfiles the way git's own settings do
// (issue #526). Smart links already live there (package links); this is everything else
// that describes a repository rather than the app:
//
//	[gitrevenge]
//		cleanupOnPull = true
//		syncNotes = true
//		hide = wip/*
//		hide = spike/*
//		hideKind = bot
//		showKind = docs
//	[gitrevenge-kind "release"]
//		pattern = release/*
//		pattern = hotfix/*
//	[gitrevenge-insights]
//		stale = 14
//		longLived = 30
//		behind = 50
//		size = 800
//	[branch "feature/b"]
//		gitrevengeStackParent = feature/a
//
// Each setting is read from the repository's config when it is set there, and from the
// global config otherwise. A list set in the repository replaces the global list; it never
// adds to it. One empty value means "none here", which is how a repository turns off a
// list its user set globally. Writes go to the repository's config only, and only for
// the settings that changed.
package repoconfig

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// The keys, as git reports them: section and variable names in lower case.
const (
	section         = "gitrevenge"
	kindSection     = "gitrevenge-kind"
	insightsSection = "gitrevenge-insights"

	KeyCleanupOnPull = section + ".cleanupOnPull"
	KeySyncNotes     = section + ".syncNotes"
	KeyHide          = section + ".hide"
	KeyHideKind      = section + ".hideKind"
	KeyShowKind      = section + ".showKind"

	// stackVar is the per-branch variable, beside git's own branch.<name>.merge. Git removes
	// a branch's section when the branch is deleted, and this goes with it.
	stackVar = "gitrevengeStackParent"
)

// insightsVars are the Insights thresholds in the order Settings.Insights holds them.
var insightsVars = [4]string{"stale", "longLived", "behind", "size"}

// KindKey is the key holding a branch kind's patterns.
func KindKey(kind string) string { return kindSection + "." + kind + ".pattern" }

// StackKey is the key holding the branch a branch is stacked on.
func StackKey(branch string) string { return "branch." + branch + "." + stackVar }

// InsightsKey is the key holding Insights threshold i (see insightsVars).
func InsightsKey(i int) string { return insightsSection + "." + insightsVars[i] }

// Settings are one repository's settings, as they apply: the repository's own where it
// sets them, the global config's otherwise.
type Settings struct {
	// Kinds overrides a branch kind's patterns, by kind. A kind not listed uses its
	// defaults; one listed with no patterns matches nothing.
	Kinds map[string][]string
	// Hide lists glob patterns of branches to leave out of the sidebar and the graph.
	Hide []string
	// HideKinds hides every branch of these kinds.
	HideKinds []string
	// ShowKinds shows these kinds' lanes in full in the graph. Only Bot and Docs lanes fold
	// to a line a day, and they do unless they're listed here.
	ShowKinds     []string
	CleanupOnPull bool
	SyncNotes     bool
	// StackParents says which local branch each branch is stacked on, by branch name; ""
	// means not stacked. Branches not listed are inferred.
	StackParents map[string]string
	// Insights are the Insights thresholds: days stale, days long-lived, commits behind,
	// lines changed. Nil when none are set.
	Insights *[4]int
}

// Equal reports whether two settings say the same thing.
func (s Settings) Equal(o Settings) bool {
	if s.CleanupOnPull != o.CleanupOnPull || s.SyncNotes != o.SyncNotes {
		return false
	}
	if !slices.Equal(s.Hide, o.Hide) || !slices.Equal(s.HideKinds, o.HideKinds) || !slices.Equal(s.ShowKinds, o.ShowKinds) {
		return false
	}
	if !maps.Equal(s.StackParents, o.StackParents) || !maps.EqualFunc(s.Kinds, o.Kinds, slices.Equal[[]string]) {
		return false
	}
	if (s.Insights == nil) != (o.Insights == nil) {
		return false
	}
	return s.Insights == nil || *s.Insights == *o.Insights
}

// Empty reports whether nothing is set.
func (s Settings) Empty() bool { return s.Equal(Settings{}) }

// layer is what one config file sets. A nil list or missing map entry means the file
// doesn't say; a non-nil empty list means it says "none".
type layer struct {
	cleanup, notes *bool
	hide, hideKind []string
	showKind       []string
	kinds          map[string][]string
	stack          map[string]string
	insights       [4]*int
}

// Load returns the settings that apply to the repository g is open on.
func Load(ctx context.Context, g *git.Repo) (Settings, error) {
	global, err := read(ctx, g, "--global")
	if err != nil {
		return Settings{}, err
	}
	local, err := read(ctx, g, "--local")
	if err != nil {
		return Settings{}, err
	}
	return merge(global, local), nil
}

// LoadGlobal returns what the global config alone sets, which is what a repository
// inherits where it sets nothing.
func LoadGlobal(ctx context.Context, g *git.Repo) (Settings, error) {
	global, err := read(ctx, g, "--global")
	if err != nil {
		return Settings{}, err
	}
	return merge(global, layer{}), nil
}

// HasLocal reports whether the repository's own config sets any of these settings, which
// is how a repository that has been moved here is told from one that hasn't.
func HasLocal(ctx context.Context, g *git.Repo) (bool, error) {
	out, err := entries(ctx, g, "--local")
	return len(out) > 0, err
}

func merge(global, local layer) Settings {
	var s Settings
	pick := func(l, g *bool) bool {
		if l != nil {
			return *l
		}
		return g != nil && *g
	}
	s.CleanupOnPull = pick(local.cleanup, global.cleanup)
	s.SyncNotes = pick(local.notes, global.notes)
	list := func(l, g []string) []string {
		if l == nil {
			l = g
		}
		if len(l) == 0 {
			return nil
		}
		return slices.Clone(l)
	}
	s.Hide = list(local.hide, global.hide)
	s.HideKinds = list(local.hideKind, global.hideKind)
	s.ShowKinds = list(local.showKind, global.showKind)
	for _, src := range []map[string][]string{global.kinds, local.kinds} {
		for k, v := range src {
			if s.Kinds == nil {
				s.Kinds = map[string][]string{}
			}
			s.Kinds[k] = slices.Clone(v)
			if s.Kinds[k] == nil {
				s.Kinds[k] = []string{}
			}
		}
	}
	for _, src := range []map[string]string{global.stack, local.stack} {
		for k, v := range src {
			if s.StackParents == nil {
				s.StackParents = map[string]string{}
			}
			s.StackParents[k] = v
		}
	}
	var t [4]int
	set := false
	for i := range t {
		v := local.insights[i]
		if v == nil {
			v = global.insights[i]
		}
		if v != nil {
			t[i], set = *v, true
		}
	}
	if set {
		s.Insights = &t
	}
	return s
}

// pattern matches every key this package owns. Git matches it against the key with the
// section and variable names in lower case.
const pattern = `^(gitrevenge(-kind|-insights)?\.|branch\..*\.gitrevengestackparent$)`

// entries returns one config file's entries for these settings, in file order, as
// key/value pairs.
func entries(ctx context.Context, g *git.Repo, scope string) ([][2]string, error) {
	out, err := g.Read(ctx, "config", scope, "-z", "--get-regexp", pattern)
	var gerr *git.Error
	// Exit 1: no such keys. A missing global file reads the same way.
	if errors.As(err, &gerr) && gerr.ExitCode == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list [][2]string
	for entry := range strings.SplitSeq(string(out), "\x00") {
		if entry == "" {
			continue
		}
		key, value, _ := strings.Cut(entry, "\n")
		// Smart links share the gitrevenge section for one key of their own.
		if strings.HasPrefix(key, "gitrevenge-link.") || key == "gitrevenge.weblinks" {
			continue
		}
		list = append(list, [2]string{key, value})
	}
	return list, nil
}

func read(ctx context.Context, g *git.Repo, scope string) (layer, error) {
	list, err := entries(ctx, g, scope)
	if err != nil {
		return layer{}, err
	}
	var l layer
	// One empty value says "none"; values after it start the list again.
	add := func(to *[]string, v string) {
		if v = strings.TrimSpace(v); v == "" {
			*to = []string{}
			return
		}
		*to = append(*to, v)
	}
	for _, e := range list {
		key, value := e[0], e[1]
		first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
		sec, variable := key[:first], key[last+1:]
		sub := ""
		if first != last {
			sub = key[first+1 : last]
		}
		switch {
		case sec == section && variable == "cleanuponpull":
			b := truthy(value)
			l.cleanup = &b
		case sec == section && variable == "syncnotes":
			b := truthy(value)
			l.notes = &b
		case sec == section && variable == "hide":
			add(&l.hide, value)
		case sec == section && variable == "hidekind":
			add(&l.hideKind, value)
		case sec == section && variable == "showkind":
			add(&l.showKind, value)
		case sec == kindSection && variable == "pattern" && sub != "":
			if l.kinds == nil {
				l.kinds = map[string][]string{}
			}
			p := l.kinds[sub]
			add(&p, value)
			l.kinds[sub] = p
		case sec == "branch" && sub != "":
			if l.stack == nil {
				l.stack = map[string]string{}
			}
			l.stack[sub] = strings.TrimSpace(value)
		case sec == insightsSection:
			for i, name := range insightsVars {
				if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && variable == strings.ToLower(name) && n > 0 {
					l.insights[i] = &n
				}
			}
		}
	}
	return l, nil
}

// truthy reads a boolean the way git does for the values it writes and the common ones
// people type. A key with no value at all is true.
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "true", "yes", "on", "1":
		return true
	}
	return false
}

func config(ctx context.Context, g *git.Repo, args ...string) error {
	_, err := g.Write(ctx, git.Cmd{Args: append([]string{"config", "--local"}, args...)})
	return err
}

// unset removes every value of a key from the repository's config. Exit 5 means it wasn't
// set, which is fine.
func unset(ctx context.Context, g *git.Repo, key string) error {
	err := config(ctx, g, "--unset-all", key)
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.ExitCode == 5 {
		return nil
	}
	return err
}

// setList writes a list to the repository's config. An empty list is written as one empty
// value only when the global config has a list to turn off; otherwise the key is left out.
func setList(ctx context.Context, g *git.Repo, key string, values, inherited []string) error {
	if err := unset(ctx, g, key); err != nil {
		return err
	}
	if len(values) == 0 {
		if len(inherited) == 0 {
			return nil
		}
		return config(ctx, g, "--add", key, "")
	}
	for _, v := range values {
		if err := config(ctx, g, "--add", key, v); err != nil {
			return err
		}
	}
	return nil
}

// Save writes to the repository's own config whatever differs between was, the settings
// as they applied, and now. Settings that didn't change aren't written, so a value that
// comes from the global config stays inherited.
func Save(ctx context.Context, g *git.Repo, was, now Settings) error {
	global, err := LoadGlobal(ctx, g)
	if err != nil {
		return err
	}
	if was.CleanupOnPull != now.CleanupOnPull {
		if err := config(ctx, g, "--replace-all", KeyCleanupOnPull, fmt.Sprint(now.CleanupOnPull)); err != nil {
			return err
		}
	}
	if was.SyncNotes != now.SyncNotes {
		if err := config(ctx, g, "--replace-all", KeySyncNotes, fmt.Sprint(now.SyncNotes)); err != nil {
			return err
		}
	}
	if !slices.Equal(was.Hide, now.Hide) {
		if err := setList(ctx, g, KeyHide, now.Hide, global.Hide); err != nil {
			return err
		}
	}
	if !slices.Equal(was.HideKinds, now.HideKinds) {
		if err := setList(ctx, g, KeyHideKind, now.HideKinds, global.HideKinds); err != nil {
			return err
		}
	}
	if !slices.Equal(was.ShowKinds, now.ShowKinds) {
		if err := setList(ctx, g, KeyShowKind, now.ShowKinds, global.ShowKinds); err != nil {
			return err
		}
	}
	for _, kind := range sortedKeys(was.Kinds, now.Kinds) {
		before, had := was.Kinds[kind]
		after, has := now.Kinds[kind]
		switch {
		case had == has && slices.Equal(before, after):
		case !has:
			// Back to what's inherited: the global config's patterns, or the defaults.
			if err := unset(ctx, g, KindKey(kind)); err != nil {
				return err
			}
		case len(after) == 0:
			// Matches nothing: one empty value, whatever is inherited.
			if err := setList(ctx, g, KindKey(kind), nil, []string{""}); err != nil {
				return err
			}
		default:
			if err := setList(ctx, g, KindKey(kind), after, nil); err != nil {
				return err
			}
		}
	}
	for _, branch := range sortedKeys(was.StackParents, now.StackParents) {
		before, had := was.StackParents[branch]
		after, has := now.StackParents[branch]
		switch {
		case had == has && before == after:
		case !has:
			if err := unset(ctx, g, StackKey(branch)); err != nil {
				return err
			}
		default:
			if err := config(ctx, g, "--replace-all", StackKey(branch), after); err != nil {
				return err
			}
		}
	}
	switch {
	case was.Insights == nil && now.Insights == nil:
	case now.Insights == nil:
		for i := range insightsVars {
			if err := unset(ctx, g, InsightsKey(i)); err != nil {
				return err
			}
		}
	case was.Insights == nil || *was.Insights != *now.Insights:
		for i, v := range now.Insights {
			if err := config(ctx, g, "--replace-all", InsightsKey(i), strconv.Itoa(v)); err != nil {
				return err
			}
		}
	}
	return nil
}

// sortedKeys returns the keys of both maps, once each, in order, so writes happen in the
// same order every time.
func sortedKeys[V any](a, b map[string]V) []string {
	var keys []string
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}
