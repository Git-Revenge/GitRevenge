package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// What Tidy (#924) reads: the branches the last fetch of a remote brought, each remote
// branch's last commit, which are merged into a base, and how many commits only one branch
// holds.

// fetchHead returns FETCH_HEAD's content, or "" when there's none.
func (r *Repo) fetchHead(ctx context.Context) string {
	p, err := r.ReadString(ctx, "rev-parse", "--path-format=absolute", "--git-path", "FETCH_HEAD")
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.dir, p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(data)
}

// FetchHeadBranches returns the branches each URL sent in the fetch FETCH_HEAD records, from
// its "branch 'x' of <url>" lines. Notes and other refs aren't branches and aren't counted.
func FetchHeadBranches(content string) map[string][]string {
	out := map[string][]string{}
	for line := range strings.SplitSeq(content, "\n") {
		parts := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 3)
		if len(parts) < 3 {
			continue
		}
		what, url, ok := strings.Cut(parts[2], " of ")
		if !ok {
			continue
		}
		name, ok := strings.CutPrefix(what, "branch '")
		if !ok || !strings.HasSuffix(name, "'") {
			continue
		}
		out[url] = append(out[url], strings.TrimSuffix(name, "'"))
	}
	return out
}

// LastFetchedBranches returns the branches the last fetch brought from a remote whose URL is
// url, and whether FETCH_HEAD says anything about that remote at all. When it doesn't (the
// last fetch was of another remote), nothing can be said about which branches are gone.
func (r *Repo) LastFetchedBranches(ctx context.Context, url string) ([]string, bool) {
	for u, names := range FetchHeadBranches(r.fetchHead(ctx)) {
		if SameRemoteURL(u, url) {
			return names, true
		}
	}
	return nil, false
}

// RemoteBranchTip is a remote-tracking branch's last commit.
type RemoteBranchTip struct {
	// Short is "origin/feature".
	Short  string
	Commit string
	// Date is the commit's committer date, in Unix seconds.
	Date   int64
	Author string
}

// RemoteBranchTips returns every remote-tracking branch's last commit, without the remotes'
// HEADs, in one git call.
func (r *Repo) RemoteBranchTips(ctx context.Context) []RemoteBranchTip {
	out, err := r.Read(ctx, "for-each-ref", "--format=%(refname)%00%(symref)%00%(objectname)%00%(committerdate:unix)%00%(authorname)", "refs/remotes/")
	if err != nil {
		return nil
	}
	var tips []RemoteBranchTip
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) < 5 || f[1] != "" {
			continue
		}
		date, _ := strconv.ParseInt(f[3], 10, 64)
		tips = append(tips, RemoteBranchTip{Short: strings.TrimPrefix(f[0], "refs/remotes/"), Commit: f[2], Date: date, Author: f[4]})
	}
	return tips
}

// MergedRemoteBranches returns a remote's branches whose every commit is on base (a full ref
// name), as short names.
func (r *Repo) MergedRemoteBranches(ctx context.Context, remote, base string) []string {
	if CheckRevs(remote, base) != nil {
		return nil
	}
	out, err := r.Read(ctx, "for-each-ref", "--merged="+base, "--format=%(refname)%00%(symref)", "refs/remotes/"+remote+"/")
	if err != nil {
		return nil
	}
	var names []string
	for line := range strings.SplitSeq(string(out), "\n") {
		ref, symref, _ := strings.Cut(line, "\x00")
		if ref != "" && symref == "" {
			names = append(names, strings.TrimPrefix(ref, "refs/remotes/"))
		}
	}
	return names
}

// OnlyOn counts the commits of a remote-tracking branch (short name) that no other branch,
// remote branch or tag holds: the ones that would live only in the reflog once it's deleted.
func (r *Repo) OnlyOn(ctx context.Context, short string) int {
	if CheckRevs(short) != nil {
		return 0
	}
	out, err := r.ReadString(ctx, "rev-list", "--count", "refs/remotes/"+short, "--not", "--exclude="+short, "--remotes", "--branches", "--tags", "--")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(out)
	return n
}

// Unpushed counts a local branch's commits that aren't on any remote-tracking branch.
func (r *Repo) Unpushed(ctx context.Context, branch string) int {
	if CheckRevs(branch) != nil {
		return 0
	}
	out, err := r.ReadString(ctx, "rev-list", "--count", "refs/heads/"+branch, "--not", "--remotes", "--")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(out)
	return n
}

// GoneBranch is a local branch whose upstream the remote deleted.
type GoneBranch struct {
	Name     string
	Commit   string
	Upstream string
	// Busy means it's checked out, here or in a worktree.
	Busy bool
}

// GoneBranches returns the local branches whose upstream is gone, read from git, in one call.
func (r *Repo) GoneBranches(ctx context.Context) []GoneBranch {
	out, err := r.Read(ctx, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(upstream:short)%00%(upstream:track)%00%(HEAD)%00%(worktreepath)", "refs/heads/")
	if err != nil {
		return nil
	}
	var gone []GoneBranch
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) < 6 || f[2] == "" || f[3] != "[gone]" {
			continue
		}
		gone = append(gone, GoneBranch{Name: strings.TrimPrefix(f[0], "refs/heads/"), Commit: f[1], Upstream: f[2], Busy: f[4] == "*" || f[5] != ""})
	}
	return gone
}
