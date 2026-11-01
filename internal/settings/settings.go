// Package settings persists app state as a local JSON file (ADR-0005).
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/risadams/gitrevenge/internal/fsutil"
)

// schemaVersion is bumped when the file format changes incompatibly.
const schemaVersion = 1

// ConfigTabID is ActiveTab's value when the Git Config tab is selected. It can't be a
// repository path.
const ConfigTabID = "gitrevenge:gitconfig"

// InsightsTabID is ActiveTab's value when the Insights tab is selected.
const InsightsTabID = "gitrevenge:insights"

// maxRecent caps the recently opened repositories list.
const maxRecent = 20

// Settings is everything the app remembers between launches.
type Settings struct {
	Version int `json:"version"`
	// Recent lists recently opened repositories, most recent first.
	Recent []RecentRepo `json:"recent"`
	// OpenTabs holds the repository paths open as tabs, in tab order.
	OpenTabs []string `json:"openTabs"`
	// ActiveTab is the path of the selected tab, ConfigTabID or InsightsTabID for those tabs,
	// or empty.
	ActiveTab string `json:"activeTab"`
	// ConfigTab is the config file selected in the Git Config tab, or empty when that tab
	// is closed.
	ConfigTab string `json:"configTab,omitempty"`
	// InsightsTab is the Insights tab's scope, "repo:<path>" or "group:<name>", or empty
	// when that tab is closed.
	InsightsTab string `json:"insightsTab,omitempty"`
	// InsightsThresholds are the Insights tab's branch thresholds by scope ("repo:<path>" or
	// "group:<name>"): days stale, days long-lived, commits behind, lines changed. A view
	// preference; every export prints the values it used.
	InsightsThresholds map[string][4]int `json:"insightsThresholds,omitempty"`
	// Groups are user-defined collections of repositories. A repo may be in several.
	Groups []Group `json:"groups"`
	// Theme is "light" or "dark", or empty to follow the operating system.
	Theme string `json:"theme,omitempty"`
	// GraphDensity is "compact" for single-line graph rows, or empty for the default
	// two-line timeline.
	GraphDensity string `json:"graphDensity,omitempty"`
	// UpdateChecksOff stops the check for a newer release at startup. A check the user
	// asks for still runs.
	UpdateChecksOff bool `json:"updateChecksOff,omitempty"`
	// GitPath is a git executable to use instead of the one found on PATH, or empty.
	GitPath string `json:"gitPath,omitempty"`
	// CloneDir is the folder the clone dialog starts in, or empty for the last repo's.
	CloneDir string `json:"cloneDir,omitempty"`
	// Zoom scales the whole interface, as a percentage; 0 means 100.
	Zoom int `json:"zoom,omitempty"`
	// Keybindings overrides command shortcuts by command id, in "Mod+Shift+K" form; an empty
	// value unbinds the command.
	Keybindings map[string]string `json:"keybindings,omitempty"`
	// Repos holds settings that belong to one repository, such as its branch kinds.
	Repos []RepoSettings `json:"repos,omitempty"`
}

// RepoSettings are one repository's own settings (ADR 0009).
type RepoSettings struct {
	Path string `json:"path"`
	// Kinds overrides a branch kind's patterns, keyed by kind ("trunk", "release",
	// "environment", "docs", "bot", "feature"). A kind not listed uses its defaults; one listed with no
	// patterns matches nothing.
	Kinds map[string][]string `json:"kinds,omitempty"`
	// Hide lists glob patterns of branches to leave out of the sidebar and the graph.
	Hide []string `json:"hide,omitempty"`
	// HideKinds hides every branch of these kinds (the graph's Docs and Bots toggles).
	HideKinds []string `json:"hideKinds,omitempty"`
	// CleanupOnPull deletes local branches whose upstream is gone and that have nothing
	// unpushed, after each pull.
	CleanupOnPull bool `json:"cleanupOnPull,omitempty"`
	// SyncNotes fetches refs/notes/* on fetch and pull, and pushes changed notes on push.
	SyncNotes bool `json:"syncNotes,omitempty"`
	// StackParents overrides which local branch each branch is stacked on, by branch name;
	// "" means not stacked. Branches not listed are inferred.
	StackParents map[string]string `json:"stackParents,omitempty"`
}

func (r RepoSettings) clone() RepoSettings {
	r.Hide = slices.Clone(r.Hide)
	r.HideKinds = slices.Clone(r.HideKinds)
	r.StackParents = maps.Clone(r.StackParents)
	if r.Kinds != nil {
		kinds := make(map[string][]string, len(r.Kinds))
		for k, v := range r.Kinds {
			kinds[k] = slices.Clone(v)
		}
		r.Kinds = kinds
	}
	return r
}

// empty reports whether r holds nothing but its path, so it can be dropped.
func (r RepoSettings) empty() bool {
	return r.Kinds == nil && len(r.Hide) == 0 && len(r.HideKinds) == 0 && len(r.StackParents) == 0 && !r.CleanupOnPull && !r.SyncNotes
}

// Repo returns the settings for the repository at path, or empty ones.
func (s Settings) Repo(path string) RepoSettings {
	for _, r := range s.Repos {
		if fsutil.SamePath(r.Path, path) {
			return r.clone()
		}
	}
	return RepoSettings{Path: path}
}

// SetRepo stores r, replacing any settings for the same path, and drops it when it holds
// nothing.
func (s *Settings) SetRepo(r RepoSettings) {
	s.Repos = slices.DeleteFunc(s.Repos, func(o RepoSettings) bool { return fsutil.SamePath(o.Path, r.Path) })
	if !r.empty() {
		s.Repos = append(s.Repos, r.clone())
	}
}

// Group is a named collection of repository paths.
type Group struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

// RecentRepo is an entry in the recent repositories list.
type RecentRepo struct {
	Path       string    `json:"path"`
	LastOpened time.Time `json:"lastOpened"`
}

func defaults() Settings {
	return Settings{Version: schemaVersion, Recent: []RecentRepo{}, OpenTabs: []string{}, Groups: []Group{}}
}

// clone returns a deep copy so callers can't mutate the store's state.
func (s Settings) clone() Settings {
	s.Recent = slices.Clone(s.Recent)
	s.OpenTabs = slices.Clone(s.OpenTabs)
	groups := make([]Group, len(s.Groups))
	for i, g := range s.Groups {
		groups[i] = Group{Name: g.Name, Paths: slices.Clone(g.Paths)}
	}
	s.Groups = groups
	s.InsightsThresholds = maps.Clone(s.InsightsThresholds)
	repos := make([]RepoSettings, len(s.Repos))
	for i, r := range s.Repos {
		repos[i] = r.clone()
	}
	s.Repos = repos
	return s
}

// AddRecent moves path to the front of the recent list, adding it if needed.
func (s *Settings) AddRecent(path string, now time.Time) {
	s.Recent = slices.DeleteFunc(s.Recent, func(r RecentRepo) bool { return fsutil.SamePath(r.Path, path) })
	s.Recent = slices.Insert(s.Recent, 0, RecentRepo{Path: path, LastOpened: now})
	if len(s.Recent) > maxRecent {
		s.Recent = s.Recent[:maxRecent]
	}
}

// RemoveRecent drops path from the recent list.
func (s *Settings) RemoveRecent(path string) {
	s.Recent = slices.DeleteFunc(s.Recent, func(r RecentRepo) bool { return fsutil.SamePath(r.Path, path) })
}

// DefaultPath returns the settings file location: %APPDATA%\GitRevenge\settings.json on
// Windows and ~/Library/Application Support/GitRevenge/settings.json on macOS.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "GitRevenge", "settings.json"), nil
}

// Read loads settings from path without opening a store, for reading a setting before the
// store exists (the git executable, at startup). A missing or unreadable file gives defaults.
func Read(path string) Settings {
	s := defaults()
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

// Store loads and saves Settings. It is safe for concurrent use.
type Store struct {
	path string

	mu  sync.Mutex
	cur Settings
}

// Open loads settings from path. A missing file yields defaults. A file that can't be
// parsed is renamed aside (settings.json.corrupt-<timestamp>) and defaults are used, so a
// bad file never stops the app from starting.
func Open(path string) (*Store, error) {
	s := &Store{path: path, cur: defaults()}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s, nil
	case err != nil:
		return nil, fmt.Errorf("reading settings: %w", err)
	}

	loaded := defaults()
	if err := json.Unmarshal(data, &loaded); err != nil || loaded.Version > schemaVersion {
		aside := fmt.Sprintf("%s.corrupt-%s", path, time.Now().UTC().Format("20060102T150405Z"))
		if rerr := os.Rename(path, aside); rerr != nil {
			return nil, fmt.Errorf("settings file %s is unreadable and could not be moved aside: %w", path, rerr)
		}
		return s, nil
	}
	loaded.Version = schemaVersion
	if loaded.Recent == nil {
		loaded.Recent = []RecentRepo{}
	}
	if loaded.OpenTabs == nil {
		loaded.OpenTabs = []string{}
	}
	if loaded.Groups == nil {
		loaded.Groups = []Group{}
	}
	s.cur = loaded
	return s, nil
}

// Path returns the settings file location.
func (s *Store) Path() string { return s.path }

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur.clone()
}

// Update applies fn to a copy of the settings and saves the result. If saving fails, the
// in-memory settings are left unchanged.
func (s *Store) Update(fn func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cur.clone()
	fn(&next)
	if err := writeAtomic(s.path, next); err != nil {
		return err
	}
	s.cur = next
	return nil
}

// writeAtomic writes to a temp file in the same directory, syncs it, then renames it over
// the target, so a crash mid-write never leaves a truncated settings file.
func writeAtomic(path string, v Settings) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating settings dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("saving settings: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("saving settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	return nil
}

// GroupIndex returns the index of the group named name, or -1.
func (s *Settings) GroupIndex(name string) int {
	return slices.IndexFunc(s.Groups, func(g Group) bool { return g.Name == name })
}

// AddToGroup adds path to the named group unless it is already there.
func (s *Settings) AddToGroup(name, path string) bool {
	i := s.GroupIndex(name)
	if i < 0 {
		return false
	}
	if !slices.ContainsFunc(s.Groups[i].Paths, func(p string) bool { return fsutil.SamePath(p, path) }) {
		s.Groups[i].Paths = append(s.Groups[i].Paths, path)
	}
	return true
}

// MoveGroup moves the named group to index to. It reports false, changing nothing, for an
// unknown group or an index out of range.
func (s *Settings) MoveGroup(name string, to int) bool {
	i := s.GroupIndex(name)
	if i < 0 || to < 0 || to >= len(s.Groups) {
		return false
	}
	g := s.Groups[i]
	s.Groups = slices.Insert(slices.Delete(s.Groups, i, i+1), to, g)
	return true
}

// MoveInGroup moves path to index to within the named group. It reports false, changing
// nothing, for an unknown group or path, or an index out of range.
func (s *Settings) MoveInGroup(name, path string, to int) bool {
	g := s.GroupIndex(name)
	if g < 0 {
		return false
	}
	paths := s.Groups[g].Paths
	i := slices.IndexFunc(paths, func(p string) bool { return fsutil.SamePath(p, path) })
	if i < 0 || to < 0 || to >= len(paths) {
		return false
	}
	p := paths[i]
	s.Groups[g].Paths = slices.Insert(slices.Delete(paths, i, i+1), to, p)
	return true
}

// RemoveFromGroup removes path from the named group.
func (s *Settings) RemoveFromGroup(name, path string) {
	if i := s.GroupIndex(name); i >= 0 {
		s.Groups[i].Paths = slices.DeleteFunc(s.Groups[i].Paths, func(p string) bool { return fsutil.SamePath(p, path) })
	}
}
