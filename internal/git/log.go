package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// LogCommit is one commit as read from `git log`.
type LogCommit struct {
	ID          string
	Parents     []string
	AuthorName  string
	AuthorEmail string
	// AuthorTime is a Unix timestamp in seconds.
	AuthorTime int64
	Subject    string
}

// LogOptions selects which history Log reads.
type LogOptions struct {
	// Revs are the starting points. Empty means every branch, remote-tracking branch and tag,
	// plus HEAD (so a detached HEAD is included).
	Revs []string
	// Extra revisions are added to the default starting points (used for stash commits).
	Extra []string
	// Exclude lists glob patterns of branches to leave out of the default starting points,
	// matched against the branch name without refs/heads/ or the remote (see Remotes).
	Exclude []string
	// Remotes are the remote names, so Exclude also applies to remote-tracking branches.
	Remotes []string
	// MaxCount limits the number of commits; 0 means no limit.
	MaxCount int
}

// logFormat separates fields with US (0x1f). Records are NUL-terminated via -z, and none of
// these fields can contain NUL.
const logFormat = "%H%x1f%P%x1f%an%x1f%ae%x1f%at%x1f%s"

// Log streams commits in date order, calling fn for each as git produces it. Returning an
// error from fn stops git and is returned from Log.
//
// Date order still lists every child before its parents, which the graph layout needs, but
// otherwise follows committer date, so parallel lines of history (an orphan gh-pages, a
// long-running branch) interleave as they happened. --topo-order kept each line together
// and drew them as stacked blocks (#275).
//
// The walk is incremental, and the first commits arrive quickly, when the
// repository has a commit-graph file (`git commit-graph write`, or fetch.writeCommitGraph).
// Without one, git must walk the whole history before printing anything.
func (r *Repo) Log(ctx context.Context, opts LogOptions, fn func(LogCommit) error) error {
	if err := CheckRevs(append(slices.Clone(opts.Revs), opts.Extra...)...); err != nil {
		return err
	}
	args := []string{"log", "--date-order", "-z", "--format=" + logFormat, "--no-show-signature"}
	if opts.MaxCount > 0 {
		args = append(args, "--max-count="+strconv.Itoa(opts.MaxCount))
	}
	args = append(args, r.logRevs(ctx, opts)...)

	err := r.Stream(ctx, func(rd io.Reader) error {
		br, ok := rd.(*bufio.Reader)
		if !ok {
			br = bufio.NewReaderSize(rd, 64*1024)
		}
		for {
			rec, err := br.ReadSlice(0)
			if errors.Is(err, bufio.ErrBufferFull) {
				// A record longer than the buffer (a very long subject). ReadSlice's result is
				// only valid until the next read, so copy it before reading the rest.
				head := append([]byte(nil), rec...)
				rest, err2 := br.ReadBytes(0)
				rec = slices.Concat(head, rest)
				err = err2
			}
			if len(rec) > 0 && rec[len(rec)-1] == 0 {
				rec = rec[:len(rec)-1]
			}
			if len(rec) > 0 {
				c, perr := parseLogRecord(rec)
				if perr != nil {
					return perr
				}
				if ferr := fn(c); ferr != nil {
					return ferr
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
	if isEmptyRepoLogError(err) {
		return nil
	}
	return err
}

// maxTagTips caps how many tags are passed to git log as separate starting points; past
// it, --tags is used instead (a command line has a length limit).
const maxTagTips = 500

// logRevs returns the starting points and the trailing "--" for a log over opts.
func (r *Repo) logRevs(ctx context.Context, opts LogOptions) []string {
	var args []string
	if len(opts.Revs) == 0 {
		// Each --exclude applies to the next --branches or --remotes only.
		for _, p := range opts.Exclude {
			args = append(args, "--exclude="+p)
		}
		args = append(args, "--branches")
		if len(opts.Exclude) > 0 {
			for _, rm := range opts.Remotes {
				// A remote's HEAD names one of its branches, which is listed anyway; leaving it
				// out keeps a hidden default branch hidden.
				args = append(args, "--exclude="+rm+"/HEAD")
				for _, p := range opts.Exclude {
					args = append(args, "--exclude="+rm+"/"+p)
				}
			}
		}
		args = append(args, "--remotes")
		// HEAD is only valid once it points at a commit (not in an empty repo or on an
		// orphan branch).
		if _, err := r.Read(ctx, "rev-parse", "--verify", "-q", "HEAD"); err == nil {
			args = append(args, "HEAD")
			args = append(args, r.tagTips(ctx)...)
		} else {
			args = append(args, "--tags")
		}
	} else {
		args = append(args, opts.Revs...)
	}
	args = append(args, opts.Extra...)
	// "--" stops revs from being read as paths.
	return append(args, "--")
}

// tagTips returns the tags to start a log from besides branches and HEAD: only those HEAD
// can't reach. The others are in the history anyway, and every extra starting point makes
// the date-order walk go down from it before printing anything. On the Linux kernel, with 953
// tags, passing --tags made the first 100 commits take a second instead of 90ms.
func (r *Repo) tagTips(ctx context.Context) []string {
	out, err := r.ReadString(ctx, "for-each-ref", "--no-merged=HEAD", "--format=%(refname)", "refs/tags")
	if err != nil {
		return []string{"--tags"}
	}
	tips := strings.Fields(out)
	if len(tips) > maxTagTips {
		return []string{"--tags"}
	}
	return tips
}

// CountCommits counts the commits Log would list for opts, ignoring MaxCount. It is cheap
// with a commit-graph file, and gives the graph a size before the history has loaded.
func (r *Repo) CountCommits(ctx context.Context, opts LogOptions) (int, error) {
	if err := CheckRevs(append(slices.Clone(opts.Revs), opts.Extra...)...); err != nil {
		return 0, err
	}
	out, err := r.ReadString(ctx, append([]string{"rev-list", "--count"}, r.logRevs(ctx, opts)...)...)
	if isEmptyRepoLogError(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

func parseLogRecord(rec []byte) (LogCommit, error) {
	// Trim the newline git may emit between records in some modes.
	rec = bytes.TrimLeft(rec, "\n")
	f := bytes.SplitN(rec, []byte{0x1f}, 6)
	if len(f) != 6 {
		return LogCommit{}, fmt.Errorf("unexpected log record %q", rec)
	}
	t, err := strconv.ParseInt(string(f[4]), 10, 64)
	if err != nil {
		return LogCommit{}, fmt.Errorf("bad author time in log record %q", rec)
	}
	var parents []string
	if len(f[1]) > 0 {
		parents = strings.Fields(string(f[1]))
	}
	return LogCommit{
		ID:          string(f[0]),
		Parents:     parents,
		AuthorName:  string(f[2]),
		AuthorEmail: string(f[3]),
		AuthorTime:  t,
		Subject:     string(f[5]),
	}, nil
}

// isEmptyRepoLogError reports whether err is git refusing to log a repository that has no
// commits yet, which is an empty history rather than a failure.
func isEmptyRepoLogError(err error) bool {
	var gerr *Error
	if !errors.As(err, &gerr) {
		return false
	}
	// With no refs at all, git falls back to HEAD and reports it as unborn.
	return strings.Contains(gerr.Stderr, "does not have any commits yet") ||
		strings.Contains(gerr.Stderr, "bad revision 'HEAD'")
}
