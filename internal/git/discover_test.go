package git

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want Version
	}{
		{"git version 2.56.0.windows.4", Version{2, 56, 0}},
		{"git version 2.39.5 (Apple Git-154)", Version{2, 39, 5}},
		{"git version 2.40.1", Version{2, 40, 1}},
		{"git version 3.0", Version{3, 0, 0}},
	}
	for _, tt := range tests {
		got, err := ParseVersion(tt.in)
		if err != nil {
			t.Errorf("ParseVersion(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseVersion(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
	if _, err := ParseVersion("not git"); err == nil {
		t.Error("ParseVersion(garbage) should fail")
	}
}

func TestVersionLess(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b Version
		want bool
	}{
		{Version{2, 39, 5}, MinVersion, true},
		{Version{2, 40, 0}, MinVersion, false},
		{Version{2, 56, 0}, MinVersion, false},
		{Version{1, 99, 99}, MinVersion, true},
		{Version{3, 0, 0}, MinVersion, false},
	}
	for _, tt := range tests {
		if got := tt.a.Less(tt.b); got != tt.want {
			t.Errorf("%v.Less(%v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	in, err := Discover(context.Background())
	var tooOld *TooOldError
	if errors.As(err, &tooOld) {
		t.Skipf("installed git is older than the minimum: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if in.Path == "" || in.Version.Major < 2 {
		t.Fatalf("unexpected install %+v", in)
	}
}
