package app

import (
	"runtime/debug"
	"testing"
)

func TestGoVersion(t *testing.T) {
	for in, want := range map[string]string{
		"go1.27.3":                      "1.27.3",
		"go1.27.3 X:nocoverageredesign": "1.27.3",
		"devel +abc123":                 "devel +abc123",
	} {
		if got := goVersion(in); got != want {
			t.Errorf("goVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestModuleVersion(t *testing.T) {
	deps := []*debug.Module{
		{Path: "example.com/other", Version: "v0.1.0"},
		{Path: wailsModule, Version: "v2.17.0"},
		{Path: "example.com/replaced", Version: "v1.0.0", Replace: &debug.Module{Path: "example.com/fork", Version: "v1.0.1"}},
		{Path: "example.com/local", Version: "v1.0.0", Replace: &debug.Module{Path: "../local"}},
	}
	for path, want := range map[string]string{
		wailsModule:            "2.17.0",
		"example.com/replaced": "1.0.1",
		"example.com/local":    "1.0.0",
		"example.com/missing":  "",
	} {
		if got := moduleVersion(deps, path); got != want {
			t.Errorf("moduleVersion(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestBuiltWithNamesGo(t *testing.T) {
	if got := New().BuiltWith().Go; got == "" {
		t.Error("BuiltWith().Go is empty")
	}
}
