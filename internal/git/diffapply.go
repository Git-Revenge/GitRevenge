package git

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Applying a plain diff to the files with git apply (#1024). A diff has no author or message,
// so it lands in the working tree (or the index) for you to look at and commit.

// DiffMode is where git apply puts a diff's changes.
type DiffMode string

const (
	// DiffToFiles changes the files only, as git apply does by default.
	DiffToFiles DiffMode = "files"
	// DiffToFilesAndIndex changes the files and stages them: --index.
	DiffToFilesAndIndex DiffMode = "index"
	// DiffToIndex stages the changes and leaves the files as they are: --cached.
	DiffToIndex DiffMode = "cached"
)

// DiffFile is one file a diff changes.
type DiffFile struct {
	// Path is where the file ends up; OldPath is where a renamed one was.
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary"`
	// Created and Removed say the diff adds or deletes the file.
	Created bool `json:"created"`
	Removed bool `json:"removed"`
}

// Paths are the paths the file touches: both names of a rename.
func (f DiffFile) Paths() []string {
	if f.OldPath != "" {
		return []string{f.OldPath, f.Path}
	}
	return []string{f.Path}
}

// indexLine is git diff's "index 1a2b3c4..5d6e7f8" line: the versions a file's change came from.
var indexLine = regexp.MustCompile(`(?m)^index [0-9a-f]{7,}\.\.[0-9a-f]{7,}`)

// NamesVersions says the diff names the versions it was made from, as git diff's index lines
// do, so a change that no longer fits can be merged. A diff from plain diff -u doesn't.
func NamesVersions(text []byte) bool {
	return indexLine.Match(text)
}

// ReadDiff lists the files the diff at file changes, as git apply reads it: counts from
// --numstat, and new, deleted and renamed files from --summary.
func (r *Repo) ReadDiff(ctx context.Context, file string) ([]DiffFile, error) {
	stats, err := r.patchNumStat(ctx, file)
	if err != nil {
		return nil, err
	}
	out, err := r.readCmd(ctx, Cmd{Args: []string{"-c", "core.quotePath=false", "apply", "--summary", file}})
	if err != nil {
		return nil, err
	}
	created, removed := map[string]bool{}, map[string]bool{}
	renamed := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "create mode "):
			if _, path, ok := strings.Cut(strings.TrimPrefix(line, "create mode "), " "); ok {
				created[path] = true
			}
		case strings.HasPrefix(line, "delete mode "):
			if _, path, ok := strings.Cut(strings.TrimPrefix(line, "delete mode "), " "); ok {
				removed[path] = true
			}
		case strings.HasPrefix(line, "rename ") || strings.HasPrefix(line, "copy "):
			if from, to, ok := renameNames(line); ok {
				renamed[to] = from
			}
		}
	}
	files := make([]DiffFile, 0, len(stats))
	for _, s := range stats {
		files = append(files, DiffFile{Path: s.Path, OldPath: renamed[s.Path], Added: s.Added, Deleted: s.Deleted, Binary: s.Binary, Created: created[s.Path], Removed: removed[s.Path]})
	}
	return files, nil
}

// renameNames reads --summary's "rename src/{a => b}.js (90%)" or "rename a => b (100%)".
func renameNames(line string) (from, to string, ok bool) {
	_, rest, _ := strings.Cut(line, " ")
	if i := strings.LastIndex(rest, " ("); i >= 0 {
		rest = rest[:i]
	}
	if open := strings.Index(rest, "{"); open >= 0 {
		end := strings.Index(rest[open:], "}")
		if end < 0 {
			return "", "", false
		}
		inner := rest[open+1 : open+end]
		a, b, found := strings.Cut(inner, " => ")
		if !found {
			return "", "", false
		}
		prefix, suffix := rest[:open], rest[open+end+1:]
		join := func(mid string) string {
			return strings.ReplaceAll(prefix+mid+suffix, "//", "/")
		}
		return join(a), join(b), true
	}
	from, to, ok = strings.Cut(rest, " => ")
	return from, to, ok
}

// Unclean lists which of paths have changes of yours, staged or not, or are there untracked:
// a diff applies only onto clean files, so taking it back is exact.
func (r *Repo) Unclean(ctx context.Context, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	out, err := r.readCmd(ctx, Cmd{Args: append([]string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames", "--"}, literal(paths)...)})
	if err != nil {
		return nil, err
	}
	var dirty []string
	for entry := range strings.SplitSeq(string(out), "\x00") {
		if len(entry) > 3 && !slices.Contains(dirty, entry[3:]) {
			dirty = append(dirty, entry[3:])
		}
	}
	return dirty, nil
}

// literal makes paths pathspecs that match only themselves.
func literal(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = ":(literal)" + p
	}
	return out
}

// includeOf is git apply's --include for one path: a wildmatch pattern, so its glob
// characters are escaped.
func includeOf(path string) string {
	var b strings.Builder
	for _, c := range path {
		if strings.ContainsRune(`*?[]\`, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return "--include=" + b.String()
}

// includes is git apply's --include for each of paths. A file's paths are both names of a
// rename: taken back out, a rename's new name is its old one.
func includes(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = includeOf(p)
	}
	return out
}

// TryDiffFile tries one file of a diff, named by its paths (DiffFile.Paths), on the
// throwaway index: as it is, else by merging when merge is set (the diff names its versions).
// Reverse takes the diff back out instead. TryApplied says the other way round applies: the
// change is in already, or for reverse, isn't there to take out.
func (t *PatchTrial) TryDiffFile(ctx context.Context, file string, paths []string, reverse, merge bool) TryResult {
	apply := func(back bool, more ...string) error {
		args := append([]string{"apply", "--cached"}, includes(paths)...)
		if back {
			args = append(args, "-R")
		}
		_, err := t.r.readCmd(ctx, t.cmd(append(append(args, more...), file)...))
		return err
	}
	if apply(reverse) == nil {
		return TryResult{Status: TryApplies}
	}
	if apply(!reverse, "--check") == nil {
		return TryResult{Status: TryApplied}
	}
	if !merge {
		return TryResult{Status: TryFails, Error: firstLine(apply(reverse, "--check"))}
	}
	err := apply(reverse, "--3way")
	if err == nil {
		return TryResult{Status: TryMerges}
	}
	if out, uerr := t.r.readCmd(ctx, t.cmd(append([]string{"ls-files", "-u", "-z", "--"}, literal(paths)...)...)); uerr == nil && len(out) > 0 {
		var files []string
		for entry := range strings.SplitSeq(string(out), "\x00") {
			if _, path, ok := strings.Cut(entry, "\t"); ok && !slices.Contains(files, path) {
				files = append(files, path)
			}
		}
		return TryResult{Status: TryConflicts, Files: files}
	}
	return TryResult{Status: TryFails, Error: firstLine(err)}
}

// ApplyDiff applies the diff at file to the paths named, in mode: as it is, else, when merge
// is set, by merging (git apply --3way). Merging stages what it changes; in DiffToFiles mode
// that's taken back out of the index unless a conflict needs it there. A conflict leaves the
// files unmerged and returns ErrConflicts. Reverse takes the diff back out instead.
func (r *Repo) ApplyDiff(ctx context.Context, file string, paths []string, mode DiffMode, reverse, merge bool) error {
	args := []string{"apply"}
	switch mode {
	case DiffToFilesAndIndex:
		args = append(args, "--index")
	case DiffToIndex:
		args = append(args, "--cached")
	}
	if reverse {
		args = append(args, "-R")
	}
	args = append(args, includes(paths)...)
	if _, err := r.Write(ctx, Cmd{Args: append(slices.Clone(args), file), Env: noLazyFetch}); err == nil || !merge {
		return err
	}
	err := r.conflictAware(ctx, Cmd{Args: append(args, "--3way", file), Env: noLazyFetch})
	if err != nil {
		return err
	}
	if unmerged, _ := r.readCmd(ctx, Cmd{Args: append([]string{"ls-files", "-u", "-z", "--"}, literal(paths)...)}); len(unmerged) > 0 {
		return ErrConflicts
	}
	if mode == DiffToFiles {
		// --3way staged it; git's default leaves the index alone.
		_, err = r.Write(ctx, Cmd{Args: append([]string{"reset", "-q", "--"}, literal(paths)...)})
	}
	return err
}

// Fingerprint reads where paths stand now: HEAD, each one's staged entry and its file's
// contents. Two fingerprints that match say nothing has touched them in between.
func (r *Repo) Fingerprint(ctx context.Context, paths []string) (string, error) {
	head, _ := r.ReadString(ctx, "rev-parse", "-q", "--verify", "HEAD")
	staged, err := r.readCmd(ctx, Cmd{Args: append([]string{"ls-files", "-s", "-z", "--"}, literal(paths)...)})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(head + "\x00" + string(staged))
	for _, p := range paths {
		info, err := os.Lstat(filepath.Join(r.dir, filepath.FromSlash(p)))
		if err != nil {
			b.WriteString("\x00-" + p)
			continue
		}
		b.WriteString("\x00" + p + ":" + info.Mode().String())
		if info.Mode().IsRegular() {
			id, err := r.ReadString(ctx, "hash-object", "--no-filters", "--", p)
			if err != nil {
				return "", err
			}
			b.WriteString(":" + id)
		}
	}
	return b.String(), nil
}

// PutBack puts paths back as HEAD has them, in the index and the files, and removes the ones
// HEAD hasn't got: undoing a diff applied onto clean files, conflicts and all.
func (r *Repo) PutBack(ctx context.Context, paths []string) error {
	var inHead, notInHead []string
	for _, p := range paths {
		if _, err := r.readCmd(ctx, Cmd{Args: []string{"cat-file", "-e", "HEAD:" + p}, Env: noLazyFetch}); err == nil {
			inHead = append(inHead, p)
		} else {
			notInHead = append(notInHead, p)
		}
	}
	if len(inHead) > 0 {
		if _, err := r.Write(ctx, Cmd{Args: append([]string{"checkout", "-q", "HEAD", "--"}, literal(inHead)...)}); err != nil {
			return err
		}
	}
	if len(notInHead) > 0 {
		if _, err := r.Write(ctx, Cmd{Args: append([]string{"rm", "-q", "-f", "--cached", "--ignore-unmatch", "--"}, literal(notInHead)...)}); err != nil {
			return err
		}
		for _, p := range notInHead {
			if err := os.Remove(filepath.Join(r.dir, filepath.FromSlash(p))); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}
