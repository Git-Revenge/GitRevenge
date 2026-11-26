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

type noDownloadsKey struct{}

// NoDownloads returns a context under which every read leaves what a partial clone lacks
// undownloaded (#1010): a read that needs it fails, as git says, rather than starting a
// download per file behind a view. Views that read across many commits (forecasts, Insights,
// the Map, People) read under it and say what they couldn't read.
func NoDownloads(ctx context.Context) context.Context {
	return context.WithValue(ctx, noDownloadsKey{}, true)
}

// withDownloadsOff adds noLazyFetch to c's environment when ctx says so.
func withDownloadsOff(ctx context.Context, c Cmd) Cmd {
	if off, _ := ctx.Value(noDownloadsKey{}).(bool); off && !slices.Contains(c.Env, noLazyFetch[0]) {
		c.Env = append(slices.Clip(c.Env), noLazyFetch...)
	}
	return c
}

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

// MergeNeeds lists the file versions a merge or rebase forecast of ours with theirs would read
// that a partial clone hasn't got (#1010), so they come in one download rather than one at a
// time behind the preview. Only the paths both sides changed since their merge base need
// contents; for those it's their versions at the base, at each tip, and along ours's commits,
// which a rebase replays one by one. When one side deleted a path the other changed, git
// looks for where it went among that side's new files, so those are read too. A whole clone
// needs nothing.
func (r *Repo) MergeNeeds(ctx context.Context, ours, theirs string) ([]string, error) {
	if !r.Partial(ctx) {
		return nil, nil
	}
	if err := CheckRevs(ours, theirs); err != nil {
		return nil, err
	}
	base, err := r.ReadString(ctx, "merge-base", ours, theirs)
	if err != nil || base == "" {
		return nil, nil // unrelated histories: the forecast says so itself
	}
	mine, err := r.changedPaths(ctx, base, ours)
	if err != nil {
		return nil, err
	}
	other, err := r.changedPaths(ctx, base, theirs)
	if err != nil {
		return nil, err
	}
	both := map[string]bool{}
	for p := range mine {
		if _, ok := other[p]; ok {
			both[p] = true
		}
	}
	if len(both) == 0 {
		return nil, nil
	}
	// Where a side deleted a path the other changed, its added files are where git looks for
	// a rename.
	added := func(side, against map[string]string) map[string]bool {
		var gone bool
		for p, s := range side {
			if _, changed := against[p]; s == "D" && changed {
				gone = true
				break
			}
		}
		out := map[string]bool{}
		for p, s := range side {
			if gone && s == "A" {
				out[p] = true
			}
		}
		return out
	}
	oursPaths, theirsPaths := union(both, added(mine, other)), union(both, added(other, mine))
	var ids []string
	for _, side := range []struct {
		rev   string
		paths map[string]bool
	}{{base, both}, {ours, oursPaths}, {theirs, theirsPaths}} {
		found, err := r.blobsAt(ctx, side.rev, side.paths)
		if err != nil {
			return nil, err
		}
		ids = append(ids, found...)
	}
	along, err := r.versionsAlong(ctx, base+".."+ours, oursPaths)
	if err != nil {
		return nil, err
	}
	return r.missingOf(ctx, append(ids, along...))
}

func union(a, b map[string]bool) map[string]bool {
	out := make(map[string]bool, len(a)+len(b))
	for p := range a {
		out[p] = true
	}
	for p := range b {
		out[p] = true
	}
	return out
}

// ReplayNeeds lists what replaying base..head's commits in another order would read that a
// partial clone hasn't got: every version of every file they touch, since any two of them may
// meet once reordered.
func (r *Repo) ReplayNeeds(ctx context.Context, base, head string) ([]string, error) {
	if !r.Partial(ctx) {
		return nil, nil
	}
	if err := CheckRevs(base, head); err != nil {
		return nil, err
	}
	ids, err := r.versionsAlong(ctx, base+".."+head, nil)
	if err != nil {
		return nil, err
	}
	return r.missingOf(ctx, ids)
}

// changedPaths lists the paths that differ between two commits, from their listings, each
// with its status letter (A, D, M, T).
func (r *Repo) changedPaths(ctx context.Context, from, to string) (map[string]string, error) {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"diff-tree", "-r", "-z", "--name-status", "--no-renames", from, to}, Env: noLazyFetch})
	if err != nil {
		return nil, err
	}
	paths := map[string]string{}
	f := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	for i := 0; i+1 < len(f); i += 2 {
		paths[f[i+1]] = f[i]
	}
	return paths, nil
}

// blobsAt lists the IDs of paths' files at rev, from its listing (the whole tree, filtered
// here, so no path list has to fit on a command line).
func (r *Repo) blobsAt(ctx context.Context, rev string, paths map[string]bool) ([]string, error) {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"ls-tree", "-r", "-z", "--full-tree", rev}, Env: noLazyFetch})
	if err != nil {
		return nil, err
	}
	var ids []string
	// "mode type id" TAB path NUL.
	for entry := range strings.SplitSeq(string(out), "\x00") {
		head, path, ok := strings.Cut(entry, "\t")
		f := strings.Fields(head)
		if ok && len(f) == 3 && f[1] == "blob" && paths[path] {
			ids = append(ids, f[2])
		}
	}
	return ids, nil
}

// versionsAlong lists both sides of every change to paths (every path when nil) in a range's
// commits, merges too.
func (r *Repo) versionsAlong(ctx context.Context, revRange string, paths map[string]bool) ([]string, error) {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"log", "-m", "-z", "--format=", "--raw", "--no-abbrev", "--no-renames", revRange}, Env: noLazyFetch})
	if err != nil {
		return nil, err
	}
	var ids []string
	fields := strings.Split(string(out), "\x00")
	for i := 0; i+1 < len(fields); i++ {
		f := strings.Fields(fields[i])
		if len(f) < 5 || !strings.HasPrefix(f[0], ":") {
			continue
		}
		path := fields[i+1]
		i++
		if paths != nil && !paths[path] {
			continue
		}
		modes := []string{strings.TrimPrefix(f[0], ":"), f[1]}
		for k, id := range f[2:4] {
			if strings.Trim(id, "0") != "" && modes[k] != "160000" {
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}

// missingOf returns which of ids, once each, a partial clone hasn't got.
func (r *Repo) missingOf(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	slices.Sort(ids)
	return r.MissingObjects(ctx, slices.Compact(ids))
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
