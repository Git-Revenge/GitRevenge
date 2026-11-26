package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// CloneOptions are what the clone dialog's Options choose. The zero value is a plain
// `git clone URL`: every commit, every file's contents, every branch and tag, no submodules.
type CloneOptions struct {
	// Depth, when above 0, fetches only that many commits of each branch's history: a
	// shallow clone (--depth).
	Depth int `json:"depth"`
	// Since, as Unix seconds, fetches only the commits made since then: a shallow clone
	// (--shallow-since). It's ignored when Depth is set.
	Since int64 `json:"since"`
	// Partial fetches every commit and folder listing now, and each file's contents the
	// first time something reads it (--filter=blob:none).
	Partial bool `json:"partial"`
	// Branch, when set, fetches only that branch, and checks it out (--single-branch).
	Branch string `json:"branch"`
	// Submodules clones the submodules too, shallow or partial as the clone is.
	Submodules bool `json:"submodules"`
	// Folders clones without writing any file (--no-checkout --sparse), so the folders to
	// check out can be chosen from the tree before CheckOutFolders writes them (#1004).
	Folders bool `json:"folders"`
}

// ErrCloneOptions means the clone's options can't be put on git's command line.
var ErrCloneOptions = errors.New("clone options")

// args are the options for git clone, checked: a negative depth, or a branch git would read
// as an option, is refused.
func (o CloneOptions) args() ([]string, error) {
	if o.Depth < 0 || o.Since < 0 {
		return nil, fmt.Errorf("%w: depth %d, since %d", ErrCloneOptions, o.Depth, o.Since)
	}
	if err := CheckRevs(o.Branch); err != nil {
		return nil, err
	}
	shallow := o.Depth > 0 || o.Since > 0
	var a []string
	switch {
	case o.Depth > 0:
		a = append(a, "--depth", strconv.Itoa(o.Depth))
	case o.Since > 0:
		a = append(a, "--shallow-since=@"+strconv.FormatInt(o.Since, 10))
	}
	if o.Partial {
		a = append(a, "--filter=blob:none")
	}
	switch {
	case o.Branch != "":
		a = append(a, "--single-branch", "--branch", o.Branch)
	case shallow:
		// A shallow clone fetches only the default branch unless told otherwise; the dialog
		// said every branch.
		a = append(a, "--no-single-branch")
	}
	if o.Submodules {
		a = append(a, "--recurse-submodules")
		if shallow {
			a = append(a, "--shallow-submodules")
		}
		if o.Partial {
			a = append(a, "--also-filter-submodules")
		}
	}
	if o.Folders {
		// The history and folder listings arrive and no file is written; --sparse starts the
		// working tree at the top level's own files, in cone mode.
		a = append(a, "--no-checkout", "--sparse")
	}
	return a, nil
}

// FolderCount is how many files a folder holds at a commit: all of them, at any depth, and
// those directly in it, which cone mode writes for a folder chosen below it.
type FolderCount struct {
	// Path is the folder, "" for the top level.
	Path   string `json:"path"`
	Files  int    `json:"files"`
	Direct int    `json:"direct"`
}

// FolderCounts counts the files in every folder at rev, from its listing alone: a partial
// clone answers without downloading anything, and nothing needs to be checked out. The top
// level comes first, then the folders in path order.
func (r *Repo) FolderCounts(ctx context.Context, rev string) ([]FolderCount, error) {
	if err := CheckRevs(rev); err != nil {
		return nil, err
	}
	if rev == "" {
		rev = "HEAD"
	}
	// The listing never needs a file's contents; told not to, a partial clone downloads none.
	out, err := r.readCmd(ctx, Cmd{Args: []string{"ls-tree", "-r", "-z", "--name-only", rev}, Env: noLazyFetch})
	if err != nil {
		return nil, err
	}
	files := map[string]int{"": 0}
	direct := map[string]int{"": 0}
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p == "" {
			continue
		}
		files[""]++
		dir, _ := splitDir(p)
		direct[dir]++
		for d := dir; d != ""; d, _ = splitDir(d) {
			files[d]++
		}
	}
	counts := make([]FolderCount, 0, len(files))
	for p, n := range files {
		counts = append(counts, FolderCount{Path: p, Files: n, Direct: direct[p]})
	}
	slices.SortFunc(counts, func(a, b FolderCount) int { return strings.Compare(a.Path, b.Path) })
	return counts, nil
}

// splitDir splits "a/b/c" into "a/b" and "c", and "c" into "" and "c".
func splitDir(p string) (dir, name string) {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i], p[i+1:]
	}
	return "", p
}

// CheckOutFolders writes the working tree of a clone made with Folders: only the folders
// named, in cone mode, or every folder when none are. Names arrive on standard input, so
// none can be read as an option.
func (r *Repo) CheckOutFolders(ctx context.Context, folders []string) error {
	if err := checkFolders(folders); err != nil {
		return err
	}
	if len(folders) == 0 {
		if _, err := r.Write(ctx, Cmd{Args: []string{"sparse-checkout", "disable"}}); err != nil {
			return err
		}
	} else {
		c := Cmd{Args: []string{"sparse-checkout", "set", "--cone", "--stdin"}, Stdin: strings.NewReader(strings.Join(folders, "\n") + "\n")}
		if _, err := r.Write(ctx, c); err != nil {
			return err
		}
	}
	// The clone wrote nothing, so the index is empty: checking out the branch writes the
	// files the patterns allow. A detached HEAD has no branch to name.
	if branch, err := r.ReadString(ctx, "symbolic-ref", "-q", "--short", "HEAD"); err == nil && branch != "" {
		if err := CheckRevs(branch); err != nil {
			return err
		}
		_, err := r.Write(ctx, Cmd{Args: []string{"checkout", "-q", branch, "--"}})
		return err
	}
	_, err := r.Write(ctx, Cmd{Args: []string{"read-tree", "-mu", "HEAD"}})
	return err
}

// checkFolders refuses a folder name that isn't a folder inside the repository.
func checkFolders(folders []string) error {
	for _, f := range folders {
		if f == "" || strings.HasPrefix(f, "/") || slices.Contains(strings.Split(f, "/"), "..") || strings.ContainsAny(f, "\r\n") {
			return fmt.Errorf("%w: folder %q", ErrCloneOptions, f)
		}
	}
	return nil
}

// Clone clones url into dir, which must not exist or be empty, with the options chosen. git
// creates dir and removes it again if the clone fails, but not if it is killed by
// cancellation.
func Clone(ctx context.Context, runner *Runner, url, dir string, opts CloneOptions, o NetOptions) error {
	extra, err := opts.args()
	if err != nil {
		return err
	}
	args := append([]string{"clone", "--progress"}, extra...)
	_, err = runner.Run(ctx, o.cmd(append(args, "--", url, dir)...))
	return err
}

// RemoteHeads are the branches a repository at a URL has, and the one a clone checks out.
type RemoteHeads struct {
	Branches []string `json:"branches"`
	// Default is the branch the server's HEAD names, or "" when it doesn't say.
	Default string `json:"default"`
}

// LsRemoteHeads asks the server at url which branches it has, as a clone would ask it.
func LsRemoteHeads(ctx context.Context, runner *Runner, url string, o NetOptions) (RemoteHeads, error) {
	out, err := runner.Run(ctx, o.cmd("ls-remote", "--symref", "--", url, "HEAD", "refs/heads/*"))
	if err != nil {
		return RemoteHeads{}, err
	}
	return parseRemoteHeads(out), nil
}

// parseRemoteHeads reads `git ls-remote --symref url HEAD refs/heads/*`: "ref: refs/heads/main
// <TAB>HEAD" names the default, and "<id><TAB>refs/heads/<name>" each branch.
func parseRemoteHeads(out []byte) RemoteHeads {
	h := RemoteHeads{Branches: []string{}}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		left, ref, ok := strings.Cut(sc.Text(), "\t")
		if !ok {
			continue
		}
		if target, ok := strings.CutPrefix(left, "ref: refs/heads/"); ok && ref == "HEAD" {
			h.Default = target
			continue
		}
		if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			h.Branches = append(h.Branches, name)
		}
	}
	return h
}

// Init creates an empty repository in dir, creating dir if needed. branch names the initial
// branch; empty uses the user's init.defaultBranch.
func Init(ctx context.Context, runner *Runner, dir, branch string) error {
	args := []string{"init", "-q"}
	if branch != "" {
		args = append(args, "--initial-branch="+branch)
	}
	_, err := runner.Run(ctx, Cmd{Args: append(args, "--", dir)})
	return err
}

// CloneName returns the folder name git clone would choose for url, e.g. "repo" for
// "git@host:owner/repo.git" or "https://host/owner/repo/". It returns "" if url has none.
func CloneName(url string) string {
	s := strings.TrimSpace(url)
	s = strings.TrimRight(s, `/\`)
	s = strings.TrimSuffix(s, "/.git")
	s = strings.TrimRight(s, `/\`)
	if i := strings.LastIndexAny(s, `/\:`); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	if s == "." || s == ".." {
		return ""
	}
	return s
}
