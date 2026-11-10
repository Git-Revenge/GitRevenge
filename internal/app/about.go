package app

import (
	"runtime"
	"runtime/debug"
	"strings"
)

// wailsModule is the module path of the Wails version this app is built on.
const wailsModule = "github.com/wailsapp/wails/v2"

// BuiltWith holds the versions of the projects this build was made with, for the About
// dialog. A version that can't be read is empty.
type BuiltWith struct {
	Go    string `json:"go"`
	Wails string `json:"wails"`
}

// BuiltWith returns the Go and Wails versions compiled into this build.
func (a *App) BuiltWith() BuiltWith {
	built := BuiltWith{Go: goVersion(runtime.Version())}
	if info, ok := debug.ReadBuildInfo(); ok {
		built.Wails = moduleVersion(info.Deps, wailsModule)
	}
	return built
}

// goVersion turns runtime.Version's "go1.27.3" into "1.27.3", dropping anything after it
// such as an experiment list. A development toolchain's version is returned as it is.
func goVersion(v string) string {
	rest, ok := strings.CutPrefix(v, "go")
	if !ok {
		return v
	}
	rest, _, _ = strings.Cut(rest, " ")
	return rest
}

// moduleVersion returns the version of the module at path among deps, without its "v", or
// "" when it isn't there. A replaced module answers with its replacement's version.
func moduleVersion(deps []*debug.Module, path string) string {
	for _, m := range deps {
		if m.Path != path {
			continue
		}
		if m.Replace != nil && m.Replace.Version != "" {
			m = m.Replace
		}
		return strings.TrimPrefix(m.Version, "v")
	}
	return ""
}
