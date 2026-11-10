package git

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
)

// ErrNoLFS means git-lfs is not installed.
var ErrNoLFS = errors.New("git-lfs is not installed")

// LFSPattern is a path pattern stored in LFS (from .gitattributes).
type LFSPattern struct {
	Pattern string `json:"pattern"`
	Source  string `json:"source"`
	// Lockable files are read-only until locked (lockable attribute).
	Lockable bool `json:"lockable"`
	Tracked  bool `json:"tracked"`
}

// LFSFile is a file stored in LFS.
type LFSFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	// Downloaded is false when only the pointer is present locally.
	Downloaded bool   `json:"downloaded"`
	OID        string `json:"oid"`
}

// LFSStatus summarizes LFS use in a repository.
type LFSStatus struct {
	Patterns []LFSPattern `json:"patterns"`
	Files    []LFSFile    `json:"files"`
}

func (r *Repo) lfsJSON(ctx context.Context, v any, args ...string) error {
	out, err := r.Read(ctx, append([]string{"lfs"}, args...)...)
	if err != nil {
		var gerr *Error
		if errors.As(err, &gerr) && gerr.MissingLFS() {
			return ErrNoLFS
		}
		return err
	}
	return json.Unmarshal(out, v)
}

// lfsFound remembers that git-lfs was found, so it's asked for only once. Not finding it isn't
// remembered: it may be installed while the app runs.
var lfsFound atomic.Bool

// lfsInstalled reports whether git can run git-lfs.
func (r *Repo) lfsInstalled(ctx context.Context) (bool, error) {
	if lfsFound.Load() {
		return true, nil
	}
	if _, err := r.Read(ctx, "lfs", "version"); err != nil {
		var gerr *Error
		if errors.As(err, &gerr) && gerr.MissingLFS() {
			return false, nil
		}
		return false, err
	}
	lfsFound.Store(true)
	return true, nil
}

// LFS reports tracked patterns and LFS files at HEAD. It runs no git-lfs command in a
// repository that tracks nothing in LFS.
func (r *Repo) LFS(ctx context.Context) (LFSStatus, error) {
	st := LFSStatus{Patterns: []LFSPattern{}, Files: []LFSFile{}}
	if ok, err := r.lfsInstalled(ctx); err != nil {
		return st, err
	} else if !ok {
		return st, ErrNoLFS
	}
	pats, err := r.lfsPatterns(ctx)
	if err != nil {
		return st, err
	}
	st.Patterns = append(st.Patterns, pats...)
	if len(st.Patterns) == 0 || !r.hasHead(ctx) {
		return st, nil
	}
	var files struct {
		Files []LFSFile `json:"files"`
	}
	if err := r.lfsJSON(ctx, &files, "ls-files", "--json"); err != nil {
		return st, err
	}
	if files.Files != nil {
		st.Files = files.Files
	}
	return st, nil
}

// LFSTrack starts storing files matching pattern in LFS (updates .gitattributes, which the
// user then commits). LFSUntrack stops it.
func (r *Repo) LFSTrack(ctx context.Context, pattern string, lockable bool) error {
	args := []string{"lfs", "track"}
	if lockable {
		args = append(args, "--lockable")
	}
	_, err := r.Write(ctx, Cmd{Args: append(args, "--", pattern)})
	return err
}

// LFSUntrack removes pattern from LFS tracking.
func (r *Repo) LFSUntrack(ctx context.Context, pattern string) error {
	_, err := r.Write(ctx, Cmd{Args: []string{"lfs", "untrack", "--", pattern}})
	return err
}

// LFSLock is a file locked on the LFS server.
type LFSLock struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Owner    string `json:"owner"`
	LockedAt string `json:"lockedAt"`
	// Ours is true when the current user holds the lock.
	Ours bool `json:"ours"`
}

// LFSLocks asks the LFS server for current locks. It needs the network and credentials.
func (r *Repo) LFSLocks(ctx context.Context, o NetOptions) ([]LFSLock, error) {
	c := o.cmd("lfs", "locks", "--verify", "--json")
	c.Dir = r.dir
	c.ReadOnly = true
	out, err := r.runner.Run(ctx, c)
	if err != nil {
		return nil, err
	}
	return parseLocks(out)
}

// parseLocks reads `git lfs locks --verify --json`: {"ours":[...],"theirs":[...]}.
func parseLocks(out []byte) ([]LFSLock, error) {
	type raw struct {
		ID       string `json:"id"`
		Path     string `json:"path"`
		LockedAt string `json:"locked_at"`
		Owner    struct {
			Name string `json:"name"`
		} `json:"owner"`
	}
	var v struct {
		Ours   []raw `json:"ours"`
		Theirs []raw `json:"theirs"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, err
	}
	locks := []LFSLock{}
	for _, l := range v.Ours {
		locks = append(locks, LFSLock{ID: l.ID, Path: l.Path, Owner: l.Owner.Name, LockedAt: l.LockedAt, Ours: true})
	}
	for _, l := range v.Theirs {
		locks = append(locks, LFSLock{ID: l.ID, Path: l.Path, Owner: l.Owner.Name, LockedAt: l.LockedAt})
	}
	return locks, nil
}

// LFSLock locks path on the server; LFSUnlock releases it (force releases another
// user's lock, where the server allows it).
func (r *Repo) LFSLock(ctx context.Context, path string, o NetOptions) error {
	c := o.cmd("lfs", "lock", "--", path)
	_, err := r.Write(ctx, c)
	return err
}

// LFSUnlock releases a lock on path.
func (r *Repo) LFSUnlock(ctx context.Context, path string, force bool, o NetOptions) error {
	args := []string{"lfs", "unlock"}
	if force {
		args = append(args, "--force")
	}
	c := o.cmd(append(args, "--", path)...)
	_, err := r.Write(ctx, c)
	return err
}
