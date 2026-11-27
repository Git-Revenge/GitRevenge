package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// Applying a plain diff with git apply (#1024). A diff has no author or message, so it lands
// in the files, or the index, to look at, edit and commit as your own. It applies only onto
// files with no changes of yours, so taking it back is exact: they go back to HEAD.

// DiffInfo is a plain diff as the preview shows it.
type DiffInfo struct {
	Files []git.DiffFile `json:"files"`
	// Named says the diff names the versions it was made from (git diff's index lines), so a
	// change that no longer fits as it is can be merged. A diff from diff -u can't.
	Named bool `json:"named"`
	// Unclean lists the files it changes that have changes of yours: it waits for those.
	Unclean []string `json:"unclean"`
}

// readDiff reads a plain diff for the preview: a copy kept for the try and the apply, the
// files it changes, and which of them have changes of yours.
func (a *App) readDiff(r *session.Repo, out PatchFile, data []byte) (PatchFile, error) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "gitrevenge-apply-")
	if err != nil {
		return PatchFile{}, writeErr(err)
	}
	copied := filepath.Join(dir, "diff.patch")
	if err := os.WriteFile(copied, data, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return PatchFile{}, writeErr(err)
	}
	files, err := r.Git.ReadDiff(ctx, copied)
	if err != nil {
		_ = os.RemoveAll(dir)
		return PatchFile{}, patchError(err)
	}
	paths := diffPaths(files)
	unclean, err := r.Git.Unclean(ctx, paths)
	if err != nil {
		_ = os.RemoveAll(dir)
		return PatchFile{}, patchError(err)
	}
	if unclean == nil {
		unclean = []string{}
	}
	named := git.NamesVersions(data)
	out.Token = a.patches.add(&patchSet{repo: r.Path, dir: dir, diff: copied, files: files, named: named})
	out.Diff = &DiffInfo{Files: files, Named: named, Unclean: unclean}
	out.Branch = r.Git.Head(ctx).Branch
	if out.Missing, err = r.Git.MissingAt(ctx, "HEAD", paths); err != nil {
		return PatchFile{}, patchError(err)
	}
	return out, nil
}

// diffPaths are every path the files touch, both names of a rename.
func diffPaths(files []git.DiffFile) []string {
	var out []string
	for _, f := range files {
		for _, p := range f.Paths() {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// DiffTry is one file's row in the diff's preview.
type DiffTry struct {
	Path string `json:"path"`
	git.TryResult
}

// TryDiff tries each file of a diff on a throwaway copy of the index made from HEAD: as it
// is, else by merging when the diff names its versions. Reverse tries taking it back out. The
// files and the index stay as they are. A new try of the same diff stops the one before.
func (a *App) TryDiff(path, token string, reverse bool) ([]DiffTry, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	p := a.patches.get(token)
	if p == nil || p.repo != r.Path || p.diff == "" {
		return nil, userError("That diff isn't open any more. Choose it again.")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = cancel
	p.mu.Unlock()

	dir, err := os.MkdirTemp(p.dir, "try-")
	if err != nil {
		return nil, writeErr(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	trial, err := r.Git.StartTrial(ctx, dir)
	if err != nil {
		return nil, patchError(err)
	}
	out := make([]DiffTry, 0, len(p.files))
	for _, f := range p.files {
		if ctx.Err() != nil {
			return out, nil
		}
		out = append(out, DiffTry{Path: f.Path, TryResult: trial.TryDiffFile(ctx, p.diff, f.Paths(), reverse, p.named)})
	}
	return out, nil
}

// AppliedDiff says how applying a diff went.
type AppliedDiff struct {
	// ID names it for UndoDiff.
	ID string `json:"id"`
	// Files counts the files it changed.
	Files int `json:"files"`
	// Conflicts lists the files merging left in conflict, to resolve in Changes.
	Conflicts []string `json:"conflicts"`
}

// ApplyDiff applies the files of a diff picked, by their paths, in mode ("files", "index" or
// "cached"), or takes them back out when reverse is set: as they are, else by merging when
// the diff names its versions. The files must have no changes of yours.
func (a *App) ApplyDiff(path, token string, picked []string, mode string, reverse bool) (AppliedDiff, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return AppliedDiff{}, err
	}
	p := a.patches.get(token)
	if p == nil || p.repo != r.Path || p.diff == "" {
		return AppliedDiff{}, userError("That diff isn't open any more. Choose it again.")
	}
	m := git.DiffMode(mode)
	if m != git.DiffToFiles && m != git.DiffToFilesAndIndex && m != git.DiffToIndex {
		return AppliedDiff{}, userError("Apply the diff to the files, the files and the index, or the index only.")
	}
	var files []git.DiffFile
	for _, f := range p.files {
		if slices.Contains(picked, f.Path) {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return AppliedDiff{}, userError("Tick at least one file to apply.")
	}
	ctx := context.Background()
	paths := diffPaths(files)
	unclean, err := r.Git.Unclean(ctx, paths)
	if err != nil {
		return AppliedDiff{}, patchError(err)
	}
	if len(unclean) > 0 {
		return AppliedDiff{}, userError(filesSaid(unclean) + " " + hasHave(len(unclean)) + " changes of yours. Commit or stash them first, so the diff can be undone cleanly.")
	}
	a.undo.touch(r.Path)
	err = r.Git.ApplyDiff(ctx, p.diff, paths, m, reverse, p.named)
	out := AppliedDiff{Files: len(files), Conflicts: []string{}}
	if errors.Is(err, git.ErrConflicts) {
		if m == git.DiffToIndex {
			// A conflict needs the files: staged only can't hold one.
			_ = r.Git.PutBack(ctx, paths)
			return AppliedDiff{}, userError("Merging left a conflict, which staging only can't hold. Apply it to the files instead.")
		}
		st, _ := r.Git.Status(ctx)
		for _, f := range st.Files {
			if f.Conflicted && slices.Contains(paths, f.Path) {
				out.Conflicts = append(out.Conflicts, f.Path)
			}
		}
		err = nil
	}
	if err != nil {
		return AppliedDiff{}, patchError(err)
	}
	mark, err := r.Git.Fingerprint(ctx, paths)
	if err != nil {
		return out, nil
	}
	out.ID = a.applied.set(r.Path, appliedDiff{paths: paths, mark: mark})
	return out, nil
}

// UndoDiff takes back the diff applied as id: its files go back to HEAD, as they were before
// it, when nothing has touched them since.
func (a *App) UndoDiff(path, id string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	d, ok := a.applied.get(r.Path, id)
	if !ok {
		return userError("That diff can no longer be undone here.")
	}
	ctx := context.Background()
	now, err := r.Git.Fingerprint(ctx, d.paths)
	if err != nil {
		return patchError(err)
	}
	if now != d.mark {
		a.applied.drop(r.Path, id)
		return userError("Its files have changed since, so the diff can't be undone here.")
	}
	a.undo.touch(r.Path)
	if err := r.Git.PutBack(ctx, d.paths); err != nil {
		return patchError(err)
	}
	a.applied.drop(r.Path, id)
	return nil
}

// filesSaid names files: "a.txt", "a.txt and b.txt", "a.txt and 3 others".
func filesSaid(files []string) string {
	switch len(files) {
	case 1:
		return files[0]
	case 2:
		return files[0] + " and " + files[1]
	}
	return files[0] + " and " + strconv.Itoa(len(files)-1) + " others"
}

func hasHave(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// appliedDiff is a diff applied: the paths it touched, and their fingerprint just after.
type appliedDiff struct {
	id    string
	paths []string
	mark  string
}

// appliedDiffs holds each repository's last diff applied.
type appliedDiffs struct {
	mu   sync.Mutex
	last map[string]appliedDiff
	n    int
}

func (s *appliedDiffs) set(repo string, d appliedDiff) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		s.last = map[string]appliedDiff{}
	}
	s.n++
	d.id = "diff-" + strconv.Itoa(s.n)
	s.last[repo] = d
	return d.id
}

func (s *appliedDiffs) get(repo, id string) (appliedDiff, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.last[repo]
	return d, ok && d.id == id && id != ""
}

func (s *appliedDiffs) drop(repo, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.last[repo]; ok && d.id == id {
		delete(s.last, repo)
	}
}
