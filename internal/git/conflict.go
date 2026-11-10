package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ConflictFile is everything needed to resolve one conflicted file.
type ConflictFile struct {
	Path string `json:"path"`
	// Base, Ours and Theirs are the index stages 1–3 (Exists=false when a side deleted or
	// never had the file). Text is in work-tree form.
	Base   Side `json:"base"`
	Ours   Side `json:"ours"`
	Theirs Side `json:"theirs"`
	// Merged is the file on disk, usually with conflict markers.
	Merged Side `json:"merged"`
	// Binary means the versions can't be merged as text; pick a side instead.
	Binary bool `json:"binary"`
	// Diff3 is the three versions merged again with the original of each conflicted place
	// kept between the markers, whatever merge.conflictStyle says. It is "" when a side is
	// missing or it couldn't be made; Merged is then all there is.
	Diff3 string `json:"diff3"`
}

// Conflict reads the stages of a conflicted file.
func (r *Repo) Conflict(ctx context.Context, path string) (ConflictFile, error) {
	cf := ConflictFile{Path: path}
	var bufs [4][]byte
	var err error
	for i, ref := range []FileRef{
		{Rev: ":1", Path: path},
		{Rev: ":2", Path: path},
		{Rev: ":3", Path: path},
		WorkTreeVersion(path),
	} {
		var s Side
		// readSide joins Rev and Path with ':' for non-index revs; stages are ":N:path".
		s, bufs[i], err = r.readSide(ctx, ref, MaxForcedBytes)
		if err != nil {
			return cf, err
		}
		switch i {
		case 0:
			cf.Base = s
		case 1:
			cf.Ours = s
		case 2:
			cf.Theirs = s
		case 3:
			cf.Merged = s
		}
	}
	for _, b := range bufs {
		if isBinary(b) {
			cf.Binary = true
		}
	}
	if !cf.Binary {
		cf.Base.Text, cf.Ours.Text, cf.Theirs.Text, cf.Merged.Text = string(bufs[0]), string(bufs[1]), string(bufs[2]), string(bufs[3])
		if cf.Base.Exists && cf.Ours.Exists && cf.Theirs.Exists {
			cf.Diff3 = r.mergeDiff3(ctx, bufs[1], bufs[0], bufs[2])
		}
	}
	return cf, nil
}

// mergeDiff3 merges three versions of a file as text, keeping the original of each conflicted
// place between the markers. It returns "" if that fails.
func (r *Repo) mergeDiff3(ctx context.Context, ours, base, theirs []byte) string {
	dir, err := os.MkdirTemp("", "gitrevenge-merge-")
	if err != nil {
		return ""
	}
	defer func() { _ = os.RemoveAll(dir) }()
	names := [3]string{"ours", "base", "theirs"}
	for i, b := range [3][]byte{ours, base, theirs} {
		if err := os.WriteFile(filepath.Join(dir, names[i]), b, 0o600); err != nil {
			return ""
		}
	}
	// merge-file exits with the number of conflicts, so a failure here is the usual case.
	out, err := r.runner.Run(ctx, Cmd{Dir: dir, Args: []string{"merge-file", "-p", "--diff3", "-L", "ours", "-L", "base", "-L", "theirs", "ours", "base", "theirs"}})
	var gerr *Error
	if err != nil && (!errors.As(err, &gerr) || gerr.ExitCode < 0 || gerr.ExitCode > 127) {
		return ""
	}
	return string(out)
}

// ResolveWith writes content as the resolved file and stages it, marking it resolved.
func (r *Repo) ResolveWith(ctx context.Context, path, content string) error {
	if err := r.SetWorkTreeContent(path, content); err != nil {
		return err
	}
	return r.Stage(ctx, []string{path})
}

// ConflictSide picks a whole side of a conflict.
type ConflictSide string

// Conflict sides.
const (
	Ours   ConflictSide = "ours"
	Theirs ConflictSide = "theirs"
)

// ErrNotConflicted means the file has no conflict stages.
var ErrNotConflicted = errors.New("file is not conflicted")

// TakeSide resolves a conflicted file with one side's version. If that side deleted the
// file, resolving means deleting it.
func (r *Repo) TakeSide(ctx context.Context, path string, side ConflictSide) error {
	stage := ":2:"
	if side == Theirs {
		stage = ":3:"
	} else if side != Ours {
		return fmt.Errorf("unknown side %q", side)
	}
	if _, err := r.Read(ctx, "ls-files", "-u", "--error-unmatch", "--", path); err != nil {
		return ErrNotConflicted
	}
	if _, err := r.Read(ctx, "cat-file", "-e", stage+path); err != nil {
		// That side has no version: the resolution is to delete the file.
		p, perr := r.inside(path)
		if perr != nil {
			return perr
		}
		if rerr := os.Remove(p); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			return rerr
		}
		_, err := r.Write(ctx, pathspecCmd([]string{path}, "rm", "--cached", "-q", "--ignore-unmatch"))
		return err
	}
	if _, err := r.Write(ctx, pathspecCmd([]string{path}, "checkout", "--"+string(side))); err != nil {
		return err
	}
	return r.Stage(ctx, []string{path})
}
