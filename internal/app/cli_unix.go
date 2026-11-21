//go:build !windows

package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// installLauncher writes script as the command at target and returns what to tell the user.
// A command the app didn't write is left alone: a link, such as Homebrew makes (also looked
// for at each of others), or a file without launcherMark. When the folder can't be written
// as this user, the script goes to a temporary file and asAdmin copies it into place.
// asAdmin returns errCancelled when the user turns down the password prompt.
func installLauncher(target, script string, others []string, asAdmin func(src, target string) error) (string, error) {
	for _, p := range append([]string{target}, others...) {
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return "A gitrevenge command is already installed at " + p + ", by Homebrew or by hand. It was left as it is.", nil
		}
	}
	switch existing, err := os.ReadFile(target); {
	case err == nil && string(existing) == script:
		return "The gitrevenge command is already installed.", nil
	case err == nil && !strings.Contains(string(existing), launcherMark):
		return "", userError("There's already a different gitrevenge command at " + target + ". Remove it first if you want this one.")
	}

	const installed = "The gitrevenge command is installed. A new terminal window has it."
	err := writeLauncher(target, script)
	if err == nil {
		return installed, nil
	}
	if !errors.Is(err, fs.ErrPermission) {
		return "", err
	}
	tmp, err := os.CreateTemp("", "gitrevenge-launcher-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.WriteString(script)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	switch err := asAdmin(tmp.Name(), target); {
	case errors.Is(err, errCancelled):
		return "", nil
	case err != nil:
		return "", err
	}
	return installed, nil
}

// errCancelled is what an asAdmin returns when the user turned down the password prompt.
var errCancelled = errors.New("cancelled")

// writeLauncher writes the command as the current user, making its folder if need be.
func writeLauncher(target, script string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, []byte(script), 0o755); err != nil {
		return err
	}
	// WriteFile's mode is cut down by the umask, and isn't applied to a file that was there.
	return os.Chmod(target, 0o755)
}
