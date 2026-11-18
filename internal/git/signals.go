package git

import (
	"bufio"
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
)

// Branch signals and hotspots for the Insights view (ADR-0008). Like the shipped query they
// pin git's settings and measure ages from an as-of point rather than the clock.

// MaxSignalBranches caps how many branches get signals and conflict checks: the most
// recently active ones, ties broken by name.
const MaxSignalBranches = 50

// OpenBranch is a branch not yet merged into the target.
type OpenBranch struct {
	// Name is the short ref name, such as "origin/feature/x" or, for a local branch,
	// "feature/x".
	Name string `json:"name"`
	SHA  string `json:"sha"`
	// Local is true for a local branch with no upstream.
	Local bool `json:"local"`
	// Author is the tip's author, as the committed .mailmap resolves it.
	Author string `json:"author"`
	// LastCommit is the tip's committer date; FirstCommit the oldest author date among the
	// commits not on the target. Unix seconds.
	LastCommit  int64 `json:"lastCommit"`
	FirstCommit int64 `json:"firstCommit"`
	// Behind counts the target's commits since the merge base.
	Behind int `json:"behind"`
	// Added and Removed are lines changed against the merge base.
	Added   int `json:"added"`
	Removed int `json:"removed"`
	// Paths are the files the branch changes against the merge base, sorted.
	Paths []string `json:"paths"`
}

// OpenBranches lists remote branches, and local branches with no upstream, that aren't
// merged into target, most recently active first (ties by name), up to limit. skip, if not
// nil, leaves out branches by their name without the remote (such as Docs branches), before
// the limit applies.
func (r *Repo) OpenBranches(ctx context.Context, target string, limit int, skip func(name string) bool) ([]OpenBranch, error) {
	if err := CheckRevs(target); err != nil {
		return nil, err
	}
	targetRef, err := r.ReadString(ctx, "rev-parse", "--symbolic-full-name", target)
	if err != nil {
		return nil, err
	}
	out, err := r.ReadString(ctx, "for-each-ref", "--no-merged="+target,
		"--format=%(refname)%00%(refname:short)%00%(objectname)%00%(committerdate:unix)%00%(upstream)",
		"refs/remotes", "refs/heads")
	if err != nil {
		return nil, err
	}
	var rows [][]string
	// Remote branches by name without the remote ("feature/x"), with their tips.
	remote := map[string][]string{}
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.Split(line, "\x00")
		if len(f) != 5 || f[0] == targetRef || strings.HasSuffix(f[0], "/HEAD") {
			continue
		}
		rows = append(rows, f)
		if rest, ok := strings.CutPrefix(f[0], "refs/remotes/"); ok {
			if _, name, ok := strings.Cut(rest, "/"); ok {
				remote[name] = append(remote[name], f[2])
			}
		}
	}
	var bs []OpenBranch
	for _, f := range rows {
		local := strings.HasPrefix(f[0], "refs/heads/")
		// A local branch that tracks a remote, or that matches a remote branch of the same
		// name exactly, is the same work as that remote branch.
		if local && (f[4] != "" || slices.Contains(remote[f[1]], f[2])) {
			continue
		}
		if skip != nil {
			name := f[1]
			if !local {
				_, name, _ = strings.Cut(f[1], "/")
			}
			if skip(name) {
				continue
			}
		}
		at, _ := strconv.ParseInt(f[3], 10, 64)
		bs = append(bs, OpenBranch{Name: f[1], SHA: f[2], Local: local, LastCommit: at})
	}
	slices.SortFunc(bs, func(a, b OpenBranch) int {
		if a.LastCommit != b.LastCommit {
			return int(b.LastCommit - a.LastCommit)
		}
		return strings.Compare(a.Name, b.Name)
	})
	bs = bs[:min(len(bs), limit)]
	for i := range bs {
		if err := r.measure(ctx, target, &bs[i]); err != nil {
			return nil, err
		}
	}
	return bs, nil
}

// measure fills in a branch's author, age, drift and size.
func (r *Repo) measure(ctx context.Context, target string, b *OpenBranch) error {
	pin := pinned(target)
	read := func(args ...string) (string, error) { return r.ReadString(ctx, append(slices.Clone(pin), args...)...) }

	author, err := read("log", "-1", "--format=%aN", b.SHA, "--")
	if err != nil {
		return err
	}
	b.Author = author
	behind, err := read("rev-list", "--count", b.SHA+".."+target, "--")
	if err != nil {
		return err
	}
	b.Behind, _ = strconv.Atoi(behind)
	dates, err := read("log", "--format=%at", target+".."+b.SHA, "--")
	if err != nil {
		return err
	}
	for line := range strings.SplitSeq(dates, "\n") {
		if t, err := strconv.ParseInt(line, 10, 64); err == nil && (b.FirstCommit == 0 || t < b.FirstCommit) {
			b.FirstCommit = t
		}
	}
	base, err := read("merge-base", target, b.SHA)
	if err != nil {
		// Unrelated histories: nothing to compare against.
		return nil
	}
	diff, err := read("diff", "--numstat", "-M", "--no-textconv", "--no-ext-diff", base, b.SHA, "--")
	if err != nil {
		return err
	}
	b.Paths = []string{}
	for line := range strings.SplitSeq(diff, "\n") {
		cols := strings.SplitN(line, "\t", 3)
		if len(cols) != 3 {
			continue
		}
		if a, err := strconv.Atoi(cols[0]); err == nil {
			b.Added += a
		}
		if d, err := strconv.Atoi(cols[1]); err == nil {
			b.Removed += d
		}
		b.Paths = append(b.Paths, renamedTo(cols[2]))
	}
	slices.Sort(b.Paths)
	return nil
}

// renamedTo returns the new path from a numstat path, which -M writes as "old => new" or
// "dir/{old => new}/file" for a rename.
func renamedTo(p string) string {
	if open := strings.Index(p, "{"); open >= 0 {
		if end := strings.Index(p[open:], "}"); end > 0 {
			inner := p[open+1 : open+end]
			if _, to, ok := strings.Cut(inner, " => "); ok {
				return strings.ReplaceAll(p[:open]+to+p[open+end+1:], "//", "/")
			}
		}
	}
	if _, to, ok := strings.Cut(p, " => "); ok {
		return to
	}
	return p
}

// Conflict is the result of a dry-run merge of a branch into the target.
type Conflict struct {
	Conflicts bool `json:"conflicts"`
	// Files are the conflicting paths, sorted.
	Files []string `json:"files"`
}

// WouldConflict merges branch into target in memory (git merge-tree), touching nothing on
// disk, and reports whether it conflicts and where.
func (r *Repo) WouldConflict(ctx context.Context, target, branch string) (Conflict, error) {
	if err := CheckRevs(target, branch); err != nil {
		return Conflict{}, err
	}
	args := append(pinned(target),
		"-c", "merge.renames=true", "-c", "merge.directoryRenames=conflict",
		"merge-tree", "--write-tree", "--name-only", "--no-messages", target, branch)
	out, err := r.ReadString(ctx, args...)
	var gerr *Error
	switch {
	case err == nil:
		return Conflict{Files: []string{}}, nil
	case errors.As(err, &gerr) && gerr.ExitCode == 1:
		lines := strings.Split(out, "\n")
		files := []string{}
		// The first line is the tree with conflict markers; the rest name the files.
		for _, l := range lines[min(1, len(lines)):] {
			if l != "" && !slices.Contains(files, l) {
				files = append(files, l)
			}
		}
		slices.Sort(files)
		return Conflict{Conflicts: true, Files: files}, nil
	}
	return Conflict{}, err
}

// HotFile is how much the shipped items in a window changed one file.
type HotFile struct {
	Path string `json:"path"`
	// Lines is lines added plus removed; Items how many shipped items touched the file.
	Lines int `json:"lines"`
	Items int `json:"items"`
}

// HotFiles totals the files changed by the shipped items of q, most changed first
// (ties by path). Renames count against the new path.
func (r *Repo) HotFiles(ctx context.Context, q ShippedQuery) ([]HotFile, error) {
	if err := CheckRevs(q.Target, q.Range); err != nil {
		return nil, err
	}
	args := append(pinned(q.Target),
		"log", "--first-parent", "--diff-merges=first-parent", "--numstat", "-M",
		"--no-textconv", "--no-ext-diff", "--format=%x1e%ct%x1d")
	if q.Range != "" {
		args = append(args, q.Range, "--")
	} else {
		args = append(args, q.Target, "--")
	}
	files := map[string]*HotFile{}
	err := r.Stream(ctx, func(rd io.Reader) error {
		br := bufio.NewReaderSize(rd, 64*1024)
		for {
			rec, err := br.ReadString('\x1e')
			if head, numstat, ok := strings.Cut(strings.TrimSuffix(rec, "\x1e"), "\x1d"); ok {
				at, _ := strconv.ParseInt(strings.TrimSpace(head), 10, 64)
				if q.Range == "" && at < q.Since {
					return errStopWalk
				}
				if q.Range != "" || at <= q.Until {
					for line := range strings.SplitSeq(numstat, "\n") {
						cols := strings.SplitN(line, "\t", 3)
						if len(cols) != 3 {
							continue
						}
						p := renamedTo(cols[2])
						fc := files[p]
						if fc == nil {
							fc = &HotFile{Path: p}
							files[p] = fc
						}
						fc.Items++
						a, _ := strconv.Atoi(cols[0])
						d, _ := strconv.Atoi(cols[1])
						fc.Lines += a + d
					}
				}
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}, args...)
	if err != nil && !errors.Is(err, errStopWalk) && !isEmptyRepoLogError(err) {
		return nil, err
	}
	out := make([]HotFile, 0, len(files))
	for _, f := range files {
		out = append(out, *f)
	}
	slices.SortFunc(out, func(a, b HotFile) int {
		if a.Lines != b.Lines {
			return b.Lines - a.Lines
		}
		return strings.Compare(a.Path, b.Path)
	})
	return out, nil
}
