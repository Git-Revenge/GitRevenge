package git

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Remote is a configured remote repository.
type Remote struct {
	Name     string `json:"name"`
	FetchURL string `json:"fetchUrl"`
	PushURL  string `json:"pushUrl"`
}

// Remotes lists configured remotes.
func (r *Repo) Remotes(ctx context.Context) ([]Remote, error) {
	out, err := r.ReadString(ctx, "remote", "-v")
	if err != nil {
		return nil, err
	}
	byName := map[string]*Remote{}
	var order []string
	for line := range strings.SplitSeq(out, "\n") {
		// "origin\thttps://example.com/x.git (fetch)"
		name, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		url, kind, _ := strings.Cut(rest, " ")
		rm, seen := byName[name]
		if !seen {
			rm = &Remote{Name: name}
			byName[name] = rm
			order = append(order, name)
		}
		if kind == "(push)" {
			rm.PushURL = url
		} else {
			rm.FetchURL = url
		}
	}
	remotes := make([]Remote, 0, len(order))
	for _, n := range order {
		remotes = append(remotes, *byName[n])
	}
	return remotes, nil
}

// checkRemoteName refuses names git could misread as options in later commands (such as
// "git fetch -x") or that can't form remote-tracking ref names.
func (r *Repo) checkRemoteName(ctx context.Context, name string) error {
	if name == "" || strings.HasPrefix(name, "-") {
		return fmt.Errorf("%q: %w", name, ErrInvalidName)
	}
	if _, err := r.Read(ctx, "check-ref-format", "refs/remotes/"+name+"/x"); err != nil {
		return fmt.Errorf("%q: %w", name, ErrInvalidName)
	}
	return nil
}

// AddRemote adds a remote.
func (r *Repo) AddRemote(ctx context.Context, name, url string) error {
	if err := r.checkRemoteName(ctx, name); err != nil {
		return err
	}
	_, err := r.Write(ctx, Cmd{Args: []string{"remote", "add", "--", name, url}})
	return err
}

// RemoveRemote deletes a remote and its remote-tracking branches.
func (r *Repo) RemoveRemote(ctx context.Context, name string) error {
	_, err := r.Write(ctx, Cmd{Args: []string{"remote", "remove", name}})
	return err
}

// RenameRemote renames a remote, updating its remote-tracking branches and upstreams.
func (r *Repo) RenameRemote(ctx context.Context, from, to string) error {
	if err := r.checkRemoteName(ctx, to); err != nil {
		return err
	}
	_, err := r.Write(ctx, Cmd{Args: []string{"remote", "rename", from, to}})
	return err
}

// SetRemoteURL changes a remote's URL (fetch and push).
func (r *Repo) SetRemoteURL(ctx context.Context, name, url string) error {
	_, err := r.Write(ctx, Cmd{Args: []string{"remote", "set-url", "--", name, url}})
	return err
}

// Progress is one update from a network operation.
type Progress struct {
	// Phase is git's label, e.g. "Receiving objects"; Line is the raw text.
	Phase   string `json:"phase"`
	Percent int    `json:"percent"`
	Line    string `json:"line"`
}

var progressRE = regexp.MustCompile(`^(?:remote: )?([A-Za-z ]+):\s+(\d+)%`)

// ParseProgress extracts a phase and percentage from a git progress line, if it has one.
func ParseProgress(line string) Progress {
	p := Progress{Line: line, Percent: -1}
	if m := progressRE.FindStringSubmatch(line); m != nil {
		p.Phase = strings.TrimSpace(m[1])
		p.Percent, _ = strconv.Atoi(m[2])
	}
	return p
}

// NetOptions configures a network command.
type NetOptions struct {
	// Env carries the askpass bridge settings for credential prompts.
	Env []string
	// OnProgress receives progress updates.
	OnProgress func(Progress)
}

func (o NetOptions) cmd(args ...string) Cmd {
	c := Cmd{Args: args, Env: o.Env}
	if o.OnProgress != nil {
		c.Progress = func(line string) { o.OnProgress(ParseProgress(line)) }
	}
	return c
}

// Fetch downloads from remote (or every remote when remote is empty), pruning
// remote-tracking branches that no longer exist. It only updates refs/remotes and tags,
// so it runs alongside other operations rather than waiting in the write queue.
func (r *Repo) Fetch(ctx context.Context, remote string, o NetOptions) error {
	args := []string{"fetch", "--progress", "--prune"}
	if remote == "" {
		args = append(args, "--all")
	} else {
		args = append(args, remote)
	}
	c := o.cmd(args...)
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}

// PullMode chooses how pull integrates upstream changes.
type PullMode string

// Pull modes. PullDefault follows the user's pull.rebase / pull.ff config.
const (
	PullDefault PullMode = ""
	PullFFOnly  PullMode = "ff-only"
	PullMerge   PullMode = "merge"
	PullRebase  PullMode = "rebase"
)

// Pull fetches and integrates the current branch's upstream.
func (r *Repo) Pull(ctx context.Context, mode PullMode, o NetOptions) error {
	// --prune, as Fetch does, so a deleted upstream shows as gone straight away.
	args := []string{"pull", "--progress", "--prune"}
	switch mode {
	case PullFFOnly:
		args = append(args, "--ff-only")
	case PullMerge:
		args = append(args, "--no-rebase")
	case PullRebase:
		args = append(args, "--rebase")
	}
	c := o.cmd(args...)
	c.Env = append(c.Env, "GIT_EDITOR=true") // merge commits keep git's message
	_, err := r.Write(ctx, c)
	return err
}

// Why PullBranch refused. In each case the branch is left where it was.
var (
	// ErrNoUpstream means the branch doesn't track anything.
	ErrNoUpstream = errors.New("branch has no upstream")
	// ErrUpstreamGone means the remote no longer has the branch being tracked.
	ErrUpstreamGone = errors.New("upstream no longer exists")
	// ErrBranchDiverged means both sides have commits, so a fast-forward can't work.
	ErrBranchDiverged = errors.New("branch has diverged from its upstream")
	// ErrBranchInUse means a worktree, rebase or bisect is using the branch.
	ErrBranchInUse = errors.New("branch is in use")
)

// ErrBranchMoved means a branch isn't at the commit MoveBranch expected.
var ErrBranchMoved = errors.New("branch moved")

// BranchUpstream returns the remote a branch tracks and the full name of the branch there.
func (r *Repo) BranchUpstream(ctx context.Context, branch string) (remote, merge string, err error) {
	remote, _ = r.ReadString(ctx, "config", "--get", "branch."+branch+".remote")
	merge, _ = r.ReadString(ctx, "config", "--get", "branch."+branch+".merge")
	if remote == "" || merge == "" {
		return "", "", ErrNoUpstream
	}
	return remote, merge, nil
}

// PullBranch fast-forwards a branch that isn't checked out from its upstream, returning where
// it was and where it is now. Only the ref moves, so the working tree and index aren't touched.
// A branch that only has commits of its own has nothing to pull, which isn't an error.
func (r *Repo) PullBranch(ctx context.Context, branch string, o NetOptions) (from, to string, err error) {
	remote, merge, err := r.BranchUpstream(ctx, branch)
	if err != nil {
		return "", "", err
	}
	ref := "refs/heads/" + branch
	if from, err = r.ReadString(ctx, "rev-parse", "--verify", "-q", ref); err != nil {
		return "", "", err
	}
	// No leading + in the refspec: git itself refuses anything but a fast-forward, or a branch
	// that a worktree, rebase or bisect is using, even if that started after any check here.
	_, err = r.Write(ctx, o.cmd("fetch", "--progress", remote, merge+":"+ref))
	var gerr *Error
	if errors.As(err, &gerr) {
		switch {
		case strings.Contains(gerr.Stderr, "refusing to fetch into"):
			return "", "", ErrBranchInUse
		case strings.Contains(gerr.Stderr, "couldn't find remote ref"):
			return "", "", ErrUpstreamGone
		case strings.Contains(gerr.Stderr, "[rejected]"), strings.Contains(gerr.Stderr, "non-fast-forward"):
			// git rejects anything that isn't forward, including a branch that's purely ahead.
			if _, aerr := r.Read(ctx, "merge-base", "--is-ancestor", branch+"@{upstream}", ref); aerr == nil {
				return from, from, nil
			}
			return "", "", ErrBranchDiverged
		}
	}
	if err != nil {
		return "", "", err
	}
	to, err = r.ReadString(ctx, "rev-parse", "--verify", "-q", ref)
	return from, to, err
}

// MoveBranch points a branch that isn't checked out at target, if it's still at expect.
func (r *Repo) MoveBranch(ctx context.Context, branch, expect, target string) error {
	tip, err := r.ReadString(ctx, "rev-parse", "--verify", "-q", "refs/heads/"+branch)
	if err != nil || tip != expect {
		return ErrBranchMoved
	}
	// git refuses to force a branch that a worktree has checked out.
	_, err = r.Write(ctx, Cmd{Args: []string{"branch", "-f", branch, target}})
	return err
}

// PushOptions selects what Push sends.
type PushOptions struct {
	Remote string
	// Refspec is what to push, e.g. "main", "refs/tags/v1.0", or ":branch" to delete.
	Refspec string
	// SetUpstream records the pushed branch as the local branch's upstream.
	SetUpstream bool
	// ForceWithLease overwrites the remote branch only if it is where we last saw it.
	ForceWithLease bool
}

// Push sends refs to a remote.
func (r *Repo) Push(ctx context.Context, p PushOptions, o NetOptions) error {
	args := []string{"push", "--progress"}
	if p.SetUpstream {
		args = append(args, "--set-upstream")
	}
	if p.ForceWithLease {
		args = append(args, "--force-with-lease")
	}
	args = append(args, "--", p.Remote, p.Refspec)
	c := o.cmd(args...)
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}

// PushBranches force pushes several branches to a remote in one step, all or none, each only if
// the remote has it where it was last seen, and records them as upstreams. A refspec is
// "branch:refs/heads/branch".
func (r *Repo) PushBranches(ctx context.Context, remote string, refspecs []string, o NetOptions) error {
	args := append([]string{"push", "--progress", "--atomic", "--force-with-lease", "--set-upstream", "--", remote}, refspecs...)
	c := o.cmd(args...)
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}

// IssueURL returns a pattern for links to a remote's issues, with "{n}" standing for the
// issue number, for remotes on GitHub or GitLab (including self-hosted GitLab, recognised
// by "gitlab" in the host name). It returns "" for anything else.
func IssueURL(remoteURL string) string {
	host, path := webLocation(remoteURL)
	if host == "" || path == "" {
		return ""
	}
	base := "https://" + host + "/" + path
	switch {
	case host == "github.com":
		return base + "/issues/{n}"
	case strings.Contains(host, "gitlab"):
		return base + "/-/issues/{n}"
	}
	return ""
}

// webLocation extracts the host and "owner/repo" path from an https, ssh or scp-style
// (git@host:owner/repo.git) remote URL.
func webLocation(remoteURL string) (host, path string) {
	u := strings.TrimSpace(remoteURL)
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		if at := strings.LastIndex(u, "@"); at >= 0 && at < strings.Index(u+"/", "/") {
			u = u[at+1:]
		}
		host, path, _ = strings.Cut(u, "/")
		host, _, _ = strings.Cut(host, ":") // drop a port
	} else {
		// scp-style: [user@]host:path
		if at := strings.Index(u, "@"); at >= 0 {
			u = u[at+1:]
		}
		var ok bool
		if host, path, ok = strings.Cut(u, ":"); !ok {
			return "", ""
		}
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	return strings.ToLower(host), path
}
