package app

import (
	"context"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// KindPatterns are one branch kind's patterns in a repository.
type KindPatterns struct {
	Kind string `json:"kind"`
	// Patterns are the patterns in use: the repository's own, or the defaults.
	Patterns []string `json:"patterns"`
	// Custom is true when the repository overrides the defaults.
	Custom bool `json:"custom"`
	// Defaults are the built-in patterns, for showing and restoring.
	Defaults []string `json:"defaults"`
}

// RepoPreferences are the settings that belong to one repository.
type RepoPreferences struct {
	// Kinds lists every branch kind, in matching order.
	Kinds []KindPatterns `json:"kinds"`
	// Hide lists patterns of branches to leave out of the sidebar and the graph.
	Hide []string `json:"hide"`
	// HideKinds lists kinds whose branches are hidden (the graph's Docs and Bots toggles).
	HideKinds     []string `json:"hideKinds"`
	CleanupOnPull bool     `json:"cleanupOnPull"`
	SyncNotes     bool     `json:"syncNotes"`
}

// RepoPreferences returns the settings for the repository at path.
func (a *App) RepoPreferences(path string) (RepoPreferences, error) {
	path = a.repoKey(path)
	m, err := a.sessions()
	if err != nil {
		return RepoPreferences{}, err
	}
	rs := m.Settings().Get().Repo(path)
	out := RepoPreferences{Hide: nonNil(rs.Hide), HideKinds: nonNil(rs.HideKinds), CleanupOnPull: rs.CleanupOnPull, SyncNotes: rs.SyncNotes}
	for _, k := range branchkind.Order {
		kp := KindPatterns{Kind: string(k), Defaults: nonNil(branchkind.Defaults[k])}
		if p, ok := rs.Kinds[string(k)]; ok {
			kp.Patterns, kp.Custom = nonNil(p), true
		} else {
			kp.Patterns = kp.Defaults
		}
		out.Kinds = append(out.Kinds, kp)
	}
	return out, nil
}

// SetRepoPreferences checks and saves the settings for the repository at path. A kind
// whose patterns equal its defaults is stored as using the defaults, so later changes to
// the defaults reach it.
func (a *App) SetRepoPreferences(path string, p RepoPreferences) error {
	if strings.TrimSpace(path) == "" {
		return userError("No repository is open.")
	}
	path = a.repoKey(path)
	rs := settings.RepoSettings{Path: path, CleanupOnPull: p.CleanupOnPull, SyncNotes: p.SyncNotes}
	for _, kp := range p.Kinds {
		k := branchkind.Kind(kp.Kind)
		if !branchkind.Valid(k) {
			return userError("Unknown branch kind " + kp.Kind + ".")
		}
		pats, err := cleanPatterns(kp.Patterns)
		if err != nil {
			return err
		}
		if !kp.Custom || slices.Equal(pats, branchkind.Defaults[k]) {
			continue
		}
		if rs.Kinds == nil {
			rs.Kinds = map[string][]string{}
		}
		rs.Kinds[string(k)] = pats
	}
	hide, err := cleanPatterns(p.Hide)
	if err != nil {
		return err
	}
	if len(hide) > 0 {
		rs.Hide = hide
	}
	for _, k := range p.HideKinds {
		if !branchkind.Valid(branchkind.Kind(k)) {
			return userError("Unknown branch kind " + k + ".")
		}
		if !slices.Contains(rs.HideKinds, k) {
			rs.HideKinds = append(rs.HideKinds, k)
		}
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	if err := m.Settings().Update(func(s *settings.Settings) { s.SetRepo(rs) }); err != nil {
		return err
	}
	if r := m.Get(path); r != nil {
		r.ReloadGraph()
	}
	return nil
}

// HideBranch hides a branch (named without its remote) in the repository at path, or
// unhides it, by adding or removing that exact name in the hide patterns. Unhiding a branch
// that a wider pattern hides isn't possible here; the error names the pattern to change.
func (a *App) HideBranch(path, name string, hide bool) error {
	path = a.repoKey(path)
	name = strings.TrimSpace(name)
	if err := branchkind.CheckPattern(name); err != nil {
		return userError("Can't hide that branch: " + err.Error() + ".")
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	var wider string
	err = m.Settings().Update(func(s *settings.Settings) {
		rs := s.Repo(path)
		rs.Hide = slices.DeleteFunc(rs.Hide, func(p string) bool { return p == name })
		if hide {
			rs.Hide = append(rs.Hide, name)
		} else {
			for _, p := range rs.Hide {
				if branchkind.Match(p, name) {
					wider = p
				}
			}
		}
		s.SetRepo(rs)
	})
	if err != nil {
		return err
	}
	if r := m.Get(path); r != nil {
		r.ReloadGraph()
	}
	if wider != "" {
		return userError(name + " is still hidden by the pattern " + wider + ". Change it in Settings, under This repository.")
	}
	return nil
}

// MarkBranchKind makes a branch (named without its remote) a kind in the repository at
// path, or ordinary when kind is "", by adding its exact name to that kind's patterns and
// taking it out of the others. A kind that was using its defaults keeps them, plus the name.
// If a wider pattern of another kind still claims the branch, the error says which.
func (a *App) MarkBranchKind(path, name, kind string) error {
	path = a.repoKey(path)
	name = strings.TrimSpace(name)
	if err := branchkind.CheckPattern(name); err != nil {
		return userError("Can't mark that branch: " + err.Error() + ".")
	}
	k := branchkind.Kind(kind)
	if k != branchkind.None && !branchkind.Valid(k) {
		return userError("Unknown branch kind " + kind + ".")
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	var result branchkind.Kind
	err = m.Settings().Update(func(s *settings.Settings) {
		rs := s.Repo(path)
		if rs.Kinds == nil {
			rs.Kinds = map[string][]string{}
		}
		for _, o := range branchkind.Order {
			pats, custom := rs.Kinds[string(o)]
			if !custom {
				pats = slices.Clone(branchkind.Defaults[o])
			}
			had := slices.Contains(pats, name)
			pats = slices.DeleteFunc(pats, func(p string) bool { return p == name })
			if o == k {
				pats = append(pats, name)
			}
			if o == k || had {
				if slices.Equal(pats, branchkind.Defaults[o]) {
					delete(rs.Kinds, string(o))
				} else {
					rs.Kinds[string(o)] = pats
				}
			}
		}
		if len(rs.Kinds) == 0 {
			rs.Kinds = nil
		}
		s.SetRepo(rs)
		result = session.Rules(rs).Kind(name)
	})
	if err != nil {
		return err
	}
	if r := m.Get(path); r != nil {
		r.ReloadGraph()
	}
	if result != k {
		return userError(name + " still matches a " + string(result) + " pattern. Change it in Settings, under This repository.")
	}
	return nil
}

// SetKindVisible shows or hides every branch of a kind in the repository at path, as the
// graph's Kinds filter does.
func (a *App) SetKindVisible(path, kind string, visible bool) error {
	path = a.repoKey(path)
	if !branchkind.Valid(branchkind.Kind(kind)) {
		return userError("Unknown branch kind " + kind + ".")
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	err = m.Settings().Update(func(s *settings.Settings) {
		rs := s.Repo(path)
		rs.HideKinds = slices.DeleteFunc(rs.HideKinds, func(k string) bool { return k == kind })
		if !visible {
			rs.HideKinds = append(rs.HideKinds, kind)
		}
		s.SetRepo(rs)
	})
	if err != nil {
		return err
	}
	if r := m.Get(path); r != nil {
		r.ReloadGraph()
	}
	return nil
}

// SetStackParent sets the branch a local branch is stacked on, in the repository at path:
// parent names a local branch, "" means not stacked, and auto drops the override so the
// stack is inferred again.
func (a *App) SetStackParent(path, branch, parent string, auto bool) error {
	branch, parent = strings.TrimSpace(branch), strings.TrimSpace(parent)
	if branch == "" || branch == parent {
		return userError("A branch can't be stacked on itself.")
	}
	return a.updateStackParents(path, func(parents map[string]string) {
		if auto {
			delete(parents, branch)
		} else {
			parents[branch] = parent
		}
	})
}

// updateStackParents changes the stack parents set by hand in the repository at path, then
// reloads its graph to show them.
func (a *App) updateStackParents(path string, change func(map[string]string)) error {
	path = a.repoKey(path)
	m, err := a.sessions()
	if err != nil {
		return err
	}
	err = m.Settings().Update(func(s *settings.Settings) {
		rs := s.Repo(path)
		if rs.StackParents == nil {
			rs.StackParents = map[string]string{}
		}
		change(rs.StackParents)
		if len(rs.StackParents) == 0 {
			rs.StackParents = nil
		}
		s.SetRepo(rs)
	})
	if err != nil {
		return err
	}
	if r := m.Get(path); r != nil {
		r.ReloadGraph()
	}
	return nil
}

// SetShowHidden shows the hidden branches of the repository at path again, or hides them,
// for this session only.
func (a *App) SetShowHidden(path string, show bool) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	r.SetShowHidden(show)
	return nil
}

// ShowingHidden reports whether the repository at path is showing its hidden branches.
func (a *App) ShowingHidden(path string) (bool, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return false, err
	}
	return r.ShowingHidden(), nil
}

// repoKey is the path a repository's settings are kept under: its work tree root, so a
// path inside the repository, a symlinked temp folder or a Windows short name all find the
// same settings. A path that isn't a repository is used as it is.
func (a *App) repoKey(path string) string {
	m, err := a.sessions()
	if err != nil {
		return path
	}
	if r := m.Get(path); r != nil {
		return r.Path
	}
	if root, err := m.Resolve(context.Background(), path); err == nil {
		return root
	}
	return path
}

// repoSettings returns the stored settings of the repository at path.
func (a *App) repoSettings(path string) settings.RepoSettings {
	path = a.repoKey(path)
	m, err := a.sessions()
	if err != nil {
		return settings.RepoSettings{Path: path}
	}
	return m.Settings().Get().Repo(path)
}

// branchRules returns the kind and hide rules for the repository at path.
func (a *App) branchRules(path string) branchkind.Rules {
	path = a.repoKey(path)
	m, err := a.sessions()
	if err != nil {
		return branchkind.New(nil, nil)
	}
	return session.Rules(m.Settings().Get().Repo(path))
}

// cleanPatterns trims patterns, drops blanks and duplicates, and checks each one.
func cleanPatterns(in []string) ([]string, error) {
	out := []string{}
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" || slices.Contains(out, p) {
			continue
		}
		if err := branchkind.CheckPattern(p); err != nil {
			return nil, userError("Branch pattern: " + err.Error() + ".")
		}
		out = append(out, p)
	}
	return out, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return slices.Clone(s)
}
