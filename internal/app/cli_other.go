//go:build !darwin

package app

// InstallCommandLineTool is for macOS. On Windows and Linux installing the app puts the
// gitrevenge command on the PATH, and the interface doesn't offer this.
func (a *App) InstallCommandLineTool() (string, error) {
	return "", userError("Installing Git Revenge already put the gitrevenge command on your PATH.")
}
