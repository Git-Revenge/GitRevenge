package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
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
	return a, nil
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
