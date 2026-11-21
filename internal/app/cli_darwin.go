package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cliCommand is where the app installs its command. /usr/local/bin is on everyone's PATH on
// a Mac (it's in /etc/paths), though writing there usually takes an administrator.
const cliCommand = "/usr/local/bin/gitrevenge"

// homebrewCommand is where Homebrew on Apple silicon links the command its cask installs.
// On an Intel Mac it links it at cliCommand.
const homebrewCommand = "/opt/homebrew/bin/gitrevenge"

// InstallCommandLineTool installs the gitrevenge command, which opens this app from a
// terminal, and returns what to tell the user. It returns "" when they turned down the
// password prompt.
func (a *App) InstallCommandLineTool() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return "", err
	}
	bundle, err := bundleOf(exe)
	if err != nil {
		return "", err
	}
	return installLauncher(cliCommand, launcherScript(bundle), []string{homebrewCommand}, copyAsAdmin)
}

// adminCopyScript is the AppleScript that copies the launcher into place as root. macOS
// asks for an administrator's password first. The paths reach it through the environment,
// so none of them is ever spliced into the script's text.
const adminCopyScript = `do shell script "/bin/mkdir -p " & quoted form of (system attribute "GITREVENGE_CLI_DIR") & " && /usr/bin/install -m 0755 " & quoted form of (system attribute "GITREVENGE_CLI_SRC") & " " & quoted form of (system attribute "GITREVENGE_CLI_TARGET") with prompt "Git Revenge wants to install the gitrevenge command." with administrator privileges`

func copyAsAdmin(src, target string) error {
	cmd := exec.Command("/usr/bin/osascript", "-e", adminCopyScript)
	cmd.Env = append(os.Environ(),
		"GITREVENGE_CLI_DIR="+filepath.Dir(target),
		"GITREVENGE_CLI_SRC="+src,
		"GITREVENGE_CLI_TARGET="+target,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	// -128 is AppleScript's "User canceled".
	if strings.Contains(string(out), "(-128)") {
		return errCancelled
	}
	return userError("Couldn't install the gitrevenge command: " + strings.TrimSpace(string(out)))
}
