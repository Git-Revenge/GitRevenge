package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
)

// MinVersion is the oldest git release Git Revenge supports: 2.50, for partial clones
// (GIT_NO_LAZY_FETCH, 2.44) and downloading what one is missing (git backfill, 2.49).
var MinVersion = Version{2, 50, 0}

// Version is a git release number.
type Version struct {
	Major, Minor, Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Less reports whether v is older than o.
func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

var versionRE = regexp.MustCompile(`git version (\d+)\.(\d+)(?:\.(\d+))?`)

// ParseVersion parses `git version` output, e.g. "git version 2.56.0.windows.4".
func ParseVersion(s string) (Version, error) {
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("unrecognized git version output %q", s)
	}
	var v Version
	v.Major, _ = strconv.Atoi(m[1])
	v.Minor, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		v.Patch, _ = strconv.Atoi(m[3])
	}
	return v, nil
}

// Install describes the git installation the app will use.
type Install struct {
	Path    string
	Version Version
	// LFS is the `git lfs version` output, or empty if git-lfs is not installed.
	LFS string
}

// ErrNotFound means no git binary could be located.
var ErrNotFound = errors.New("git not found")

// TooOldError means the located git is older than MinVersion.
type TooOldError struct {
	Install Install
}

func (e *TooOldError) Error() string {
	return fmt.Sprintf("git %s at %s is too old; Git Revenge needs %s or newer", e.Install.Version, e.Install.Path, MinVersion)
}

// Discover locates git, checks its version, and probes for git-lfs. A non-nil Install is
// returned alongside a *TooOldError so the UI can show what was found.
func Discover(ctx context.Context) (*Install, error) {
	path, err := findBinary()
	if err != nil {
		return nil, err
	}
	return Probe(ctx, path)
}

// Probe checks the git at path like Discover does: its version, and git-lfs beside it.
func Probe(ctx context.Context, path string) (*Install, error) {
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return nil, fmt.Errorf("%s: %w", path, ErrNotFound)
	}
	r := NewRunner(path)
	out, err := r.Output(ctx, Cmd{Args: []string{"version"}})
	if err != nil {
		return nil, fmt.Errorf("running %s: %w", path, err)
	}
	v, err := ParseVersion(out)
	if err != nil {
		return nil, err
	}
	in := &Install{Path: path, Version: v}
	if lfs, err := r.Output(ctx, Cmd{Args: []string{"lfs", "version"}}); err == nil {
		in.LFS = lfs
	}
	if v.Less(MinVersion) {
		return in, &TooOldError{Install: *in}
	}
	return in, nil
}

// findBinary looks on PATH, then in well-known install locations. macOS apps launched from
// Finder don't inherit the shell's PATH, so Homebrew installs must be checked explicitly.
func findBinary() (string, error) {
	if p, err := exec.LookPath("git"); err == nil {
		return filepath.Abs(p)
	}
	for _, p := range candidatePaths() {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return "", ErrNotFound
}

func candidatePaths() []string {
	switch runtime.GOOS {
	case "windows":
		var paths []string
		for _, env := range []string{"ProgramFiles", "ProgramW6432", "LocalAppData"} {
			if dir := os.Getenv(env); dir != "" {
				paths = append(paths,
					filepath.Join(dir, "Git", "cmd", "git.exe"),
					filepath.Join(dir, "Programs", "Git", "cmd", "git.exe"))
			}
		}
		return paths
	case "darwin":
		return []string{"/opt/homebrew/bin/git", "/usr/local/bin/git", "/usr/bin/git"}
	default:
		return []string{"/usr/bin/git", "/usr/local/bin/git"}
	}
}
