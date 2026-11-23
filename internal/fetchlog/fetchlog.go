// Package fetchlog remembers when the app last fetched each of a repository's remotes.
//
// Git keeps no time per remote: FETCH_HEAD only says what the latest fetch brought in. So
// after each of its fetches the app writes the time down here, one small file per repository
// in a folder of the operating system's cache folder, named for the repository's shared git
// directory as the trailer index is. Nothing is written inside the repository. Losing the
// files loses nothing but the times.
package fetchlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// mu serialises writes, so two fetches finishing together don't lose one's times.
var mu sync.Mutex

// Dir returns the folder the files are kept in: GitRevenge/fetched under the operating
// system's cache folder, or "" when there's no such folder, and then nothing is kept.
func Dir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "GitRevenge", "fetched")
}

// fileFor names the file of the repository whose shared git directory is commonDir.
func fileFor(dir, commonDir string) string {
	key := filepath.Clean(commonDir)
	if resolved, err := filepath.EvalSymlinks(key); err == nil {
		key = resolved
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		key = strings.ToLower(key)
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, hex.EncodeToString(sum[:16])+".json")
}

// Read returns when each remote was last fetched, by name. It's empty when nothing has been
// written down, or the file can't be read.
func Read(dir, commonDir string) map[string]time.Time {
	times := map[string]time.Time{}
	if dir == "" || commonDir == "" {
		return times
	}
	data, err := os.ReadFile(fileFor(dir, commonDir))
	if err != nil {
		return times
	}
	var raw map[string]int64
	if json.Unmarshal(data, &raw) != nil {
		return times
	}
	for name, at := range raw {
		times[name] = time.Unix(at, 0)
	}
	return times
}

// Note writes down that the named remotes were fetched at at. Remotes it doesn't name keep
// their times. It does nothing when dir is "" and gives up quietly when the file can't be
// written: a missing time is shown as unknown, never as an error.
func Note(dir, commonDir string, names []string, at time.Time) {
	if dir == "" || commonDir == "" || len(names) == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	file := fileFor(dir, commonDir)
	raw := map[string]int64{}
	if data, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	for _, n := range names {
		raw[n] = at.Unix()
	}
	write(file, raw)
}

// Rename moves a remote's time to its new name.
func Rename(dir, commonDir, from, to string) {
	edit(dir, commonDir, func(raw map[string]int64) bool {
		at, ok := raw[from]
		if ok {
			delete(raw, from)
			raw[to] = at
		}
		return ok
	})
}

// Forget drops a removed remote's time.
func Forget(dir, commonDir, name string) {
	edit(dir, commonDir, func(raw map[string]int64) bool {
		_, ok := raw[name]
		delete(raw, name)
		return ok
	})
}

// edit rewrites the file when change says it changed something.
func edit(dir, commonDir string, change func(map[string]int64) bool) {
	if dir == "" || commonDir == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	file := fileFor(dir, commonDir)
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}
	raw := map[string]int64{}
	if json.Unmarshal(data, &raw) != nil || !change(raw) {
		return
	}
	write(file, raw)
}

// write replaces file with raw, through a temporary file so a reader never sees half of it.
func write(file string, raw map[string]int64) {
	data, err := json.Marshal(raw)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".fetched-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), file) != nil {
		_ = os.Remove(tmp.Name())
	}
}
