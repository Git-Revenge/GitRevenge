package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/settings"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Preferences are the settings the settings panel edits.
type Preferences struct {
	// Theme is "light", "dark" or "" for the operating system's.
	Theme string `json:"theme"`
	// GraphDensity is "compact" or "" for the timeline.
	GraphDensity string `json:"graphDensity"`
	UpdateChecks bool   `json:"updateChecks"`
	// GitPath is a git executable to use instead of the one found automatically, or "".
	// It takes effect the next time the app starts.
	GitPath string `json:"gitPath"`
	// GitInUse is the git this session runs; read-only.
	GitInUse string `json:"gitInUse"`
	// CloneDir is the folder the clone dialog starts in, or "".
	CloneDir string `json:"cloneDir"`
	// Zoom scales the interface, as a percentage (80 to 150).
	Zoom int `json:"zoom"`
}

// Zoom limits, as percentages.
const (
	minZoom = 80
	maxZoom = 150
)

// gitOverride is the git executable chosen in settings, read before the settings store
// opens (discovery runs first).
func (a *App) gitOverride() string {
	path := a.settingsPath
	if path == "" {
		var err error
		if path, err = settings.DefaultPath(); err != nil {
			return ""
		}
	}
	return settings.Read(path).GitPath
}

// Preferences returns the settings the settings panel edits.
func (a *App) Preferences() (Preferences, error) {
	m, err := a.sessions()
	if err != nil {
		return Preferences{}, err
	}
	s := m.Settings().Get()
	zoom := s.Zoom
	if zoom == 0 {
		zoom = 100
	}
	return Preferences{
		Theme:        s.Theme,
		GraphDensity: s.GraphDensity,
		UpdateChecks: !s.UpdateChecksOff,
		GitPath:      s.GitPath,
		GitInUse:     a.Environment().GitPath,
		CloneDir:     s.CloneDir,
		Zoom:         zoom,
	}, nil
}

// SetPreferences checks and saves the settings panel's settings.
func (a *App) SetPreferences(p Preferences) error {
	switch p.Theme {
	case "", "light", "dark":
	default:
		return userError("Unknown theme " + p.Theme + ".")
	}
	switch p.GraphDensity {
	case "", "compact":
	default:
		return userError("Unknown graph density " + p.GraphDensity + ".")
	}
	if p.Zoom < minZoom || p.Zoom > maxZoom {
		return userError("Zoom must be between 80% and 150%.")
	}
	p.GitPath = strings.TrimSpace(p.GitPath)
	if p.GitPath != "" {
		in, err := git.Probe(context.Background(), p.GitPath)
		if err != nil {
			return userError("That doesn't look like a working git: " + err.Error())
		}
		if in.Version.Less(git.MinVersion) {
			return userError("That git is " + in.Version.String() + "; Git Revenge needs " + git.MinVersion.String() + " or newer.")
		}
	}
	p.CloneDir = strings.TrimSpace(p.CloneDir)
	if p.CloneDir != "" {
		if !filepath.IsAbs(p.CloneDir) {
			return userError("Choose a full path for the clone folder.")
		}
		if info, err := os.Stat(p.CloneDir); err != nil || !info.IsDir() {
			return userError("The clone folder " + p.CloneDir + " doesn't exist.")
		}
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	zoom := p.Zoom
	if zoom == 100 {
		zoom = 0
	}
	return m.Settings().Update(func(s *settings.Settings) {
		s.Theme = p.Theme
		s.GraphDensity = p.GraphDensity
		s.UpdateChecksOff = !p.UpdateChecks
		s.GitPath = p.GitPath
		s.CloneDir = p.CloneDir
		s.Zoom = zoom
	})
}

// ChooseFile shows a file picker and returns the chosen path, or "" if cancelled.
func (a *App) ChooseFile(title string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

// Keybindings returns the shortcuts the user has changed, by command id ("" for unbound).
func (a *App) Keybindings() (map[string]string, error) {
	m, err := a.sessions()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for id, keys := range m.Settings().Get().Keybindings {
		out[id] = keys
	}
	return out, nil
}

// SetKeybindings replaces the user's shortcut changes. Each value is "" (unbound) or keys in
// "Mod+Shift+K" form ending in one key.
func (a *App) SetKeybindings(bindings map[string]string) error {
	for id, keys := range bindings {
		if id == "" || len(id) > 200 {
			return userError("A shortcut has no command.")
		}
		if keys == "" {
			continue
		}
		parts := strings.Split(keys, "+")
		if parts[len(parts)-1] == "" && !strings.HasSuffix(keys, "++") {
			return userError("The shortcut " + keys + " has no key.")
		}
		for _, mod := range parts[:len(parts)-1] {
			switch mod {
			case "Mod", "Shift", "Alt", "Ctrl", "Meta", "":
			default:
				return userError("Unknown modifier " + mod + " in " + keys + ".")
			}
		}
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) {
		s.Keybindings = map[string]string{}
		for id, keys := range bindings {
			s.Keybindings[id] = keys
		}
	})
}
