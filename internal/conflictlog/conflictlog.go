// Package conflictlog remembers the conflicts resolved in the app while rerere is off, so it
// can notice one coming back and offer to turn rerere on, once (#938).
//
// Each conflict is kept as its fingerprint (git.Repo.ConflictFingerprint): a hash of the
// conflicting text, never the text. One small file per repository, in a folder of the
// operating system's cache folder, named for the repository's shared git directory as
// fetchlog's are. Nothing is written inside the repository. Losing the files loses nothing but
// the chance to notice a repeat.
package conflictlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// keep is the most fingerprints kept for a repository; the oldest go first.
const keep = 500

// mu serialises writes, so two resolutions saved together don't lose one.
var mu sync.Mutex

// Dir returns the folder the files are kept in: GitRevenge/conflicts under the operating
// system's cache folder, or "" when there's no such folder, and then nothing is kept.
func Dir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "GitRevenge", "conflicts")
}

// record is a repository's file.
type record struct {
	// Resolved maps each fingerprint to when it was last resolved, in Unix seconds.
	Resolved map[string]int64 `json:"resolved"`
	// Offered says the app has offered to turn rerere on here, and isn't to again.
	Offered bool `json:"offered,omitempty"`
}

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

func read(dir, commonDir string) record {
	rec := record{Resolved: map[string]int64{}}
	if dir == "" || commonDir == "" {
		return rec
	}
	if data, err := os.ReadFile(fileFor(dir, commonDir)); err == nil {
		_ = json.Unmarshal(data, &rec)
	}
	if rec.Resolved == nil {
		rec.Resolved = map[string]int64{}
	}
	return rec
}

// Resolved notes that the conflict with fingerprint fp was resolved at at. It gives up quietly
// when the file can't be written.
func Resolved(dir, commonDir, fp string, at time.Time) {
	if dir == "" || commonDir == "" || fp == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	rec := read(dir, commonDir)
	rec.Resolved[fp] = at.Unix()
	if len(rec.Resolved) > keep {
		fps := make([]string, 0, len(rec.Resolved))
		for k := range rec.Resolved {
			fps = append(fps, k)
		}
		sort.Slice(fps, func(i, j int) bool { return rec.Resolved[fps[i]] < rec.Resolved[fps[j]] })
		for _, k := range fps[:len(fps)-keep] {
			delete(rec.Resolved, k)
		}
	}
	write(fileFor(dir, commonDir), rec)
}

// Seen says which of fps were resolved here before.
func Seen(dir, commonDir string, fps []string) []bool {
	rec := read(dir, commonDir)
	out := make([]bool, len(fps))
	for i, fp := range fps {
		_, out[i] = rec.Resolved[fp]
	}
	return out
}

// Offered says the offer to turn rerere on has been made here.
func Offered(dir, commonDir string) bool {
	return read(dir, commonDir).Offered
}

// SetOffered notes that the offer has been made, taken or turned down: it isn't made again.
func SetOffered(dir, commonDir string) {
	if dir == "" || commonDir == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	rec := read(dir, commonDir)
	rec.Offered = true
	write(fileFor(dir, commonDir), rec)
}

// write replaces file with rec, through a temporary file so a reader never sees half of it.
func write(file string, rec record) {
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".conflicts-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), file) != nil {
		_ = os.Remove(tmp.Name())
	}
}
