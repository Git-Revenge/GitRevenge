package app

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/repoconfig"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// A repository's own settings live in git config (package repoconfig, issue #526): its
// .git/config where it sets them, the global config otherwise. The app's settings file keeps
// a copy of what applies, which is what everything else reads: it's there before git has
// answered, it costs nothing to read while a graph loads, and a release from before the
// move still finds the settings it knows.
//
// The copy is brought up to date when a repository opens and whenever its config or the
// global config changes. Changes made in the app are written to the repository's config.

// insightsScope is the Insights scope of one repository, as the thresholds are keyed.
func insightsScope(root string) string { return "repo:" + root }

// toRepoConfig is the app's stored settings in repoconfig's terms.
func toRepoConfig(rs settings.RepoSettings, thresholds *[4]int) repoconfig.Settings {
	c := repoconfig.Settings{
		Hide:          slices.Clone(rs.Hide),
		HideKinds:     slices.Clone(rs.HideKinds),
		ShowKinds:     slices.Clone(rs.ShowKinds),
		CleanupOnPull: rs.CleanupOnPull,
		SyncNotes:     rs.SyncNotes,
		StackParents:  maps.Clone(rs.StackParents),
		Insights:      thresholds,
	}
	if len(c.Hide) == 0 {
		c.Hide = nil
	}
	if len(c.HideKinds) == 0 {
		c.HideKinds = nil
	}
	if len(c.ShowKinds) == 0 {
		c.ShowKinds = nil
	}
	if len(c.StackParents) == 0 {
		c.StackParents = nil
	}
	for k, v := range rs.Kinds {
		if c.Kinds == nil {
			c.Kinds = map[string][]string{}
		}
		c.Kinds[k] = append([]string{}, v...)
	}
	return c
}

// fromRepoConfig is what git config says, as the app stores it.
func fromRepoConfig(root string, c repoconfig.Settings) settings.RepoSettings {
	rs := settings.RepoSettings{
		Path:          root,
		Hide:          slices.Clone(c.Hide),
		HideKinds:     slices.Clone(c.HideKinds),
		ShowKinds:     slices.Clone(c.ShowKinds),
		CleanupOnPull: c.CleanupOnPull,
		SyncNotes:     c.SyncNotes,
		StackParents:  maps.Clone(c.StackParents),
		InGitConfig:   true,
	}
	for k, v := range c.Kinds {
		if rs.Kinds == nil {
			rs.Kinds = map[string][]string{}
		}
		rs.Kinds[k] = append([]string{}, v...)
	}
	return rs
}

// storedThresholds returns a repository's Insights thresholds from the app's copy, or nil.
func storedThresholds(s settings.Settings, root string) *[4]int {
	if v, ok := s.InsightsThresholds[insightsScope(root)]; ok {
		return &v
	}
	return nil
}

// syncRepoConfig brings the settings of the repository at path in from git config, and
// reports whether that changed what the app holds. The first time it meets a repository
// whose settings are still only in the app's file, it writes them to the repository's
// config: once, and only when that config sets none of them itself, since a config that
// already has them (from a dotfile, say) is the newer word. A folder that isn't a
// repository has nothing to bring in.
func (a *App) syncRepoConfig(ctx context.Context, path string) (bool, error) {
	m, err := a.sessions()
	if err != nil {
		return false, err
	}
	g, root, err := m.GitFor(ctx, path)
	if err != nil {
		return false, nil
	}
	a.repoConfigMu.Lock()
	defer a.repoConfigMu.Unlock()

	st := m.Settings().Get()
	held := st.Repo(root)
	heldThresholds := storedThresholds(st, root)
	now, err := repoconfig.Load(ctx, g)
	if err != nil {
		return false, err
	}
	if !held.InGitConfig {
		if want := toRepoConfig(held, heldThresholds); !want.Empty() {
			has, err := repoconfig.HasLocal(ctx, g)
			if err != nil {
				return false, err
			}
			if !has {
				if err := repoconfig.Save(ctx, g, now, want); err != nil {
					return false, err
				}
				if now, err = repoconfig.Load(ctx, g); err != nil {
					return false, err
				}
			}
		}
	}
	changed := !toRepoConfig(held, heldThresholds).Equal(now)
	if !changed && held.InGitConfig {
		return false, nil
	}
	err = m.Settings().Update(func(s *settings.Settings) {
		s.SetRepo(fromRepoConfig(root, now))
		scope := insightsScope(root)
		if now.Insights != nil {
			if s.InsightsThresholds == nil {
				s.InsightsThresholds = map[string][4]int{}
			}
			s.InsightsThresholds[scope] = *now.Insights
		} else {
			delete(s.InsightsThresholds, scope)
		}
	})
	return changed, err
}

// syncRepoConfigSoon does syncRepoConfig off the caller's path, and reloads the
// repository's graph if its settings turned out different. Opening a repository mustn't
// wait on it; Shutdown does.
func (a *App) syncRepoConfigSoon(path string) {
	a.inBackground(func() {
		changed, err := a.syncRepoConfig(context.Background(), path)
		if err != nil || !changed {
			return
		}
		if m, err := a.sessions(); err == nil {
			if r := m.Get(path); r != nil {
				r.ReloadGraph()
			}
		}
	})
}

// updateRepoSettings changes the settings of the repository at path, in the app's copy and
// in the repository's git config, where only what changed is written. A path that isn't a
// repository keeps them in the app alone.
func (a *App) updateRepoSettings(m *session.Manager, path string, change func(rs *settings.RepoSettings)) error {
	a.repoConfigMu.Lock()
	defer a.repoConfigMu.Unlock()
	before := m.Settings().Get().Repo(path)
	var after settings.RepoSettings
	err := m.Settings().Update(func(s *settings.Settings) {
		rs := s.Repo(path)
		change(&rs)
		rs.InGitConfig = true
		s.SetRepo(rs)
		after = rs
	})
	if err != nil {
		return err
	}
	ctx := context.Background()
	g, _, err := m.GitFor(ctx, path)
	if err != nil {
		return nil
	}
	return friendly(repoconfig.Save(ctx, g, toRepoConfig(before, nil), toRepoConfig(after, nil)))
}

// saveRepoThresholds writes a repository's Insights thresholds to its git config, given what
// applied before.
func (a *App) saveRepoThresholds(m *session.Manager, scope string, before *[4]int, after [4]int) error {
	root, ok := strings.CutPrefix(scope, "repo:")
	if !ok {
		return nil
	}
	ctx := context.Background()
	g, _, err := m.GitFor(ctx, root)
	if err != nil {
		return nil
	}
	return friendly(repoconfig.Save(ctx, g, repoconfig.Settings{Insights: before}, repoconfig.Settings{Insights: &after}))
}
