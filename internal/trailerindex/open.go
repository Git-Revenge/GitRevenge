package trailerindex

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// An index's file is named for the repository it belongs to, in a folder of the operating
// system's cache folder. Linked worktrees share a git directory and so share an index. A
// repository that moves gets a new name and is read again; the old file is cleared out
// once it hasn't been opened for a while.

// suffix ends every index file's name.
const suffix = ".trailers"

// unusedFor is how long an index file is kept after it was last opened.
const unusedFor = 90 * 24 * time.Hour

var (
	openMu sync.Mutex
	// opened holds the indexes in use, by file, so two tabs on one repository's worktrees
	// read into one.
	opened = map[string]*Index{}
)

// Dir returns the folder index files are kept in: GitRevenge/trailers under the operating
// system's cache folder. It's "" when there's no such folder, and indexes are then kept in
// memory only.
func Dir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "GitRevenge", "trailers")
}

// fileFor names the index file of the repository whose shared git directory is commonDir.
func fileFor(dir, commonDir string) string {
	key := filepath.Clean(commonDir)
	if resolved, err := filepath.EvalSymlinks(key); err == nil {
		key = resolved
	}
	// The same folder is spelled in more than one case where the file system doesn't mind.
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		key = strings.ToLower(key)
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, hex.EncodeToString(sum[:16])+suffix)
}

// Open returns the index of the repository whose shared git directory is commonDir, kept
// under dir. Nothing is read yet: the file is loaded by the first Read. Each Open must be
// matched by a Close. With dir "" the index is kept in memory only.
func Open(dir, commonDir string) *Index {
	if dir == "" {
		x := New()
		x.refs = 1
		return x
	}
	path := fileFor(dir, commonDir)
	openMu.Lock()
	defer openMu.Unlock()
	if x := opened[path]; x != nil {
		x.refs++
		return x
	}
	x := &Index{path: path, parser: git.TrailerParserVersion, refs: 1}
	opened[path] = x
	// The folder is made here so the first write doesn't have to think about it, and the
	// file's time says when the index was last wanted.
	if err := os.MkdirAll(dir, 0o700); err == nil {
		now := time.Now()
		_ = os.Chtimes(path, now, now)
		go clearUnused(dir, now)
	}
	return x
}

// Close says the caller is done with the index. Its memory goes when the last one is.
func (x *Index) Close() {
	openMu.Lock()
	defer openMu.Unlock()
	x.refs--
	if x.refs <= 0 && x.path != "" && opened[x.path] == x {
		delete(opened, x.path)
	}
}

// clearUnused removes index files that haven't been opened for a long time: the indexes of
// repositories that were deleted or moved.
func clearUnused(dir string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < unusedFor {
			continue
		}
		path := filepath.Join(dir, e.Name())
		openMu.Lock()
		inUse := opened[path] != nil
		openMu.Unlock()
		if !inUse {
			_ = os.Remove(path)
		}
	}
}
