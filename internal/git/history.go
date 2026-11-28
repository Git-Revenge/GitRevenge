package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// FileCommit is one commit in a file's history.
type FileCommit struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Time    int64  `json:"time"`
	Subject string `json:"subject"`
	// Path is the file's name in this commit; OldPath is set when the commit renamed it.
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
	// Status is the change to the file: A, M, D, R…
	Status string `json:"status"`
}

// MaxFileHistory caps how many commits FileHistory returns.
const MaxFileHistory = 5000

// HistoryStop says where a file's history stopped in a partial clone (#1010): following the
// rename at Commit compares files never downloaded. Missing lists them, to download in one
// request and follow further.
type HistoryStop struct {
	Commit  string   `json:"commit"`
	Missing []string `json:"missing"`
}

// FileHistory lists the commits that changed path, newest first, following renames. It
// downloads nothing: in a partial clone, where a rename can't be followed without the files
// it compares, the history ends there and stop says so.
func (r *Repo) FileHistory(ctx context.Context, path string) (commits []FileCommit, stop *HistoryStop, err error) {
	out, err := r.readCmd(ctx, Cmd{
		Args: []string{
			"log", "--follow", "--name-status", "-z",
			"--max-count=" + strconv.Itoa(MaxFileHistory),
			"--format=%x1e%H%x1f%aN%x1f%at%x1f%s",
			"--", path,
		},
		Env: append([]string{"GIT_LITERAL_PATHSPECS=1"}, noLazyFetch...),
	})
	if err == nil {
		return parseFileHistory(out), nil, nil
	}
	if ctx.Err() != nil || !r.Partial(ctx) {
		return nil, nil, err
	}
	// git wrote the commits up to the one it couldn't follow past.
	commits = parseFileHistory(out)
	if stop, serr := r.historyStop(ctx, commits, path); serr == nil && stop != nil {
		return commits, stop, nil
	}
	return nil, nil, err
}

// historyStop finds the commit a file's history stopped at, after commits, and what following
// a rename there reads that a partial clone hasn't got: the file's versions there and every
// file the commit deleted, any of which it may have been renamed from.
func (r *Repo) historyStop(ctx context.Context, commits []FileCommit, path string) (*HistoryStop, error) {
	from, cur := "HEAD", path
	if len(commits) > 0 {
		last := commits[len(commits)-1]
		from, cur = last.ID+"^", last.Path
		if last.OldPath != "" {
			cur = last.OldPath
		}
	}
	at, err := r.readCmd(ctx, Cmd{Args: []string{"rev-list", "-1", from, "--", cur}, Env: append([]string{"GIT_LITERAL_PATHSPECS=1"}, noLazyFetch...)})
	commit := strings.TrimSpace(string(at))
	if err != nil || commit == "" {
		return nil, err
	}
	args := []string{"diff-tree", "-r", "-z", "--raw", "--no-renames", "--no-abbrev", "--no-commit-id", "--root", commit}
	raw, err := r.readCmd(ctx, Cmd{Args: args, Env: noLazyFetch})
	if err != nil {
		return nil, err
	}
	var ids []string
	f := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	for i := 0; i+1 < len(f); i += 2 {
		head := strings.Fields(f[i])
		if len(head) < 5 || !strings.HasPrefix(head[0], ":") {
			continue
		}
		if p := f[i+1]; p == cur {
			for _, id := range head[2:4] {
				if strings.Trim(id, "0") != "" {
					ids = append(ids, id)
				}
			}
		} else if head[4] == "D" && head[0] != ":160000" {
			ids = append(ids, head[2])
		}
	}
	missing, err := r.missingOf(ctx, ids)
	if err != nil || len(missing) == 0 {
		return nil, err
	}
	return &HistoryStop{Commit: commit, Missing: missing}, nil
}

// parseFileHistory reads records of: RS sha US author US time US subject, then -z
// name-status fields (STATUS NUL path NUL, or for renames STATUS NUL old NUL new NUL).
func parseFileHistory(out []byte) []FileCommit {
	commits := []FileCommit{}
	for rec := range bytes.SplitSeq(out, []byte{0x1e}) {
		if len(rec) == 0 {
			continue
		}
		header, rest, _ := bytes.Cut(rec, []byte{0})
		// With -z, the header line ends with a newline before the first status field.
		header, extra, _ := bytes.Cut(header, []byte{'\n'})
		h := strings.SplitN(string(header), "\x1f", 4)
		if len(h) != 4 {
			continue
		}
		t, _ := strconv.ParseInt(h[2], 10, 64)
		c := FileCommit{ID: h[0], Author: h[1], Time: t, Subject: h[3]}
		fields := strings.Split(strings.Trim(string(extra)+"\x00"+string(rest), "\x00\n"), "\x00")
		if len(fields) >= 2 {
			c.Status = fields[0][:1]
			if (c.Status == "R" || c.Status == "C") && len(fields) >= 3 {
				c.OldPath, c.Path = fields[1], fields[2]
			} else {
				c.Path = fields[1]
			}
		}
		commits = append(commits, c)
	}
	return commits
}

// BlameLine is one line of a blamed file.
type BlameLine struct {
	Commit  string `json:"commit"`
	Author  string `json:"author"`
	Time    int64  `json:"time"`
	Summary string `json:"summary"`
	// Line is the line's text, without its newline.
	Line string `json:"line"`
	// OrigLine is the line number in that commit's version of the file.
	OrigLine int `json:"origLine"`
	// Edge means Commit is at a shallow clone's edge (#1006): the line is as old as it, or
	// older, from history the clone didn't fetch.
	Edge bool `json:"edge,omitempty"`
	// boundary is blame's word that it looked no further back than Commit.
	boundary bool
}

// Blame attributes each line of path (at rev, or the work tree when rev is "") to the
// commit that last changed it. Blame reads every version of the file: in a partial clone,
// only mayDownload lets git fetch the ones never downloaded, one at a time (#1003);
// otherwise it stops at the first and says which (a NotDownloadedError).
func (r *Repo) Blame(ctx context.Context, path, rev string, mayDownload bool) ([]BlameLine, error) {
	return r.blame(ctx, path, rev, nil, mayDownload)
}

// BlameRange is lines From to To of a file, counting from 1, both included.
type BlameRange struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// BlameRanges blames only some lines of path at rev, as Blame does the whole file, without
// downloading: the blame view's track works out the rest of a version's lines from a
// neighbour it has blamed (#1079). On the Linux kernel's mm/slub.c a few ranges took 168 to
// 385 ms, against 1.4 s for every line. The lines come back in file order. No ranges is
// every line.
func (r *Repo) BlameRanges(ctx context.Context, path, rev string, ranges []BlameRange) ([]BlameLine, error) {
	return r.blame(ctx, path, rev, ranges, false)
}

func (r *Repo) blame(ctx context.Context, path, rev string, ranges []BlameRange, mayDownload bool) ([]BlameLine, error) {
	if err := CheckRevs(rev); err != nil {
		return nil, err
	}
	args := []string{"blame", "--porcelain"}
	for _, rg := range ranges {
		if rg.From < 1 || rg.To < rg.From {
			return nil, fmt.Errorf("lines %d to %d aren't a range", rg.From, rg.To)
		}
		args = append(args, "-L", strconv.Itoa(rg.From)+","+strconv.Itoa(rg.To))
	}
	if rev != "" {
		args = append(args, rev)
	}
	args = append(args, "--", path)
	c := Cmd{Args: args}
	if !mayDownload {
		c.Env = noLazyFetch
	}
	var lines []BlameLine
	err := r.streamCmd(ctx, c, func(rd io.Reader) error {
		var err error
		lines, err = parseBlame(rd)
		return err
	})
	var gerr *Error
	if errors.As(err, &gerr) {
		if m := blobNotRead.FindStringSubmatch(gerr.Stderr); m != nil {
			return nil, &NotDownloadedError{IDs: []string{m[1]}}
		}
	}
	if err == nil {
		r.markEdge(ctx, lines)
	}
	return lines, err
}

// markEdge marks the lines blame gives a shallow clone's edge commits (#1006). git can't look
// past the edge, so such a line was written there or earlier: the view says "before" its date
// rather than crediting the edge commit with it.
func (r *Repo) markEdge(ctx context.Context, lines []BlameLine) {
	if !slices.ContainsFunc(lines, func(l BlameLine) bool { return l.boundary }) {
		return
	}
	edge := r.shallowSet(ctx)
	for i := range lines {
		lines[i].Edge = lines[i].boundary && edge[lines[i].Commit]
	}
}

// blobNotRead is how blame says a version wasn't there to read.
var blobNotRead = regexp.MustCompile(`Cannot read blob ([0-9a-f]{40,64})`)

type blameCommit struct {
	author   string
	time     int64
	summary  string
	boundary bool
}

// parseBlame reads `git blame --porcelain`. Commit details appear only the first time a
// commit is mentioned, so they are remembered by ID.
func parseBlame(rd io.Reader) ([]BlameLine, error) {
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	commits := map[string]*blameCommit{}
	lines := []BlameLine{}
	var cur string
	var orig int
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "\t") {
			c := commits[cur]
			bl := BlameLine{Commit: cur, OrigLine: orig, Line: strings.TrimSuffix(line[1:], "\r")}
			if c != nil {
				bl.Author, bl.Time, bl.Summary, bl.boundary = c.author, c.time, c.summary, c.boundary
			}
			lines = append(lines, bl)
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		if IsCommitID(key) {
			// "<sha> <orig-line> <final-line> [<count>]" starts each line's entry.
			cur = key
			f := strings.Fields(val)
			if len(f) > 0 {
				orig, _ = strconv.Atoi(f[0])
			}
			if commits[cur] == nil {
				commits[cur] = &blameCommit{}
			}
			continue
		}
		c := commits[cur]
		if c == nil {
			continue
		}
		switch key {
		case "author":
			c.author = val
		case "author-time":
			c.time, _ = strconv.ParseInt(val, 10, 64)
		case "summary":
			c.summary = val
		case "boundary":
			c.boundary = true
		}
	}
	return lines, sc.Err()
}
