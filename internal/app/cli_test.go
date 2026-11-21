package app

import (
	"strings"
	"testing"
)

func TestLauncherScriptOpensTheBundle(t *testing.T) {
	got := launcherScript("/Applications/Git Revenge.app")
	want := "#!/bin/sh\n" + launcherMark + "\nexec /usr/bin/open -a '/Applications/Git Revenge.app' --args \"$@\"\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A folder name can hold anything, so the bundle's path is one quoted word whatever is in it.
func TestLauncherScriptQuotesTheBundle(t *testing.T) {
	got := launcherScript(`/Users/o'brien/My "Apps" $HOME/Git Revenge.app`)
	if !strings.Contains(got, `open -a '/Users/o'\''brien/My "Apps" $HOME/Git Revenge.app' --args`) {
		t.Errorf("got:\n%s", got)
	}
}

func TestBundleOf(t *testing.T) {
	tests := []struct {
		name, exe, want string
	}{
		{"in Applications", "/Applications/Git Revenge.app/Contents/MacOS/gitrevenge", "/Applications/Git Revenge.app"},
		{"in a user's folder", "/Users/sam/Applications/Git Revenge.app/Contents/MacOS/gitrevenge", "/Users/sam/Applications/Git Revenge.app"},
		{"a bare binary, as wails dev runs", "/Users/sam/src/GitRevenge/build/bin/gitrevenge", ""},
		{"not in MacOS", "/Applications/Git Revenge.app/Contents/Resources/gitrevenge", ""},
		{"run from where it was downloaded", "/private/var/folders/x/T/AppTranslocation/1B2C/d/Git Revenge.app/Contents/MacOS/gitrevenge", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bundleOf(tt.exe)
			if got != tt.want || (err == nil) != (tt.want != "") {
				t.Errorf("bundleOf(%q) = %q, %v; want %q", tt.exe, got, err, tt.want)
			}
		})
	}
}
