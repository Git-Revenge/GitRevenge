package app

import (
	"path"
	"strings"
)

// The gitrevenge command, for starting the app from a terminal. The Linux packages install
// the app itself on the PATH and the Windows installer adds its launchers to it, but a Mac
// app dragged out of a disk image has no install step, so there the app installs the
// command when asked (InstallCommandLineTool).

// launcherMark is a line of every launcher script the app writes, by which it knows one of
// its own from a gitrevenge command somebody else put there.
const launcherMark = "# Opens Git Revenge. Written by the app's \"Install the gitrevenge command\"."

// launcherScript is the gitrevenge command on a Mac: a script that opens the app bundle the
// way Finder does. It returns at once, and when Git Revenge is already running it brings
// that one forward instead of starting another.
func launcherScript(bundle string) string {
	return "#!/bin/sh\n" + launcherMark + "\nexec /usr/bin/open -a " + shellQuote(bundle) + " --args \"$@\"\n"
}

// shellQuote quotes s as one word for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// bundleOf returns the .app that holds the executable at exe, a path with its symlinks
// resolved: <bundle>.app/Contents/MacOS/<name>.
func bundleOf(exe string) (string, error) {
	macOS := path.Dir(exe)
	bundle := path.Dir(path.Dir(macOS))
	if path.Base(macOS) != "MacOS" || !strings.HasSuffix(bundle, ".app") {
		return "", userError("This build isn't an app bundle, so there's nothing for the command to open.")
	}
	// An app opened where it was downloaded, or from its disk image, runs from a private
	// copy that macOS makes for that one run. A command pointing there would stop working.
	if strings.Contains(bundle, "/AppTranslocation/") {
		return "", userError("Move Git Revenge to the Applications folder and open it from there, then install the command.")
	}
	return bundle, nil
}
