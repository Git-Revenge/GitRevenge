package app

import (
	"slices"

	"github.com/git-revenge/gitrevenge/internal/settings"
)

// Theme returns the saved theme: "light", "dark", or "" to follow the operating system.
func (a *App) Theme() (string, error) {
	m, err := a.sessions()
	if err != nil {
		return "", err
	}
	return m.Settings().Get().Theme, nil
}

// SetTheme saves the theme: "light", "dark", or "" to follow the operating system.
func (a *App) SetTheme(theme string) error {
	switch theme {
	case "", "light", "dark":
	default:
		return userError("Unknown theme " + theme + ".")
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) { s.Theme = theme })
}

// GraphDensity returns the saved graph row density: "compact", or "" for the timeline.
func (a *App) GraphDensity() (string, error) {
	m, err := a.sessions()
	if err != nil {
		return "", err
	}
	return m.Settings().Get().GraphDensity, nil
}

// SetGraphDensity saves the graph row density: "compact", or "" for the timeline.
func (a *App) SetGraphDensity(density string) error {
	switch density {
	case "", "compact":
	default:
		return userError("Unknown graph density " + density + ".")
	}
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) { s.GraphDensity = density })
}

// experimentalViews are the graph views that settings can switch on, by the names saved for
// them and in the order the view menu lists them.
var experimentalViews = []string{}

// GraphViews is what the graph's view menu needs.
type GraphViews struct {
	// View is the experimental view showing in place of the rows, or "" for the rows.
	View string `json:"view"`
	// Experimental are the experimental views switched on, in menu order.
	Experimental []string `json:"experimental"`
}

// enabledViews returns the experimental views switched on, leaving out names this version
// doesn't know (a settings file from a newer one).
func enabledViews(s settings.Settings) []string {
	on := []string{}
	for _, v := range experimentalViews {
		if slices.Contains(s.ExperimentalViews, v) {
			on = append(on, v)
		}
	}
	return on
}

// GraphViews returns the view the graph shows and the experimental views on offer. A saved
// view that's no longer switched on reads as the rows.
func (a *App) GraphViews() (GraphViews, error) {
	m, err := a.sessions()
	if err != nil {
		return GraphViews{}, err
	}
	s := m.Settings().Get()
	on := enabledViews(s)
	view := s.GraphView
	if !slices.Contains(on, view) {
		view = ""
	}
	return GraphViews{View: view, Experimental: on}, nil
}

// SetGraphView saves which view the graph shows: an experimental view that's switched on, or
// "" for the rows.
func (a *App) SetGraphView(view string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	if view != "" && !slices.Contains(enabledViews(m.Settings().Get()), view) {
		return userError("The " + view + " view isn't switched on. Turn it on in Settings first.")
	}
	return m.Settings().Update(func(s *settings.Settings) { s.GraphView = view })
}
