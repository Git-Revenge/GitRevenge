package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Restoring a version from the blame view's track (#1082): the file, or some of its lines,
// as they were at a commit, written into the work tree as a change to commit or throw away.
// History is never edited, and the index is left alone.

// ErrLinesMoved means the lines to restore aren't in the work tree's file any more, as they
// were where they were chosen.
var ErrLinesMoved = errors.New("those lines are no longer in the file as they were")

// VersionText reads the file named source at commit in work-tree form, as path's attributes
// would check it out: its filters and line endings applied. source is path when "": a
// version from before a rename is read under its old name and written under today's.
func (r *Repo) VersionText(ctx context.Context, commit, source, path string) ([]byte, error) {
	if err := CheckRevs(commit); err != nil {
		return nil, err
	}
	if source == "" {
		source = path
	}
	blob, err := r.ReadString(ctx, "rev-parse", "--verify", "-q", commit+":"+source)
	if err != nil {
		return nil, err
	}
	return r.readCmd(ctx, Cmd{Args: []string{"cat-file", "--filters", "--path=" + path, strings.TrimSpace(blob)}, Env: noLazyFetch})
}

// WorkFile reads path in the work tree. exists is false when there's no such file.
func (r *Repo) WorkFile(path string) (data []byte, exists bool, err error) {
	data, err = os.ReadFile(r.workPath(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return data, err == nil, err
}

// WriteWorkFile writes data to path in the work tree, keeping the file's mode when it's
// there; exists false removes it. It takes the repository's write turn, as git commands that
// change files do.
func (r *Repo) WriteWorkFile(ctx context.Context, path string, data []byte, exists bool) error {
	select {
	case r.writes <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-r.writes }()
	p := r.workPath(path)
	if !exists {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(p); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, mode)
}

// workPath is path's place in the work tree. Paths come from git, relative and with slashes.
func (r *Repo) workPath(path string) string {
	return filepath.Join(r.dir, filepath.FromSlash(path))
}

// LinesRestored is the work tree's text with lines from..to (counting from 1) of was, the
// file at the version they were chosen in, replaced by lines, where those lines are now:
// the run of the work tree's lines that reads as they did, nearest where they were. Each new
// line ends as the lines it replaces did ("\r\n" or "\n"). ErrLinesMoved when they're not
// there any more.
func LinesRestored(work, was []byte, from, to int, lines []string) ([]byte, error) {
	old := splitKeep(was)
	if from < 1 || to < from || to > len(old) {
		return nil, errors.New("not a range of lines in that version")
	}
	block := old[from-1 : to]
	now := splitKeep(work)
	at := -1
	for p := 0; p+len(block) <= len(now); p++ {
		if sameLines(now[p:p+len(block)], block) && (at < 0 || abs(p-(from-1)) < abs(at-(from-1))) {
			at = p
		}
	}
	if at < 0 {
		return nil, ErrLinesMoved
	}
	last := now[at+len(block)-1]
	eol := "\n"
	if strings.HasSuffix(last, "\r\n") {
		eol = "\r\n"
	}
	var b bytes.Buffer
	for _, l := range now[:at] {
		b.WriteString(l)
	}
	for i, l := range lines {
		b.WriteString(l)
		// The last line ends as the last one replaced did, which may be the file's end.
		if i < len(lines)-1 || strings.HasSuffix(last, "\n") {
			b.WriteString(eol)
		}
	}
	for _, l := range now[at+len(block):] {
		b.WriteString(l)
	}
	return b.Bytes(), nil
}

// splitKeep splits text into lines, each keeping its line ending.
func splitKeep(text []byte) []string {
	var out []string
	for s := string(text); s != ""; {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
	return out
}

// sameLines compares lines without their endings.
func sameLines(a, b []string) bool {
	for i := range a {
		if strings.TrimRight(a[i], "\r\n") != strings.TrimRight(b[i], "\r\n") {
			return false
		}
	}
	return true
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
