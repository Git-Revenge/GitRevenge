package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// RerereOn says whether git reuses recorded conflict resolutions here (#938): rerere.enabled,
// or, when that's unset, an rr-cache folder, as git decides it.
func (r *Repo) RerereOn(ctx context.Context) bool {
	if v, err := r.ReadString(ctx, "config", "--type=bool", "--get", "rerere.enabled"); err == nil && v != "" {
		return v == "true"
	}
	dir, err := r.ReadString(ctx, "rev-parse", "--git-path", "rr-cache")
	if err != nil {
		return false
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(r.dir, dir)
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// TurnOnRerere sets rerere.enabled in this repository's own config. rerere records a conflict
// as it happens, so in the middle of one it's run once as well: the conflicts still open are
// recorded, and resolving them now is what's reused next time.
func (r *Repo) TurnOnRerere(ctx context.Context) error {
	if _, err := r.runner.Run(ctx, Cmd{Dir: r.dir, Args: []string{"config", "rerere.enabled", "true"}}); err != nil {
		return err
	}
	_, _ = r.runner.Run(ctx, Cmd{Dir: r.dir, Args: []string{"rerere"}})
	return nil
}

// stagesOf reads index stage lines, "mode id stage\tpath", into each path's ids by stage.
func stagesOf(out []byte) map[string][4]string {
	files := map[string][4]string{}
	for _, entry := range bytes.Split(out, []byte{0}) {
		meta, path, ok := strings.Cut(string(entry), "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) != 3 || len(f[2]) != 1 || f[2][0] < '1' || f[2][0] > '3' {
			continue
		}
		s := files[path]
		s[f[2][0]-'0'] = f[1]
		files[path] = s
	}
	return files
}

// RerereResolved lists the files of the conflict in progress that rerere settled with a
// resolution recorded before (#938), so they aren't taken for files that merged cleanly. git
// lists neither in rerere status nor rerere remaining once it has: what's left to go on is that
// the file was conflicted, and that rerere, which records every conflict it can, didn't record
// this one. With rerere.autoUpdate the file is staged, and the index's resolve-undo record
// still has its sides. A conflict rerere can't hold (a side deleted, or a binary file) is never
// counted.
func (r *Repo) RerereResolved(ctx context.Context) ([]string, error) {
	if !r.RerereOn(ctx) {
		return nil, nil
	}
	// Three reads, each a git start: on Windows each is worth running together.
	var unmerged, undo, status []byte
	var errs [3]error
	var wg sync.WaitGroup
	for i, args := range [][]string{{"ls-files", "-u", "-z"}, {"ls-files", "--resolve-undo", "-z"}, {"rerere", "status"}} {
		wg.Go(func() {
			out, err := r.Read(ctx, args...)
			errs[i] = err
			switch i {
			case 0:
				unmerged = out
			case 1:
				undo = out
			default:
				status = out
			}
		})
	}
	wg.Wait()
	if err := errors.Join(errs[:]...); err != nil {
		return nil, err
	}
	tracked := map[string]bool{}
	for line := range strings.SplitSeq(string(status), "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			tracked[line] = true
		}
	}
	candidates := stagesOf(unmerged)
	// resolve-undo outlives the commit that ends an operation: only a file whose sides are
	// still what HEAD and the commit coming in have belongs to the conflict in progress. With
	// nothing coming in, nothing is in progress.
	undone := map[string][4]string{}
	for path, s := range stagesOf(undo) {
		if _, ok := candidates[path]; !ok && !tracked[path] && s[2] != "" && s[3] != "" {
			undone[path] = s
		}
	}
	if len(undone) > 0 {
		if incoming := r.incomingCommit(ctx); incoming != "" {
			paths := make([]string, 0, len(undone))
			for p := range undone {
				paths = append(paths, p)
			}
			ours, theirs := r.blobIDsAt(ctx, "HEAD", paths), r.blobIDsAt(ctx, incoming, paths)
			for path, s := range undone {
				if ours[path] == s[2] && theirs[path] == s[3] {
					candidates[path] = s
				}
			}
		}
	}
	var out []string
	for path, s := range candidates {
		if tracked[path] || s[2] == "" || s[3] == "" || r.binaryBlob(ctx, s[2]) {
			continue
		}
		out = append(out, path)
	}
	slices.Sort(out)
	return out, nil
}

// incomingCommit is what the operation in progress brings in, as the side git calls theirs: a
// merge's MERGE_HEAD, a cherry-pick's or a rebase stop's commit, or for a revert the reverted
// commit's parent. "" when no operation is in progress.
func (r *Repo) incomingCommit(ctx context.Context) string {
	for _, ref := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REBASE_HEAD", "REVERT_HEAD^"} {
		if _, err := r.ReadString(ctx, "rev-parse", "-q", "--verify", ref+"^{commit}"); err == nil {
			return ref
		}
	}
	return ""
}

// blobIDsAt reads the blob ids of paths in commit, by path, in one listing.
func (r *Repo) blobIDsAt(ctx context.Context, commit string, paths []string) map[string]string {
	ids := map[string]string{}
	out, err := r.Read(ctx, append([]string{"ls-tree", "-z", commit, "--"}, paths...)...)
	if err != nil {
		return ids
	}
	for _, entry := range bytes.Split(out, []byte{0}) {
		meta, path, ok := strings.Cut(string(entry), "\t")
		if f := strings.Fields(meta); ok && len(f) == 3 && f[1] == "blob" {
			ids[path] = f[2]
		}
	}
	return ids
}

// binaryBlob says a blob reads as binary, as git decides it: a NUL in its first 8,000 bytes.
func (r *Repo) binaryBlob(ctx context.Context, id string) bool {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"cat-file", "blob", id}, Env: noLazyFetch})
	if err != nil {
		return false
	}
	return bytes.IndexByte(out[:min(len(out), 8000)], 0) >= 0
}

// ErrNoResolution means there's no resolution recorded for the file's conflict to forget.
var ErrNoResolution = errors.New("no recorded resolution")

// ForgetResolution throws away the resolution rerere recorded for file's conflict and puts the
// conflict back in the file, to be resolved again (#938). A file rerere staged is unstaged
// first: git checkout -m rebuilds its conflict from the index's resolve-undo record.
func (r *Repo) ForgetResolution(ctx context.Context, file string) error {
	if _, err := r.runner.Run(ctx, Cmd{Dir: r.dir, Args: []string{"checkout", "-m", "--", file}}); err != nil {
		return err
	}
	out, err := r.runner.Run(ctx, Cmd{Dir: r.dir, Args: []string{"rerere", "forget", "--", file}})
	if err != nil {
		return err
	}
	if strings.Contains(string(out), "no remembered resolution") {
		return ErrNoResolution
	}
	return nil
}

// ConflictFingerprint names a conflicted file's conflict by its text, as rerere does: each
// conflicting hunk's two sides, the smaller first so a rebase that swaps them matches, hashed
// together. The file's name and the lines around the hunks don't count, so the same conflict
// coming back in a later rebase, or another file, has the same name. "" for a conflict without
// two sides to compare (a side deleted), or a binary file.
func (r *Repo) ConflictFingerprint(ctx context.Context, file string) (string, error) {
	out, err := r.Read(ctx, "ls-files", "-u", "-z", "--", file)
	if err != nil {
		return "", err
	}
	return r.fingerprintOf(ctx, stagesOf(out)[file]), nil
}

// ConflictFingerprints names every conflicted file's conflict, as ConflictFingerprint does, at
// most limit of them, read four at a time: each is a git start.
func (r *Repo) ConflictFingerprints(ctx context.Context, limit int) (map[string]string, error) {
	out, err := r.Read(ctx, "ls-files", "-u", "-z")
	if err != nil {
		return nil, err
	}
	stages := stagesOf(out)
	paths := make([]string, 0, len(stages))
	for p := range stages {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	paths = paths[:min(len(paths), limit)]
	fps := make([]string, len(paths))
	var wg sync.WaitGroup
	slots := make(chan struct{}, 4)
	for i, p := range paths {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			fps[i] = r.fingerprintOf(ctx, stages[p])
		})
	}
	wg.Wait()
	named := map[string]string{}
	for i, p := range paths {
		named[p] = fps[i]
	}
	return named, nil
}

// fingerprintOf hashes the conflict between a file's index stages; "" without both sides, or
// for a binary file, which merge-file won't merge.
func (r *Repo) fingerprintOf(ctx context.Context, s [4]string) string {
	if s[2] == "" || s[3] == "" {
		return ""
	}
	base := s[1]
	if base == "" {
		// Added on both sides: merged from nothing.
		var err error
		if base, err = r.emptyBlob(ctx); err != nil {
			return ""
		}
	}
	// merge-file exits with the number of conflicts: its output is what counts. A binary file
	// is refused with an exit code of its own.
	merged, err := r.runner.Run(ctx, Cmd{Dir: r.dir, Args: []string{"merge-file", "-p", "--object-id", s[2], base, s[3]}, Env: noLazyFetch})
	var gerr *Error
	if err != nil && (!errors.As(err, &gerr) || gerr.ExitCode < 0 || gerr.ExitCode > 127) {
		return ""
	}
	return hunksHash(string(merged))
}

// hunksHash hashes the conflict hunks of a merged file with markers, each with its two sides in
// order; "" when there are none.
func hunksHash(merged string) string {
	h := sha256.New()
	found := false
	var ours, theirs strings.Builder
	part := 0 // 0 outside a hunk, 1 in ours, 2 in base, 3 in theirs
	for line := range strings.SplitSeq(merged, "\n") {
		switch {
		case strings.HasPrefix(line, "<<<<<<<"):
			part = 1
			ours.Reset()
			theirs.Reset()
		case part == 1 && strings.HasPrefix(line, "|||||||"):
			part = 2
		case part > 0 && strings.HasPrefix(line, "======="):
			part = 3
		case part == 3 && strings.HasPrefix(line, ">>>>>>>"):
			a, b := ours.String(), theirs.String()
			if b < a {
				a, b = b, a
			}
			h.Write([]byte(a + "\x00" + b + "\x00"))
			found = true
			part = 0
		case part == 1:
			ours.WriteString(strings.TrimRight(line, "\r") + "\n")
		case part == 3:
			theirs.WriteString(strings.TrimRight(line, "\r") + "\n")
		}
	}
	if !found {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// emptyBlob is the id of the empty file in this repository's hash.
func (r *Repo) emptyBlob(ctx context.Context) (string, error) {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"hash-object", "-t", "blob", "--stdin"}, Stdin: strings.NewReader("")})
	return strings.TrimSpace(string(out)), err
}
