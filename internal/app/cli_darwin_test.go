package app

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The script that copies the launcher as an administrator can't be run in a test, since it
// asks for a password. Compiling it at least proves it's AppleScript.
func TestAdminCopyScriptCompiles(t *testing.T) {
	out, err := exec.Command("/usr/bin/osacompile", "-o", filepath.Join(t.TempDir(), "copy.scpt"), "-e", adminCopyScript).CombinedOutput()
	if err != nil {
		t.Fatalf("osacompile: %v\n%s", err, out)
	}
}
