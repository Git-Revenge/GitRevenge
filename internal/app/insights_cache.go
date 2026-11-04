package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// Insights results are kept in memory for the session (ADR-0008: nothing is stored). Each
// is keyed by its repository, view, inputs and the state of every ref, so any change to the
// refs (a commit, a fetch, a merge) misses the cache and recomputes. A hit is always what a
// fresh computation would give.

// maxCachedInsights bounds the cache; past it, the cache starts over.
const maxCachedInsights = 64

type insightsCache struct {
	mu      sync.Mutex
	results map[string]any
	// conflicts holds dry-run merge results by target and branch SHA, which never change.
	conflicts map[string]git.Conflict
}

func (c *insightsCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.results[key]
	return v, ok
}

func (c *insightsCache) put(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.results == nil || len(c.results) >= maxCachedInsights {
		c.results = map[string]any{}
	}
	c.results[key] = v
}

func conflictKey(targetSHA, branchSHA string) string { return targetSHA + "\x00" + branchSHA }

func (c *insightsCache) conflict(targetSHA, branchSHA string) (git.Conflict, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.conflicts[conflictKey(targetSHA, branchSHA)]
	return v, ok
}

func (c *insightsCache) putConflict(targetSHA, branchSHA string, v git.Conflict) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conflicts == nil || len(c.conflicts) >= 16*maxCachedInsights {
		c.conflicts = map[string]git.Conflict{}
	}
	c.conflicts[conflictKey(targetSHA, branchSHA)] = v
}

// refState fingerprints every ref and the commit each points at.
func refState(ctx context.Context, g *git.Repo) (string, error) {
	out, err := g.Read(ctx, "for-each-ref", "--format=%(objectname) %(refname)")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(out)
	return hex.EncodeToString(sum[:]), nil
}

// cachedInsight returns the cached result of one view for these inputs and the current
// refs, or computes and caches it.
func cachedInsight[T any](ctx context.Context, a *App, g *git.Repo, root, view string, in InsightsInputs, compute func() (T, error)) (T, error) {
	var zero T
	refs, err := refState(ctx, g)
	if err != nil {
		return zero, err
	}
	inputs, err := json.Marshal(in)
	if err != nil {
		return zero, err
	}
	// Branch kinds decide which branches and items count (ADR 0009), so they're part of
	// the key too.
	kinds, err := json.Marshal(a.branchRules(root).KindPatterns())
	if err != nil {
		return zero, err
	}
	key := view + "\x00" + root + "\x00" + string(inputs) + "\x00" + string(kinds) + "\x00" + refs
	if v, ok := a.insights.get(key); ok {
		if t, ok := v.(T); ok {
			return t, nil
		}
	}
	v, err := compute()
	if err != nil {
		return zero, err
	}
	a.insights.put(key, v)
	return v, nil
}

// SetInsightsTab remembers the Insights tab's scope ("repo:<path>" or "group:<name>") for
// the next launch; "" records that the tab is closed.
func (a *App) SetInsightsTab(scope string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) { s.InsightsTab = scope })
}

// InsightsThresholds returns the thresholds saved for a scope, with defaults for any not set.
func (a *App) InsightsThresholds(scope string) (Thresholds, error) {
	m, err := a.sessions()
	if err != nil {
		return Thresholds{}, err
	}
	v := m.Settings().Get().InsightsThresholds[scope]
	return Thresholds{Stale: v[0], LongLived: v[1], Behind: v[2], Size: v[3]}.withDefaults(), nil
}

// SetInsightsThresholds saves a scope's thresholds. Values below 1 are rejected.
func (a *App) SetInsightsThresholds(scope string, t Thresholds) error {
	if t.Stale < 1 || t.LongLived < 1 || t.Behind < 1 || t.Size < 1 {
		return userError("Thresholds must be 1 or more.")
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) {
		if s.InsightsThresholds == nil {
			s.InsightsThresholds = map[string][4]int{}
		}
		s.InsightsThresholds[scope] = [4]int{t.Stale, t.LongLived, t.Behind, t.Size}
	})
}

// InsightsFetchedAt returns when the repository last fetched (FETCH_HEAD's modification
// time, Unix seconds), or 0 if it never has. It's for display only: file times aren't
// repository state, so they never enter a result.
func (a *App) InsightsFetchedAt(path string) (int64, error) {
	ctx := context.Background()
	g, _, err := a.repoGit(ctx, path)
	if err != nil {
		return 0, err
	}
	config, err := g.LocalConfigPath(ctx)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(config), "FETCH_HEAD"))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return info.ModTime().Unix(), nil
}
