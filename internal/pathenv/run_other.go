//go:build !windows

package pathenv

import "io"

// Run is only for the Windows installer. Elsewhere the packages put the app on the PATH.
func Run([]string, io.Writer) (code int, handled bool) { return 0, false }
