package app

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPreferencesRoundTrip(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	p, err := a.Preferences()
	if err != nil {
		t.Fatal(err)
	}
	if p.Zoom != 100 || !p.UpdateChecks || p.Theme != "" {
		t.Fatalf("defaults = %+v", p)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	want := Preferences{Theme: "dark", GraphDensity: "compact", UpdateChecks: false, GitPath: gitPath, CloneDir: dir, Zoom: 125}
	if err := a.SetPreferences(want); err != nil {
		t.Fatal(err)
	}
	got, _ := a.Preferences()
	got.GitInUse = ""
	if got != want {
		t.Fatalf("saved = %+v, want %+v", got, want)
	}
	if a.gitOverride() != gitPath {
		t.Fatalf("gitOverride = %q", a.gitOverride())
	}
}

func TestPreferencesValidation(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	base := Preferences{UpdateChecks: true, Zoom: 100}
	bad := map[string]Preferences{
		"theme":          {Theme: "neon", Zoom: 100},
		"zoom":           {Zoom: 300},
		"missing git":    {Zoom: 100, GitPath: filepath.Join(t.TempDir(), "nope", "git")},
		"relative dir":   {Zoom: 100, CloneDir: "repos"},
		"missing dir":    {Zoom: 100, CloneDir: filepath.Join(t.TempDir(), "absent")},
		"density":        {Zoom: 100, GraphDensity: "dense"},
		"not git at all": {Zoom: 100, GitPath: t.TempDir()},
	}
	for name, p := range bad {
		if err := a.SetPreferences(p); err == nil {
			t.Errorf("%s: accepted %+v", name, p)
		}
	}
	if err := a.SetPreferences(base); err != nil {
		t.Fatalf("defaults rejected: %v", err)
	}
}

func TestKeybindings(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	if got, err := a.Keybindings(); err != nil || len(got) != 0 {
		t.Fatalf("defaults = %v, %v", got, err)
	}
	want := map[string]string{"repo.fetch": "Mod+Alt+F", "app.palette": "", "view.plus": "Mod++"}
	if err := a.SetKeybindings(want); err != nil {
		t.Fatal(err)
	}
	got, _ := a.Keybindings()
	if len(got) != 3 || got["repo.fetch"] != "Mod+Alt+F" || got["app.palette"] != "" || got["view.plus"] != "Mod++" {
		t.Fatalf("saved = %v", got)
	}
	for _, bad := range []string{"Hyper+K", "Mod+"} {
		if err := a.SetKeybindings(map[string]string{"x": bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
