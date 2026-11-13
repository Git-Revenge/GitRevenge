//go:build darwin

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/git-revenge/gitrevenge/internal/fsutil"
)

// carryOverWebStorage copies the web view's stored data from the bundle ID used before 1.0
// (com.risadams.gitrevenge) to the current one, the first time 1.0 or later starts. WebKit
// keeps an app's local storage under its bundle ID, and the interface keeps panel sizes and
// similar layout there, so without this the layout would reset. Settings are in
// settings.json and don't depend on the bundle ID. The old copy is left for an older version.
func carryOverWebStorage() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	webkit := filepath.Join(home, "Library", "WebKit")
	if _, err := fsutil.CopyDirIfMissing(filepath.Join(webkit, "com.risadams.gitrevenge"), filepath.Join(webkit, "com.gitrevenge.gitrevenge")); err != nil {
		log.Printf("couldn't carry over the web view's stored layout: %v", err)
	}
}
