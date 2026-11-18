// Package fsutil holds small file system helpers.
package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SamePath reports whether two repository paths refer to the same location. Windows and
// macOS (by default) have case-insensitive file systems.
func SamePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// SameFolder reports whether two paths lead to the same place, however each is spelled. One
// folder can be reached by several spellings: through a symbolic link (on macOS /var is one,
// to /private/var), a Windows junction or substituted drive, or a short name ("RUNNER~1").
// git reports a folder's real path, so a path kept from elsewhere is compared this way: by
// asking the file system whether the two are the same folder. Paths that can't be read are
// the same only if they're spelled the same.
func SameFolder(a, b string) bool {
	if SamePath(a, b) {
		return true
	}
	ia, err := os.Stat(a)
	if err != nil {
		return false
	}
	ib, err := os.Stat(b)
	return err == nil && os.SameFile(ia, ib)
}
