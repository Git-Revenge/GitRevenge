package git

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
)

// BlockVersion is one version of a block of lines (#1081): a commit that changed them, as
// git log -L lists it, with what it did to them.
type BlockVersion struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Time    int64  `json:"time"`
	Subject string `json:"subject"`
	// From and To are where the lines are in this commit's version, counting from 1. To is
	// From-1 when the commit left none of them.
	From int `json:"from"`
	To   int `json:"to"`
	// Lines are the commit's change to them, in order.
	Lines []BlockLine `json:"lines"`
}

// BlockLine is a line of a block's change: Kind " " for one it kept, "+" added, "-" removed.
type BlockLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// MaxBlockVersions caps how many versions BlockHistory returns: the newest ones.
const MaxBlockVersions = 500

// BlockHistory lists the commits that changed lines from to to (counting from 1, both
// included) of path at rev, newest first, following the lines as they move down the file:
// git log -L. On the Linux kernel's mm/slub.c, a 40-line block's last 20 versions took 1.6
// s. It downloads nothing.
func (r *Repo) BlockHistory(ctx context.Context, path, rev string, from, to int) ([]BlockVersion, error) {
	if err := CheckRevs(rev); err != nil {
		return nil, err
	}
	if rev == "" {
		return nil, errors.New("BlockHistory needs a commit")
	}
	if from < 1 || to < from {
		return nil, errors.New("not a range of lines")
	}
	out, err := r.readCmd(ctx, Cmd{
		Args: []string{
			"log", "--no-color", "--max-count=" + strconv.Itoa(MaxBlockVersions),
			"--format=%x1e%H%x1f%aN%x1f%at%x1f%s",
			"-L", strconv.Itoa(from) + "," + strconv.Itoa(to) + ":" + path,
			rev, "--",
		},
		Env: noLazyFetch,
	})
	if err != nil {
		return nil, err
	}
	return parseBlockHistory(out), nil
}

// parseBlockHistory reads records of: RS sha US author US time US subject, then the diff git
// log -L shows for the lines, with a hunk header saying where they are.
func parseBlockHistory(out []byte) []BlockVersion {
	versions := []BlockVersion{}
	for rec := range bytes.SplitSeq(out, []byte{0x1e}) {
		lines := strings.Split(strings.TrimRight(string(rec), "\n"), "\n")
		h := strings.SplitN(lines[0], "\x1f", 4)
		if len(h) != 4 {
			continue
		}
		t, _ := strconv.ParseInt(h[2], 10, 64)
		v := BlockVersion{ID: h[0], Author: h[1], Time: t, Subject: h[3], Lines: []BlockLine{}}
		inHunk := false
		for _, l := range lines[1:] {
			switch {
			case strings.HasPrefix(l, "@@ "):
				v.From, v.To = hunkNewSide(l)
				inHunk = true
			case strings.HasPrefix(l, "diff --git "):
				inHunk = false
			case !inHunk || l == "":
			case l[0] == ' ' || l[0] == '+' || l[0] == '-':
				v.Lines = append(v.Lines, BlockLine{Kind: l[:1], Text: strings.TrimSuffix(l[1:], "\r")})
			}
		}
		versions = append(versions, v)
	}
	return versions
}

// hunkNewSide reads where a hunk's lines are on its new side, "@@ -a,b +c,d @@": from c to
// c+d-1, d being 1 when it isn't given.
func hunkNewSide(header string) (from, to int) {
	f := strings.Fields(header)
	if len(f) < 3 || !strings.HasPrefix(f[2], "+") {
		return 0, -1
	}
	start, count, found := strings.Cut(f[2][1:], ",")
	from, _ = strconv.Atoi(start)
	n := 1
	if found {
		n, _ = strconv.Atoi(count)
	}
	return from, from + n - 1
}
