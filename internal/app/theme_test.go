package app

import (
	"reflect"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/settings"
)

func TestThemeRoundTrip(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)

	if got, err := a.Theme(); err != nil || got != "" {
		t.Fatalf("default theme = %q, %v", got, err)
	}
	if err := a.SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Theme(); got != "dark" {
		t.Fatalf("theme = %q", got)
	}
	if err := a.SetTheme("neon"); err == nil {
		t.Fatal("unknown theme accepted")
	}
	if got, _ := a.Theme(); got != "dark" {
		t.Fatalf("theme after bad set = %q", got)
	}
}

func TestGraphDensityRoundTrip(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)

	if got, err := a.GraphDensity(); err != nil || got != "" {
		t.Fatalf("default density = %q, %v", got, err)
	}
	if err := a.SetGraphDensity("compact"); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.GraphDensity(); got != "compact" {
		t.Fatalf("density = %q", got)
	}
	if err := a.SetGraphDensity("roomy"); err == nil {
		t.Fatal("unknown density accepted")
	}
}

// Stand-in experimental views, since which ones exist changes from version to version.
func withExperimentalViews(t *testing.T, views ...string) {
	t.Helper()
	before := experimentalViews
	experimentalViews = views
	t.Cleanup(func() { experimentalViews = before })
}

func TestGraphViewNeedsSwitchingOn(t *testing.T) {
	withExperimentalViews(t, "first", "second")
	a := newTestApp(t)

	got, err := a.GraphViews()
	if err != nil || got.View != "" || len(got.Experimental) != 0 {
		t.Fatalf("default views = %+v, %v", got, err)
	}
	if err := a.SetGraphView("first"); err == nil {
		t.Fatal("a view that isn't switched on was accepted")
	}

	p, _ := a.Preferences()
	p.ExperimentalViews = []string{"second", "first"}
	if err := a.SetPreferences(p); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGraphView("second"); err != nil {
		t.Fatal(err)
	}
	got, _ = a.GraphViews()
	if got.View != "second" || !reflect.DeepEqual(got.Experimental, []string{"first", "second"}) {
		t.Fatalf("views = %+v, want second showing and both on offer in menu order", got)
	}
	if err := a.SetGraphView("third"); err == nil {
		t.Fatal("unknown view accepted")
	}

	// Switching the view off in settings sends the graph back to its rows.
	p.ExperimentalViews = []string{"first"}
	if err := a.SetPreferences(p); err != nil {
		t.Fatal(err)
	}
	got, _ = a.GraphViews()
	if got.View != "" || !reflect.DeepEqual(got.Experimental, []string{"first"}) {
		t.Fatalf("views after switching off = %+v", got)
	}
	if err := a.SetGraphView(""); err != nil {
		t.Fatal(err)
	}
}

// A settings file from a newer version can name a view this one doesn't have.
func TestGraphViewIgnoresUnknownSavedView(t *testing.T) {
	withExperimentalViews(t, "first")
	a := newTestApp(t)
	m, err := a.sessions()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Settings().Update(func(s *settings.Settings) {
		s.ExperimentalViews = []string{"later", "first"}
		s.GraphView = "later"
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := a.GraphViews()
	if got.View != "" || !reflect.DeepEqual(got.Experimental, []string{"first"}) {
		t.Fatalf("views = %+v", got)
	}
}
