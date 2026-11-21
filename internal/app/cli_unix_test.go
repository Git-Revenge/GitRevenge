//go:build !windows

package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testBundle = "/Applications/Git Revenge.app"

// noAdmin fails the test if the install asks for an administrator.
func noAdmin(t *testing.T) func(src, target string) error {
	return func(string, string) error {
		t.Helper()
		t.Error("asked for an administrator")
		return nil
	}
}

func TestInstallLauncherWritesARunnableScript(t *testing.T) {
	target := filepath.Join(t.TempDir(), "not", "there", "yet", "gitrevenge")
	script := launcherScript(testBundle)
	said, err := installLauncher(target, script, nil, noAdmin(t))
	if err != nil || !strings.Contains(said, "is installed") {
		t.Fatalf("said %q, err %v", said, err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != script {
		t.Fatalf("wrote %q, err %v", got, err)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o755 {
		t.Errorf("mode %v, want 0755", info.Mode().Perm())
	}
	if out, err := exec.Command("/bin/sh", "-n", target).CombinedOutput(); err != nil {
		t.Errorf("the script doesn't parse: %v\n%s", err, out)
	}

	// Asked again, it says so and changes nothing.
	said, err = installLauncher(target, script, nil, noAdmin(t))
	if err != nil || !strings.Contains(said, "already installed") {
		t.Errorf("the second time: said %q, err %v", said, err)
	}
}

// The script the app wrote runs open on the bundle with the arguments it was given.
func TestInstalledLauncherPassesItsArguments(t *testing.T) {
	dir := t.TempDir()
	// The same script, with a stand-in for /usr/bin/open that prints what it was run with.
	open := filepath.Join(dir, "open")
	if err := os.WriteFile(open, []byte("#!/bin/sh\nprintf '<%s>' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := strings.Replace(launcherScript(`/Apps/It's here/Git Revenge.app`), "/usr/bin/open", shellQuote(open), 1)
	target := filepath.Join(dir, "gitrevenge")
	if _, err := installLauncher(target, script, nil, noAdmin(t)); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(target, "a repo", "two").CombinedOutput()
	if want := `<-a></Apps/It's here/Git Revenge.app><--args><a repo><two>`; err != nil || string(out) != want {
		t.Errorf("ran with %s, err %v; want %s", out, err, want)
	}
}

func TestInstallLauncherReplacesItsOwnOlderScript(t *testing.T) {
	target := filepath.Join(t.TempDir(), "gitrevenge")
	if _, err := installLauncher(target, launcherScript("/Users/sam/Downloads/Git Revenge.app"), nil, noAdmin(t)); err != nil {
		t.Fatal(err)
	}
	script := launcherScript(testBundle)
	if said, err := installLauncher(target, script, nil, noAdmin(t)); err != nil || !strings.Contains(said, "is installed") {
		t.Fatalf("said %q, err %v", said, err)
	}
	if got, _ := os.ReadFile(target); string(got) != script {
		t.Errorf("still the older script:\n%s", got)
	}
}

func TestInstallLauncherLeavesSomeoneElsesCommand(t *testing.T) {
	target := filepath.Join(t.TempDir(), "gitrevenge")
	theirs := "#!/bin/sh\necho mine\n"
	if err := os.WriteFile(target, []byte(theirs), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := installLauncher(target, launcherScript(testBundle), nil, noAdmin(t)); err == nil {
		t.Error("no error for a command that isn't the app's")
	}
	if got, _ := os.ReadFile(target); string(got) != theirs {
		t.Errorf("their command was overwritten:\n%s", got)
	}
}

// Homebrew's cask links the command, either where the app would write it or in its own folder.
func TestInstallLauncherLeavesALinkedCommand(t *testing.T) {
	for _, where := range []string{"at the target", "elsewhere"} {
		t.Run(where, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "gitrevenge")
			link := target
			if where == "elsewhere" {
				link = filepath.Join(dir, "homebrew-gitrevenge")
			}
			if err := os.Symlink(filepath.Join(dir, "cask-wrapper.sh"), link); err != nil {
				t.Fatal(err)
			}
			said, err := installLauncher(target, launcherScript(testBundle), []string{link}, noAdmin(t))
			if err != nil || !strings.Contains(said, link) {
				t.Fatalf("said %q, err %v", said, err)
			}
			if where == "elsewhere" {
				if _, err := os.Lstat(target); err == nil {
					t.Error("wrote a second command beside the linked one")
				}
			}
		})
	}
}

// A folder this user can't write, as /usr/local/bin usually is, sends the script through the
// administrator's copy.
func TestInstallLauncherAsksAnAdministratorWhenItCannotWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(locked, "gitrevenge")
	script := launcherScript(testBundle)

	var copied string
	said, err := installLauncher(target, script, nil, func(src, to string) error {
		if to != target {
			t.Errorf("asked to copy to %q", to)
		}
		got, err := os.ReadFile(src)
		copied = string(got)
		return err
	})
	if err != nil || !strings.Contains(said, "is installed") {
		t.Fatalf("said %q, err %v", said, err)
	}
	if copied != script {
		t.Errorf("handed the administrator %q", copied)
	}

	// Turning down the password prompt isn't an error, and says nothing.
	said, err = installLauncher(target, script, nil, func(string, string) error { return errCancelled })
	if said != "" || err != nil {
		t.Errorf("after cancelling: said %q, err %v", said, err)
	}

	failed := errors.New("no")
	if _, err = installLauncher(target, script, nil, func(string, string) error { return failed }); !errors.Is(err, failed) {
		t.Errorf("after a failed copy: err %v", err)
	}
}
