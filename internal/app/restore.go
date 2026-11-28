package app

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/semdiff"
)

// Restoring from the blame view's track (#1082): a version of the file, or a block's lines,
// written into the work tree as a change to commit or throw away. History is never edited,
// and the index is left alone. Undo puts the file back as it was, uncommitted changes and
// all, while nothing has touched it since.

// RestorePlan says what restoring a version would do, for the view to ask first.
type RestorePlan struct {
	// Added and Removed count the lines restoring adds and takes out, against the file as it
	// is in the work tree.
	Added   int `json:"added"`
	Removed int `json:"removed"`
	// Uncommitted counts the lines changed in the work tree that aren't committed (added and
	// removed against HEAD), which restoring replaces; 0 when there are none.
	Uncommitted int `json:"uncommitted"`
}

// Restored says a restore happened. ID names it for UndoRestore.
type Restored struct {
	ID string `json:"id"`
}

// RestorePlan works out what restoring file as it was at commit would change. source is its
// name at commit, "" for file: a version from before a rename is written under today's name.
func (a *App) RestorePlan(path, file, commit, source string) (RestorePlan, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return RestorePlan{}, err
	}
	ctx := context.Background()
	want, err := r.Git.VersionText(ctx, commit, source, file)
	if err != nil {
		return RestorePlan{}, restoreError(err)
	}
	now, _, err := r.Git.WorkFile(file)
	if err != nil {
		return RestorePlan{}, err
	}
	plan := RestorePlan{}
	if res, err := semdiff.Diff(ctx, string(now), string(want), semdiff.Options{}); err == nil {
		plan.Added, plan.Removed = res.Added, res.Removed
	}
	if dirty, _ := r.Git.Uncommitted(ctx, file); dirty {
		head, _ := r.Git.VersionText(ctx, "HEAD", "", file)
		if res, err := semdiff.Diff(ctx, string(head), string(now), semdiff.Options{}); err == nil {
			plan.Uncommitted = res.Added + res.Removed
		}
	}
	return plan, nil
}

// RestoreVersion writes file into the work tree as it was at commit (under the name source
// there, "" for file), in work-tree form. The index is left alone.
func (a *App) RestoreVersion(path, file, commit, source string) (Restored, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return Restored{}, err
	}
	want, err := r.Git.VersionText(context.Background(), commit, source, file)
	if err != nil {
		return Restored{}, restoreError(err)
	}
	return a.restore(r.Path, file, func([]byte) ([]byte, error) { return want, nil })
}

// RestoreBlock puts lines back into file in the work tree: lines from..to (counting from 1)
// of the file at commit at (named source there, "" for file), where they still are, become
// lines, as one of their versions had them. It says so when they've moved on since.
func (a *App) RestoreBlock(path, file, source, at string, from, to int, lines []string) (Restored, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return Restored{}, err
	}
	was, err := r.Git.VersionText(context.Background(), at, source, file)
	if err != nil {
		return Restored{}, restoreError(err)
	}
	return a.restore(r.Path, file, func(now []byte) ([]byte, error) {
		got, err := git.LinesRestored(now, was, from, to, lines)
		if errors.Is(err, git.ErrLinesMoved) {
			return nil, userError("Those lines have changed in the file since, so they can't be put back where they were.")
		}
		return got, err
	})
}

// restore writes what change makes of file's work-tree text, keeping what was there for
// UndoRestore.
func (a *App) restore(repo, file string, change func(now []byte) ([]byte, error)) (Restored, error) {
	r, err := a.openRepo(repo)
	if err != nil {
		return Restored{}, err
	}
	ctx := context.Background()
	before, existed, err := r.Git.WorkFile(file)
	if err != nil {
		return Restored{}, err
	}
	after, err := change(before)
	if err != nil {
		return Restored{}, err
	}
	a.undo.touch(r.Path)
	if err := r.Git.WriteWorkFile(ctx, file, after, true); err != nil {
		return Restored{}, err
	}
	mark, err := r.Git.Fingerprint(ctx, []string{file})
	if err != nil {
		return Restored{}, nil
	}
	return Restored{ID: a.restored.set(r.Path, restore{file: file, before: before, existed: existed, mark: mark})}, nil
}

// UndoRestore puts the file restored as id back as it was before, uncommitted changes and
// all, when nothing has touched it since.
func (a *App) UndoRestore(path, id string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	d, ok := a.restored.get(r.Path, id)
	if !ok {
		return userError("That restore can no longer be undone here.")
	}
	ctx := context.Background()
	now, err := r.Git.Fingerprint(ctx, []string{d.file})
	if err != nil {
		return err
	}
	if now != d.mark {
		a.restored.drop(r.Path, id)
		return userError(d.file + " has changed since, so the restore can't be undone here.")
	}
	a.undo.touch(r.Path)
	if err := r.Git.WriteWorkFile(ctx, d.file, d.before, d.existed); err != nil {
		return err
	}
	a.restored.drop(r.Path, id)
	return nil
}

// restoreError says why a version couldn't be read: most often, it isn't in this clone.
func restoreError(err error) error {
	var gerr *git.Error
	if errors.As(err, &gerr) {
		return userError("That version of the file couldn't be read from this clone.")
	}
	return err
}

// restore is one restore: the file, what it held before, and its fingerprint just after.
type restore struct {
	id      string
	file    string
	before  []byte
	existed bool
	mark    string
}

// restores holds each repository's last restore.
type restores struct {
	mu   sync.Mutex
	last map[string]restore
	n    int
}

func (s *restores) set(repo string, d restore) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		s.last = map[string]restore{}
	}
	s.n++
	d.id = "restore-" + strconv.Itoa(s.n)
	s.last[repo] = d
	return d.id
}

func (s *restores) get(repo, id string) (restore, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.last[repo]
	return d, ok && d.id == id && id != ""
}

func (s *restores) drop(repo, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.last[repo]; ok && d.id == id {
		delete(s.last, repo)
	}
}
