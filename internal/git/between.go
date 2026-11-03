package git

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

// CommitLine is a commit as a line in a list.
type CommitLine struct {
	ID      string `json:"id"`
	Short   string `json:"short"`
	Author  string `json:"author"`
	Subject string `json:"subject"`
	// Merge says it has more than one parent, so reverting it takes choosing which.
	Merge bool `json:"merge"`
}

// CommitsBetween lists up to limit commits reachable from include but not exclude, newest
// first, and how many there are in all.
func (r *Repo) CommitsBetween(ctx context.Context, exclude, include string, limit int) ([]CommitLine, int, error) {
	span := exclude + ".." + include
	out, err := r.ReadString(ctx, "rev-list", "--count", span)
	if err != nil {
		return nil, 0, err
	}
	total, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return nil, 0, err
	}
	out, err = r.ReadString(ctx, "log", "--max-count="+strconv.Itoa(limit), "--format=%H%x00%h%x00%an%x00%P%x00%s", span)
	if err != nil {
		return nil, 0, err
	}
	var commits []CommitLine
	for line := range strings.Lines(out) {
		f := strings.SplitN(strings.TrimRight(line, "\r\n"), "\x00", 5)
		if len(f) == 5 {
			commits = append(commits, CommitLine{ID: f[0], Short: f[1], Author: f[2], Merge: len(strings.Fields(f[3])) > 1, Subject: f[4]})
		}
	}
	return commits, total, nil
}

// UpstreamOf returns the short name of branch's upstream, or "" if it has none, or none that
// exists any more.
func (r *Repo) UpstreamOf(ctx context.Context, branch string) string {
	out, err := r.ReadString(ctx, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// reflogDepth is how far back WasOnBranch looks.
const reflogDepth = "2000"

// WasOnBranch reports whether the ref points at commit, or has pointed at it recently enough
// for its reflog to remember.
func (r *Repo) WasOnBranch(ctx context.Context, branch, commit string) bool {
	out, err := r.ReadString(ctx, "log", "-g", "-n", reflogDepth, "--format=%H", "refs/heads/"+branch)
	if err != nil {
		return false
	}
	return slices.Contains(strings.Fields(out), commit)
}
