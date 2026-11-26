package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// A partial clone (#935, #1003) has every commit and folder listing, and fetches a file's
// contents the first time something reads it. Git does that one file at a time, behind
// whatever asked: fine for one file, and ruinous for thousands (blame on the kernel's
// Makefile made 1,888 downloads in 15 minutes). So reads here run with downloads off, say
// what's missing, and the app downloads it in one request when the rule says it may.

// noLazyFetch is the environment that stops git downloading what a partial clone lacks.
var noLazyFetch = []string{"GIT_NO_LAZY_FETCH=1"}

// NotDownloadedError says which objects a read needed that a partial clone hasn't got. It
// matches ErrNotDownloaded.
type NotDownloadedError struct {
	IDs []string
}

func (e *NotDownloadedError) Error() string {
	if len(e.IDs) == 1 {
		return "a version of the file was never downloaded to this partial clone"
	}
	return fmt.Sprintf("%d versions of files were never downloaded to this partial clone", len(e.IDs))
}

// Is makes errors.Is(err, ErrNotDownloaded) true.
func (e *NotDownloadedError) Is(target error) bool { return target == ErrNotDownloaded }

// NotDownloaded returns the objects err says weren't downloaded, if it says so.
func NotDownloaded(err error) ([]string, bool) {
	var nd *NotDownloadedError
	if errors.As(err, &nd) {
		return nd.IDs, true
	}
	return nil, false
}

// Partial reports whether the repository is a partial clone: some remote is a promisor, so
// file contents may be missing until something reads them.
func (r *Repo) Partial(ctx context.Context) bool {
	return r.promisorRemote(ctx) != ""
}

// promisorRemote names the remote a partial clone's missing objects come from, or "".
func (r *Repo) promisorRemote(ctx context.Context) string {
	out, err := r.ReadString(ctx, "config", "--type=bool", "--get-regexp", `^remote\..*\.promisor$`)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(out, "\n") {
		key, val, _ := strings.Cut(strings.TrimSpace(line), " ")
		if val == "true" {
			return strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".promisor")
		}
	}
	return ""
}

// ErrNotPartial means a download was asked of a repository with nothing to download from.
var ErrNotPartial = errors.New("not a partial clone")

// Download fetches objects by ID from the partial clone's remote in one request, as git's
// own lazy fetch does for one object at a time.
func (r *Repo) Download(ctx context.Context, ids []string, o NetOptions) error {
	if len(ids) == 0 {
		return nil
	}
	remote := r.promisorRemote(ctx)
	if remote == "" {
		return ErrNotPartial
	}
	if err := CheckRevs(remote); err != nil {
		return err
	}
	c := o.cmd("-c", "fetch.negotiationAlgorithm=noop", "fetch", "--progress", "--no-tags", "--no-write-fetch-head",
		"--recurse-submodules=no", "--filter=blob:none", "--stdin", remote)
	c.Dir = r.dir
	c.Stdin = strings.NewReader(strings.Join(ids, "\n") + "\n")
	_, err := r.runner.Run(ctx, c)
	return err
}

// FileVersions lists the object IDs of every version of path in rev's history: what blame
// reads. Merges count too, since a conflict's resolution makes a version of its own. Only
// commits and folder listings are read, so a partial clone answers without downloading.
func (r *Repo) FileVersions(ctx context.Context, path, rev string) ([]string, error) {
	if err := CheckRevs(rev); err != nil {
		return nil, err
	}
	if rev == "" {
		rev = "HEAD"
	}
	out, err := r.readCmd(ctx, Cmd{
		Args: []string{"--literal-pathspecs", "log", "-m", "--format=", "--raw", "--no-abbrev", "--no-renames", rev, "--", path},
		Env:  noLazyFetch,
	})
	if err != nil {
		return nil, err
	}
	var ids []string
	for line := range strings.SplitSeq(string(out), "\n") {
		// ":100644 100644 <old> <new> M\tpath"
		f := strings.Fields(line)
		if len(f) < 5 || !strings.HasPrefix(f[0], ":") {
			continue
		}
		for _, id := range f[2:4] {
			if strings.Trim(id, "0") != "" {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}
