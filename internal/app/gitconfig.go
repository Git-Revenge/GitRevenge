package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gitconfig"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// gitConfigs holds the config files open in the Git Config tab (ADR-0007).
type gitConfigs struct {
	once sync.Once
	err  error

	mu      sync.Mutex
	runner  *git.Runner
	workDir string
	docs    map[string]*gitconfig.Doc
	backups *gitconfig.Backups
	watcher *gitconfig.Watcher
}

// ConfigScopes are the config files the Git Config tab can edit.
type ConfigScopes struct {
	Global string `json:"global"`
	// GlobalAlt is the XDG global file when it exists alongside ~/.gitconfig; git reads both
	// but writes to Global.
	GlobalAlt string `json:"globalAlt"`
	// Local is the repository's config file, empty without a repository.
	Local string `json:"local"`
}

// ConfigDoc is an open config file as the editor shows it.
type ConfigDoc struct {
	Path string `json:"path"`
	// Text is the working copy with "\n" line endings.
	Text string `json:"text"`
	// Entries are the working copy's key = value lines, in file order.
	Entries []git.ConfigEntry `json:"entries"`
	// Exists is false for a file that will be created on save.
	Exists bool `json:"exists"`
	Dirty  bool `json:"dirty"`
	// ChangedOnDisk means the file changed outside the editor since it was opened or saved.
	ChangedOnDisk bool `json:"changedOnDisk"`
	// Problem is set when git can't parse the working copy; Entries is then empty.
	Problem *ConfigProblem `json:"problem"`
}

// ConfigProblem is why git rejected a config file or edit.
type ConfigProblem struct {
	// Line is 1-based, or 0 when the problem isn't about one line.
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ConfigSaveResult is the outcome of SaveGitConfig.
type ConfigSaveResult struct {
	Saved bool      `json:"saved"`
	Doc   ConfigDoc `json:"doc"`
	// DiskText is the file as changed on disk, when saving stopped because of that, so the
	// UI can show what would be overwritten.
	DiskText string `json:"diskText"`
}

// ConfigChange is the payload of the "gitconfig:changed" event.
type ConfigChange struct {
	Path string `json:"path"`
}

// gitConfig returns the open-file registry, starting the watcher on first use.
func (a *App) gitConfig() (*gitConfigs, error) {
	c := &a.configs
	c.once.Do(func() {
		env := a.Environment()
		if env.Problem != "" {
			c.err = errors.New(env.Problem)
			return
		}
		dir := a.configBackupDir
		if dir == "" {
			p, err := settings.DefaultPath()
			if err != nil {
				c.err = err
				return
			}
			dir = filepath.Join(filepath.Dir(p), "config-backups")
		}
		c.workDir, c.err = os.MkdirTemp("", "gitrevenge-config-*")
		if c.err != nil {
			return
		}
		c.runner = git.NewRunner(env.GitPath)
		c.docs = map[string]*gitconfig.Doc{}
		c.backups = &gitconfig.Backups{Dir: dir, Keep: gitconfig.DefaultKeep}
		c.watcher = gitconfig.NewWatcher(func(path string) {
			// A change to the global config can change what every open repository inherits.
			if a.sess != nil {
				for _, r := range a.sess.List() {
					a.syncRepoConfigSoon(r.Path)
				}
			}
			a.emit("gitconfig:changed", ConfigChange{Path: path})
		})
		if global, alt, err := c.runner.GlobalConfigPath(context.Background()); err == nil {
			c.watcher.Add(global)
			if alt != "" {
				c.watcher.Add(alt)
			}
		}
	})
	return c, c.err
}

func (c *gitConfigs) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.watcher != nil {
		c.watcher.Close()
	}
	for _, d := range c.docs {
		d.Close()
	}
	if c.workDir != "" {
		_ = os.RemoveAll(c.workDir)
	}
}

// SetGitConfigTab remembers the Git Config tab's selected file for the next launch; an
// empty path records that the tab is closed.
func (a *App) SetGitConfigTab(path string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) { s.ConfigTab = path })
}

// GitConfigScopes returns the global config file and, with a repository path, its local one.
func (a *App) GitConfigScopes(repoPath string) (ConfigScopes, error) {
	c, err := a.gitConfig()
	if err != nil {
		return ConfigScopes{}, err
	}
	ctx := context.Background()
	var s ConfigScopes
	if s.Global, s.GlobalAlt, err = c.runner.GlobalConfigPath(ctx); err != nil {
		return ConfigScopes{}, err
	}
	if repoPath != "" {
		g, _, err := a.repoGit(ctx, repoPath)
		if err != nil {
			return ConfigScopes{}, err
		}
		if s.Local, err = g.LocalConfigPath(ctx); err != nil {
			return ConfigScopes{}, err
		}
	}
	return s, nil
}

// EffectiveGitConfig lists every config entry git sees from the repository at repoPath (or,
// with no path, from outside any repository), with its scope and file, in the order git
// reads them. It reads the files on disk, not unsaved edits.
func (a *App) EffectiveGitConfig(repoPath string) ([]git.ConfigEntry, error) {
	c, err := a.gitConfig()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	dir := ""
	if repoPath != "" {
		if _, dir, err = a.repoGit(ctx, repoPath); err != nil {
			return nil, err
		}
	}
	entries, err := c.runner.EffectiveConfig(ctx, dir)
	if entries == nil {
		entries = []git.ConfigEntry{}
	}
	return entries, err
}

// repoGit returns a git handle for a repository and its root, whether or not it is open as
// a tab.
func (a *App) repoGit(ctx context.Context, path string) (*git.Repo, string, error) {
	m, err := a.sessions()
	if err != nil {
		return nil, "", err
	}
	g, root, err := m.GitFor(ctx, path)
	return g, root, friendly(err)
}

// OpenGitConfig opens a config file for editing, or returns it with its edits if already
// open. A file that doesn't exist yet opens empty.
func (a *App) OpenGitConfig(path string) (ConfigDoc, error) {
	c, err := a.gitConfig()
	if err != nil {
		return ConfigDoc{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, err := c.openLocked(path)
	if err != nil {
		return ConfigDoc{}, err
	}
	return c.view(d)
}

func (c *gitConfigs) openLocked(path string) (*gitconfig.Doc, error) {
	id := gitconfig.Normalize(path)
	if d := c.docs[id]; d != nil {
		return d, nil
	}
	d, err := gitconfig.Open(path, c.workDir)
	if err != nil {
		return nil, err
	}
	c.docs[id] = d
	c.watcher.Add(path)
	return d, nil
}

// doc returns an already open file.
func (c *gitConfigs) doc(path string) (*gitconfig.Doc, error) {
	if d := c.docs[gitconfig.Normalize(path)]; d != nil {
		return d, nil
	}
	return nil, userError("That config file isn't open.")
}

// view describes d's working copy for the UI.
func (c *gitConfigs) view(d *gitconfig.Doc) (ConfigDoc, error) {
	text, err := d.Text()
	if err != nil {
		return ConfigDoc{}, err
	}
	v := ConfigDoc{Path: d.Path, Text: text, Exists: d.Exists(), Entries: []git.ConfigEntry{}}
	if v.Dirty, err = d.Dirty(); err != nil {
		return ConfigDoc{}, err
	}
	if v.ChangedOnDisk, err = d.ChangedOnDisk(); err != nil {
		return ConfigDoc{}, err
	}
	entries, err := c.runner.FileConfig(context.Background(), d.WorkPath())
	var ce *git.ConfigError
	switch {
	case errors.As(err, &ce):
		v.Problem = &ConfigProblem{Line: ce.Line, Message: ce.Message}
	case err != nil:
		return ConfigDoc{}, err
	}
	for _, e := range entries {
		e.File = d.Path
		v.Entries = append(v.Entries, e)
	}
	return v, nil
}

// edit runs a `git config` edit against an open file's working copy and returns the result.
func (a *App) edit(path string, args ...string) (ConfigDoc, error) {
	c, err := a.gitConfig()
	if err != nil {
		return ConfigDoc{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, err := c.doc(path)
	if err != nil {
		return ConfigDoc{}, err
	}
	if err := c.runner.EditConfig(context.Background(), d.WorkPath(), args...); err != nil {
		return ConfigDoc{}, configEditErr(err)
	}
	return c.view(d)
}

// configEditErr shows git's reason for rejecting an edit.
func configEditErr(err error) error {
	var ce *git.ConfigError
	if errors.As(err, &ce) {
		return userError(ce.Message)
	}
	if errors.Is(err, git.ErrNoSuchKey) {
		return userError("That key isn't set in this file.")
	}
	return err
}

// SetGitConfig sets key to value in an open file, replacing its value. Keys with several
// values must use ReplaceGitConfigValue instead.
func (a *App) SetGitConfig(path, key, value string) (ConfigDoc, error) {
	return a.edit(path, "--", key, value)
}

// AddGitConfigValue adds another value for a multi-valued key.
func (a *App) AddGitConfigValue(path, key, value string) (ConfigDoc, error) {
	return a.edit(path, "--add", "--", key, value)
}

// ReplaceGitConfigValue replaces one value of a key, leaving its other values alone.
func (a *App) ReplaceGitConfigValue(path, key, oldValue, newValue string) (ConfigDoc, error) {
	return a.edit(path, "--fixed-value", "--", key, newValue, oldValue)
}

// UnsetGitConfig removes key from an open file: every value, or with value set, just that one.
func (a *App) UnsetGitConfig(path, key, value string) (ConfigDoc, error) {
	if value == "" {
		return a.edit(path, "--unset-all", "--", key)
	}
	return a.edit(path, "--unset", "--fixed-value", "--", key, value)
}

// RenameGitConfigKey moves every value of key from to key to, as when renaming an alias.
func (a *App) RenameGitConfigKey(path, from, to string) (ConfigDoc, error) {
	c, err := a.gitConfig()
	if err != nil {
		return ConfigDoc{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, err := c.doc(path)
	if err != nil {
		return ConfigDoc{}, err
	}
	ctx := context.Background()
	entries, err := c.runner.FileConfig(ctx, d.WorkPath())
	if err != nil {
		return ConfigDoc{}, configEditErr(err)
	}
	var values []string
	for _, e := range entries {
		if git.SameConfigKey(e.Key, from) {
			values = append(values, e.Value)
		}
	}
	if len(values) == 0 {
		return ConfigDoc{}, userError("That key isn't set in this file.")
	}
	if slices.ContainsFunc(entries, func(e git.ConfigEntry) bool { return git.SameConfigKey(e.Key, to) }) {
		return ConfigDoc{}, userError(to + " is already set in this file.")
	}
	for _, v := range values {
		if err := c.runner.EditConfig(ctx, d.WorkPath(), "--add", "--", to, v); err != nil {
			return ConfigDoc{}, configEditErr(err)
		}
	}
	if err := c.runner.EditConfig(ctx, d.WorkPath(), "--unset-all", "--", from); err != nil {
		return ConfigDoc{}, configEditErr(err)
	}
	return c.view(d)
}

// RenameGitConfigSection renames a section, such as `remote.old` to `remote.new`.
func (a *App) RenameGitConfigSection(path, from, to string) (ConfigDoc, error) {
	return a.edit(path, "--rename-section", from, to)
}

// RemoveGitConfigSection removes a section and every key in it.
func (a *App) RemoveGitConfigSection(path, section string) (ConfigDoc, error) {
	return a.edit(path, "--remove-section", section)
}

// SetGitConfigText replaces an open file's working copy with text from the raw editor. Text
// git can't parse is kept; the result's Problem says where it fails.
func (a *App) SetGitConfigText(path, text string) (ConfigDoc, error) {
	c, err := a.gitConfig()
	if err != nil {
		return ConfigDoc{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, err := c.doc(path)
	if err != nil {
		return ConfigDoc{}, err
	}
	if err := d.SetText(text); err != nil {
		return ConfigDoc{}, err
	}
	return c.view(d)
}

// SaveGitConfig writes an open file's working copy, after backing up the current file. It
// doesn't save a working copy git can't parse, and unless force is set it doesn't
// overwrite a file that changed on disk since it was opened; in both cases Saved is false.
func (a *App) SaveGitConfig(path string, force bool) (ConfigSaveResult, error) {
	c, err := a.gitConfig()
	if err != nil {
		return ConfigSaveResult{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, err := c.doc(path)
	if err != nil {
		return ConfigSaveResult{}, err
	}
	validate := func(work string) error {
		_, err := c.runner.FileConfig(context.Background(), work)
		return err
	}
	err = d.Save(validate, c.backups, force)
	var ce *git.ConfigError
	var res ConfigSaveResult
	switch {
	case err == nil:
		res.Saved = true
	case errors.As(err, &ce):
	case errors.Is(err, gitconfig.ErrChangedOnDisk):
		disk, err := d.DiskText()
		if err != nil {
			return ConfigSaveResult{}, err
		}
		res.DiskText = string(disk)
	case errors.Is(err, gitconfig.ErrLocked):
		return ConfigSaveResult{}, userError("Another git process is writing " + d.Path + ". Try again in a moment. If no git process is running, delete " + d.Path + ".lock.")
	default:
		return ConfigSaveResult{}, err
	}
	if res.Doc, err = c.view(d); err != nil {
		return ConfigSaveResult{}, err
	}
	return res, nil
}

// ReloadGitConfig discards an open file's edits and reads it again from disk.
func (a *App) ReloadGitConfig(path string) (ConfigDoc, error) {
	c, err := a.gitConfig()
	if err != nil {
		return ConfigDoc{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	d, err := c.doc(path)
	if err != nil {
		return ConfigDoc{}, err
	}
	if err := d.Reload(); err != nil {
		return ConfigDoc{}, err
	}
	return c.view(d)
}

// CloseGitConfig discards an open file's working copy.
func (a *App) CloseGitConfig(path string) error {
	c, err := a.gitConfig()
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	id := gitconfig.Normalize(path)
	if d := c.docs[id]; d != nil {
		d.Close()
		delete(c.docs, id)
	}
	return nil
}

// GitConfigBackups lists a config file's backups, newest first.
func (a *App) GitConfigBackups(path string) ([]gitconfig.Backup, error) {
	c, err := a.gitConfig()
	if err != nil {
		return nil, err
	}
	return c.backups.List(path)
}

// GitConfigBackupText returns the content of one backup.
func (a *App) GitConfigBackupText(path, id string) (string, error) {
	c, err := a.gitConfig()
	if err != nil {
		return "", err
	}
	b, err := c.backups.Read(path, id)
	if errors.Is(err, os.ErrNotExist) {
		return "", userError("That backup no longer exists.")
	}
	return string(b), err
}

// GitConfigFileText returns a config file as it is on disk now, with "\n" line endings, or
// "" if it doesn't exist. History compares backups with it.
func (a *App) GitConfigFileText(path string) (string, error) {
	if _, err := a.gitConfig(); err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n"), err
}

// RestoreGitConfig replaces a config file with one of its backups, backing up the current
// file first so the restore can be undone. If the file is open, its edits are discarded.
func (a *App) RestoreGitConfig(path, id string) (ConfigDoc, error) {
	c, err := a.gitConfig()
	if err != nil {
		return ConfigDoc{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := c.backups.Read(path, id)
	if errors.Is(err, os.ErrNotExist) {
		return ConfigDoc{}, userError("That backup no longer exists.")
	}
	if err != nil {
		return ConfigDoc{}, err
	}
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		current = nil
	} else if err != nil {
		return ConfigDoc{}, err
	}
	if err := gitconfig.Write(path, data, current, c.backups); err != nil {
		if errors.Is(err, gitconfig.ErrLocked) {
			return ConfigDoc{}, userError("Another git process is writing " + path + ". Try again in a moment.")
		}
		return ConfigDoc{}, err
	}
	d, err := c.openLocked(path)
	if err != nil {
		return ConfigDoc{}, err
	}
	if err := d.Reload(); err != nil {
		return ConfigDoc{}, err
	}
	return c.view(d)
}

// GitConfigKeys lists every config key the installed git documents, for autocompletion.
func (a *App) GitConfigKeys() ([]string, error) {
	c, err := a.gitConfig()
	if err != nil {
		return nil, err
	}
	return c.runner.ConfigKeys(context.Background())
}

// GitCommands lists git's built-in commands, which aliases can't override.
func (a *App) GitCommands() ([]string, error) {
	c, err := a.gitConfig()
	if err != nil {
		return nil, err
	}
	return c.runner.Commands(context.Background())
}
