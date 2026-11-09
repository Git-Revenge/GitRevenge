package app

import (
	"os/exec"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/version"
)

func TestVersion(t *testing.T) {
	t.Parallel()
	if got := New().Version(); got != version.Version {
		t.Fatalf("Version() = %q, want %q", got, version.Version)
	}
}

func TestStartupDiscoversGit(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	env := New().Environment()
	if env.Problem != "" {
		t.Fatalf("unexpected problem: %s", env.Problem)
	}
	if env.GitPath == "" || env.GitVersion == "" || env.MinVersion != git.MinVersion.String() {
		t.Fatalf("incomplete environment: %+v", env)
	}
}
