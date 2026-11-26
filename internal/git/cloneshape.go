package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// A clone can hold less than the whole repository (#1005): part of its history (shallow),
// files only when they're needed (partial), some folders (sparse) or one branch. Whoever
// made it, the app or a terminal, these read how a clone differs and fetch the rest.

// CloneShape is how a clone differs from a whole one. Each part is nil when the clone is
// whole in that way.
type CloneShape struct {
	Shallow   *ShallowShape   `json:"shallow,omitempty"`
	Partial   *PartialShape   `json:"partial,omitempty"`
	Sparse    *SparseShape    `json:"sparse,omitempty"`
	OneBranch *OneBranchShape `json:"oneBranch,omitempty"`
}

// ShallowShape is a clone with part of its history.
type ShallowShape struct {
	// Since is the oldest commit date at the history's edge, Unix seconds.
	Since int64 `json:"since"`
	// Commits counts the commits the clone has on its branches and tags.
	Commits int `json:"commits"`
}

// PartialShape is a clone that downloads file contents when they're needed.
type PartialShape struct {
	Remote string `json:"remote"`
	// Filter is git's filter spec, such as "blob:none".
	Filter string `json:"filter"`
}

// SparseShape is a clone with only some of its files written.
type SparseShape struct {
	// Cone means Folders are folders; otherwise they're git's patterns.
	Cone    bool     `json:"cone"`
	Folders []string `json:"folders"`
	// Files counts the files written; Total, every file of the checked-out commit.
	Files int `json:"files"`
	Total int `json:"total"`
}

// OneBranchShape is a clone that fetches one branch from a remote.
type OneBranchShape struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
}

// Whole reports whether the clone is whole in every way.
func (s CloneShape) Whole() bool {
	return s.Shallow == nil && s.Partial == nil && s.Sparse == nil && s.OneBranch == nil
}

// CloneShape reads how the clone differs from a whole one. It reads commits, config and the
// index only: nothing is downloaded.
func (r *Repo) CloneShape(ctx context.Context) (CloneShape, error) {
	var s CloneShape
	var err error
	if s.Shallow, err = r.shallowShape(ctx); err != nil {
		return s, err
	}
	if remote := r.promisorRemote(ctx); remote != "" {
		filter, _ := r.ReadString(ctx, "config", "--get", "remote."+remote+".partialclonefilter")
		s.Partial = &PartialShape{Remote: remote, Filter: filter}
	}
	if s.Sparse, err = r.sparseShape(ctx); err != nil {
		return s, err
	}
	s.OneBranch = r.oneBranchShape(ctx)
	return s, nil
}

func (r *Repo) shallowShape(ctx context.Context) (*ShallowShape, error) {
	if out, _ := r.ReadString(ctx, "rev-parse", "--is-shallow-repository"); out != "true" {
		return nil, nil
	}
	file, err := r.ReadString(ctx, "rev-parse", "--git-path", "shallow")
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(r.dir, file)
	}
	edge, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	s := &ShallowShape{}
	// The edge's commits are the oldest the clone has: the history starts at the oldest.
	dates, err := r.readCmd(ctx, Cmd{Args: []string{"log", "--no-walk", "--format=%ct", "--stdin"}, Stdin: strings.NewReader(string(edge))})
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(string(dates), "\n") {
		if t, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64); err == nil && (s.Since == 0 || t < s.Since) {
			s.Since = t
		}
	}
	count, err := r.ReadString(ctx, "rev-list", "--count", "--branches", "--remotes", "--tags")
	if err != nil {
		return nil, err
	}
	s.Commits, _ = strconv.Atoi(count)
	return s, nil
}

func (r *Repo) sparseShape(ctx context.Context) (*SparseShape, error) {
	if on, _ := r.ReadString(ctx, "config", "--type=bool", "core.sparseCheckout"); on != "true" {
		return nil, nil
	}
	cone, _ := r.ReadString(ctx, "config", "--type=bool", "core.sparseCheckoutCone")
	s := &SparseShape{Cone: cone == "true", Folders: []string{}}
	list, err := r.ReadString(ctx, "sparse-checkout", "list")
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(list, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			s.Folders = append(s.Folders, line)
		}
	}
	// -t tags each entry: S for one the sparse checkout leaves unwritten.
	err = r.streamCmd(ctx, Cmd{Args: []string{"ls-files", "-t", "-z"}}, func(rd io.Reader) error {
		br := bufio.NewReaderSize(rd, 64*1024)
		for {
			entry, err := br.ReadString(0)
			if len(entry) > 2 {
				s.Total++
				if !strings.HasPrefix(entry, "S ") {
					s.Files++
				}
			}
			if err != nil {
				return nil
			}
		}
	})
	return s, err
}

// oneBranchRefspec is a remote's fetch refspec naming a single branch.
var oneBranchRefspec = regexp.MustCompile(`^\+?refs/heads/([^*:]+):refs/remotes/([^*:]+)/([^*:]+)$`)

// oneBranchShape finds a remote that fetches one branch, origin before the others.
func (r *Repo) oneBranchShape(ctx context.Context) *OneBranchShape {
	out, err := r.ReadString(ctx, "remote")
	if err != nil {
		return nil
	}
	remotes := strings.Fields(out)
	slices.SortStableFunc(remotes, func(a, b string) int {
		if a == "origin" || b == "origin" {
			return boolRank(b == "origin") - boolRank(a == "origin")
		}
		return 0
	})
	for _, remote := range remotes {
		specs, err := r.ReadString(ctx, "config", "--get-all", "remote."+remote+".fetch")
		if err != nil {
			continue
		}
		lines := strings.Fields(specs)
		if len(lines) != 1 {
			continue
		}
		if m := oneBranchRefspec.FindStringSubmatch(lines[0]); m != nil && m[2] == remote && m[1] == m[3] {
			return &OneBranchShape{Remote: remote, Branch: m[1]}
		}
	}
	return nil
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// MoreHistory says how far back FetchMoreHistory goes: Commits more, back to Since, or All
// of it.
type MoreHistory struct {
	Commits int   `json:"commits"`
	Since   int64 `json:"since"`
	All     bool  `json:"all"`
}

// ErrNotShallow is returned when there's no more history to fetch.
var ErrNotShallow = errors.New("the clone has all of its history")

// FetchMoreHistory deepens a shallow clone from remote.
func (r *Repo) FetchMoreHistory(ctx context.Context, remote string, more MoreHistory, o NetOptions) error {
	if err := CheckRevs(remote); err != nil {
		return err
	}
	if out, _ := r.ReadString(ctx, "rev-parse", "--is-shallow-repository"); out != "true" {
		return ErrNotShallow
	}
	args := []string{"fetch", "--progress"}
	switch {
	case more.All:
		args = append(args, "--unshallow")
	case more.Commits > 0:
		args = append(args, "--deepen="+strconv.Itoa(more.Commits))
	case more.Since > 0:
		args = append(args, "--shallow-since=@"+strconv.FormatInt(more.Since, 10)+" +0000")
	default:
		return fmt.Errorf("%w: how much more history", ErrCloneOptions)
	}
	c := o.cmd(append(args, remote)...)
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}

// Backfill downloads every version of every file in the checked-out branch's history that a
// partial clone hasn't got, in batches (git backfill). In a sparse checkout, only those in
// its folders.
func (r *Repo) Backfill(ctx context.Context, o NetOptions) error {
	if !r.Partial(ctx) {
		return ErrNotPartial
	}
	c := o.cmd("backfill")
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}

// BackfillCost is what Backfill would download, as far as the clone can tell.
type BackfillCost struct {
	// Size is the bytes the clone's objects take now.
	Size int64 `json:"size"`
	// Missing counts the file versions Backfill would download.
	Missing int `json:"missing"`
}

// BackfillCost counts the versions Backfill would download: those in the checked-out
// branch's history (in a sparse checkout, its folders') never downloaded. It walks the whole
// history's folder listings, which takes a while on a big repository.
func (r *Repo) BackfillCost(ctx context.Context) (BackfillCost, error) {
	var cost BackfillCost
	// Backfill keeps to a sparse checkout's folders. Patterns that aren't folders can't be
	// read here: then every version counts.
	var cone []string
	if s, err := r.sparseShape(ctx); err == nil && s != nil && s.Cone {
		cone = s.Folders
	}
	if out, err := r.ReadString(ctx, "count-objects", "-v"); err == nil {
		for line := range strings.SplitSeq(out, "\n") {
			key, val, _ := strings.Cut(line, ": ")
			if key == "size" || key == "size-pack" {
				kb, _ := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
				cost.Size += kb * 1024
			}
		}
	}
	// Downloads off: a missing folder listing (a tree:0 clone) is counted, not fetched.
	// print-info names each missing object's path: "?<id> path=<path> type=<type>".
	args := []string{"-c", "core.quotePath=false", "rev-list", "--objects", "--missing=print-info", "HEAD", "--"}
	err := r.streamCmd(ctx, Cmd{Args: args, Env: noLazyFetch}, func(rd io.Reader) error {
		sc := bufio.NewScanner(rd)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "?") {
				continue
			}
			if cone != nil {
				_, path, _ := strings.Cut(line, " path=")
				path, _, _ = strings.Cut(path, " type=")
				if path != "" && !inCone(path, cone) {
					continue
				}
			}
			cost.Missing++
		}
		return sc.Err()
	})
	return cost, err
}

// inCone reports whether a cone-mode sparse checkout of folders writes path: the top level's
// files, everything in the folders, and the files directly in the folders above them.
func inCone(path string, folders []string) bool {
	slash := strings.LastIndex(path, "/")
	if slash < 0 {
		return true
	}
	dir := path[:slash]
	for _, f := range folders {
		if strings.HasPrefix(path, f+"/") || strings.HasPrefix(f, dir+"/") {
			return true
		}
	}
	return false
}

// FetchEveryBranch has remote fetch every branch, not one, and fetches.
func (r *Repo) FetchEveryBranch(ctx context.Context, remote string, o NetOptions) error {
	if err := CheckRevs(remote); err != nil {
		return err
	}
	if _, err := r.Write(ctx, Cmd{Args: []string{"remote", "set-branches", remote, "*"}}); err != nil {
		return err
	}
	return r.Fetch(ctx, remote, o)
}

// SetSparseFolders writes the folders given (and the top level's files), or every folder
// when there are none. In a partial clone the files they add are downloaded, so it runs as a
// network command.
func (r *Repo) SetSparseFolders(ctx context.Context, folders []string, o NetOptions) error {
	if err := checkFolders(folders); err != nil {
		return err
	}
	var c Cmd
	if len(folders) == 0 {
		c = o.cmd("sparse-checkout", "disable")
	} else {
		c = o.cmd("sparse-checkout", "set", "--cone", "--stdin")
		c.Stdin = strings.NewReader(strings.Join(folders, "\n") + "\n")
	}
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}
