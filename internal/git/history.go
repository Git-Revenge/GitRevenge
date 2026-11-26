package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
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

// FileHistory lists the commits that changed path, newest first, following renames.
func (r *Repo) FileHistory(ctx context.Context, path string) ([]FileCommit, error) {
	out, err := r.readCmd(ctx, Cmd{
		Args: []string{
			"log", "--follow", "--name-status", "-z",
			"--max-count=" + strconv.Itoa(MaxFileHistory),
			"--format=%x1e%H%x1f%aN%x1f%at%x1f%s",
			"--", path,
		},
		Env: []string{"GIT_LITERAL_PATHSPECS=1"},
	})
	if err != nil {
		return nil, err
	}
	return parseFileHistory(out), nil
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
}

// Blame attributes each line of path (at rev, or the work tree when rev is "") to the
// commit that last changed it. Blame reads every version of the file: in a partial clone,
// only mayDownload lets git fetch the ones never downloaded, one at a time (#1003);
// otherwise it stops at the first and says which (a NotDownloadedError).
func (r *Repo) Blame(ctx context.Context, path, rev string, mayDownload bool) ([]BlameLine, error) {
	if err := CheckRevs(rev); err != nil {
		return nil, err
	}
	args := []string{"blame", "--porcelain"}
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
	return lines, err
}

// blobNotRead is how blame says a version wasn't there to read.
var blobNotRead = regexp.MustCompile(`Cannot read blob ([0-9a-f]{40,64})`)

type blameCommit struct {
	author  string
	time    int64
	summary string
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
				bl.Author, bl.Time, bl.Summary = c.author, c.time, c.summary
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
		}
	}
	return lines, sc.Err()
}
