// Package version exposes the application version.
package version

// Version is overridden at build time by the release workflow:
//
//	-ldflags "-X github.com/git-revenge/gitrevenge/internal/version.Version=0.1.0"
var Version = "0.9.1-dev"
