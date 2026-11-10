package git

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// lfsPatterns lists the patterns stored in LFS, read from the repository's attributes files:
// every .gitattributes in the work tree, .git/info/attributes and the global attributes file.
//
// It reads them itself rather than asking `git lfs track`, because that command installs LFS's
// hooks in the repository whenever it runs, even just to list. Those hooks then run git-lfs on
// every commit, checkout and merge, which made a rebase in the app take seconds per commit
// (#604) in repositories that don't use LFS at all.
//
// It follows `git lfs track --json`: a pattern from a .gitattributes in a folder is prefixed
// with that folder, quotes are removed and other escapes are kept, and a macro that sets
// filter=lfs counts.
func (r *Repo) lfsPatterns(ctx context.Context) ([]LFSPattern, error) {
	out, err := r.Read(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ":(glob)**/.gitattributes")
	if err != nil {
		return nil, err
	}
	var rels []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" && !slices.Contains(rels, p) {
			rels = append(rels, p)
		}
	}
	// The top folder's first, then the rest by path.
	slices.SortFunc(rels, func(a, b string) int {
		if a == ".gitattributes" || b == ".gitattributes" {
			return boolCmp(b == ".gitattributes", a == ".gitattributes")
		}
		return strings.Compare(a, b)
	})

	var pats []LFSPattern
	add := func(abs, source, dir string) error {
		b, err := os.ReadFile(abs)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		pats = append(pats, parseLFSAttributes(string(b), source, dir)...)
		return nil
	}
	for _, rel := range rels {
		abs, err := r.inside(rel)
		if err != nil {
			continue
		}
		dir := path.Dir(rel)
		if dir == "." {
			dir = ""
		}
		if err := add(abs, rel, dir); err != nil {
			return nil, err
		}
	}
	if p, err := r.ReadString(ctx, "rev-parse", "--git-path", "info/attributes"); err == nil && p != "" {
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.dir, p)
		}
		if err := add(filepath.Clean(p), ".git/info/attributes", ""); err != nil {
			return nil, err
		}
	}
	if p, err := r.globalAttributesFile(ctx); err == nil {
		if err := add(p, p, ""); err != nil {
			return nil, err
		}
	}
	return pats, nil
}

// globalAttributesFile returns core.attributesFile, or git's default when it isn't set.
func (r *Repo) globalAttributesFile(ctx context.Context) (string, error) {
	if p, err := r.ReadString(ctx, "config", "--path", "--get", "core.attributesFile"); err == nil && p != "" {
		return p, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "git", "attributes"), nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = h
	}
	return filepath.Join(home, ".config", "git", "attributes"), nil
}

// parseLFSAttributes returns the patterns an attributes file stores in LFS. dir is the
// folder the file is in, relative to the top of the work tree ("" for the top or for files
// outside the work tree).
func parseLFSAttributes(text, source, dir string) []LFSPattern {
	macros := map[string]bool{}
	var pats []LFSPattern
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern, rest := splitAttrPattern(line)
		attrs := strings.Fields(rest)
		lfs := false
		for _, a := range attrs {
			if a == "filter=lfs" || macros[a] {
				lfs = true
			}
		}
		if name, ok := strings.CutPrefix(pattern, "[attr]"); ok {
			if lfs {
				macros[name] = true
			}
			continue
		}
		if !lfs || pattern == "" {
			continue
		}
		if dir != "" {
			pattern = dir + "/" + pattern
		}
		pats = append(pats, LFSPattern{
			Pattern:  pattern,
			Source:   source,
			Lockable: slices.Contains(attrs, "lockable"),
			Tracked:  true,
		})
	}
	return pats
}

// splitAttrPattern splits an attributes line into its pattern and the rest. A pattern may be
// quoted, as git quotes a name with spaces; the quotes and their escapes are removed.
func splitAttrPattern(line string) (pattern, rest string) {
	if !strings.HasPrefix(line, `"`) {
		i := strings.IndexAny(line, " \t")
		if i < 0 {
			return line, ""
		}
		return line[:i], line[i+1:]
	}
	var b strings.Builder
	for i := 1; i < len(line); i++ {
		switch c := line[i]; c {
		case '"':
			return b.String(), line[i+1:]
		case '\\':
			if i+1 < len(line) {
				i++
				switch e := line[i]; e {
				case 't':
					b.WriteByte('\t')
				case 'n':
					b.WriteByte('\n')
				default:
					b.WriteByte(e)
				}
			}
		default:
			b.WriteByte(c)
		}
	}
	// No closing quote: git takes the line as it is.
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return line, ""
	}
	return line[:i], line[i+1:]
}
