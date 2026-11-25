package git

import (
	"bufio"
	"context"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// The history search (#981): the commits where a text appeared or went away (git log -S),
// or whose added or removed lines have it (-G). The commits are listed once, newest first,
// and handed to git in chunks, several at a time: each chunk done is progress that can be
// shown, stopped at and carried on from. On the Linux kernel, chunks run on seven processes
// took 2.5 s for what one walk took 12 s to search.

// HistoryOptions is what a history search looks for.
type HistoryOptions struct {
	// Text is what to find: as written, or a regular expression when Regex is set. It
	// can't hold a line break.
	Text      string `json:"text"`
	MatchCase bool   `json:"matchCase"`
	Regex     bool   `json:"regex"`
	// Lines counts a commit when a line it adds or removes has the text (-G), rather than
	// when the number of times the text is there changes (-S).
	Lines bool `json:"lines"`
	// Paths narrows the search to some files, as Find's Files box: "!x" leaves x out.
	Paths []string `json:"paths"`
	// Since leaves out commits older than this, in Unix seconds; 0 is the whole history.
	Since int64 `json:"since"`
}

// HistoryCommit is a commit to search, and its committer date in Unix seconds.
type HistoryCommit struct {
	ID string
	At int64
}

// HistoryChange is how a commit changed the text in one file: how many times its added
// lines have it and its removed lines have it, or under Lines, how many of each match.
type HistoryChange struct {
	File    string `json:"file"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// HistoryCommits lists the commits a history search goes through, newest first: those the
// graph shows for opts, made since since (0 for all).
func (r *Repo) HistoryCommits(ctx context.Context, opts LogOptions, since int64, each func(HistoryCommit) error) error {
	args := []string{"rev-list", "--timestamp"}
	if since > 0 {
		args = append(args, "--since=@"+strconv.FormatInt(since, 10))
	}
	args = append(args, r.logRevs(ctx, opts)...)
	err := r.Stream(ctx, func(out io.Reader) error {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			at, id, ok := strings.Cut(sc.Text(), " ")
			if !ok {
				continue
			}
			t, _ := strconv.ParseInt(at, 10, 64)
			if err := each(HistoryCommit{ID: id, At: t}); err != nil {
				return err
			}
		}
		return sc.Err()
	}, args...)
	if isEmptyRepoLogError(err) {
		return nil
	}
	return err
}

// HistoryCount counts the commits HistoryCommits would list. It's cheap with a commit-graph.
func (r *Repo) HistoryCount(ctx context.Context, opts LogOptions, since int64) (int, error) {
	args := []string{"rev-list", "--count"}
	if since > 0 {
		args = append(args, "--since=@"+strconv.FormatInt(since, 10))
	}
	out, err := r.ReadString(ctx, append(args, r.logRevs(ctx, opts)...)...)
	if isEmptyRepoLogError(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

// HistoryChunk returns which of ids o finds, in the order given. A commit's files that were
// never downloaded, in a partial clone, stop it with ErrNotDownloaded: nothing is fetched.
func (r *Repo) HistoryChunk(ctx context.Context, o HistoryOptions, ids []string) ([]string, error) {
	c, err := r.historyCmd(ctx, o, ids, "--format=%H")
	if err != nil {
		return nil, err
	}
	out, err := r.readCmd(ctx, c)
	if err = historyError(err); err != nil {
		return nil, err
	}
	var found []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			found = append(found, line)
		}
	}
	return found, nil
}

// HistoryChanges says, for each of ids that o finds, how it changed the text in each file it
// changed it in: what the +1 and -1 beside a commit come from.
func (r *Repo) HistoryChanges(ctx context.Context, o HistoryOptions, ids []string) (map[string][]HistoryChange, error) {
	match := historyMatcher(o)
	// The patches of the files the search finds (git leaves the others out of a commit's
	// patch without --pickaxe-all), with no context lines: only what changed is counted.
	c, err := r.historyCmd(ctx, o, ids, "--format=%x01%H", "-p", "--unified=0", "--no-ext-diff", "--no-color")
	if err != nil {
		return nil, err
	}
	out, err := r.readCmd(ctx, c)
	if err = historyError(err); err != nil {
		return nil, err
	}
	changes := map[string][]HistoryChange{}
	if match == nil {
		return changes, nil
	}
	var id string
	var cur *HistoryChange
	// Between "diff --git" and the first hunk: "--- " and "+++ " name the file there, and
	// are lines removed or added after it ("-- a comment" removed reads "--- a comment").
	header := false
	flush := func() {
		if cur != nil && (cur.Added > 0 || cur.Removed > 0) {
			changes[id] = append(changes[id], *cur)
		}
		cur = nil
	}
	count := func(line string) int {
		if o.Lines {
			if match.MatchString(line) {
				return 1
			}
			return 0
		}
		return len(match.FindAllStringIndex(line, -1))
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, "\x01"):
			flush()
			id = line[1:]
		case strings.HasPrefix(line, "diff --git "):
			flush()
			cur = &HistoryChange{}
			header = true
		case cur == nil:
		case header && strings.HasPrefix(line, "--- "):
			if name := patchPath(line[4:], "a/"); name != "" {
				cur.File = name
			}
		case header && strings.HasPrefix(line, "+++ "):
			// The new name, unless the file was deleted.
			if name := patchPath(line[4:], "b/"); name != "" {
				cur.File = name
			}
		case strings.HasPrefix(line, "@@"):
			header = false
		case header:
		case strings.HasPrefix(line, "+"):
			cur.Added += count(line[1:])
		case strings.HasPrefix(line, "-"):
			cur.Removed += count(line[1:])
		}
	}
	flush()
	return changes, nil
}

// patchPath reads a patch header's file name ("b/src/feed.js", or quoted when it has odd
// characters), without its prefix; "" for /dev/null.
func patchPath(s, prefix string) string {
	s = strings.TrimSuffix(s, "\t")
	if strings.HasPrefix(s, `"`) {
		if u, err := strconv.Unquote(s); err == nil {
			s = u
		}
	}
	if s == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(s, prefix)
}

// historyCmd is git log over ids with o's search, the commits given on its input.
func (r *Repo) historyCmd(ctx context.Context, o HistoryOptions, ids []string, extra ...string) (Cmd, error) {
	if o.Text == "" || strings.ContainsAny(o.Text, "\r\n") {
		return Cmd{}, ErrFindText
	}
	// A commit's files may never have been downloaded, in a partial clone: told not to
	// fetch them, git stops, and the search says why. A git older than 2.44 can't be told.
	if r.runner.version().Less(noLazyFetch) && r.partialClone(ctx) {
		return Cmd{}, ErrLazyFetch
	}
	args := append([]string{"log", "--no-walk=unsorted", "--stdin"}, extra...)
	if o.Lines {
		text := o.Text
		if !o.Regex {
			text = quoteERE(text)
		}
		args = append(args, "-G"+text)
	} else {
		args = append(args, "-S"+o.Text)
		if o.Regex {
			args = append(args, "--pickaxe-regex")
		}
	}
	if !o.MatchCase {
		args = append(args, "-i")
	}
	args = append(args, "--")
	args = append(args, grepPathspecs(o.Paths)...)
	return Cmd{Args: args, Env: []string{"GIT_NO_LAZY_FETCH=1"}, Stdin: strings.NewReader(strings.Join(ids, "\n") + "\n")}, nil
}

// historyMatcher is o's pattern for Go, to count what a commit's lines have; nil when Go
// can't read a pattern git can.
func historyMatcher(o HistoryOptions) *regexp.Regexp {
	return grepMatcher(GrepOptions{Text: o.Text, MatchCase: o.MatchCase, Regex: o.Regex})
}

// quoteERE escapes what a POSIX extended regular expression would read as more than itself,
// for -G to find plain text.
func quoteERE(s string) string {
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(`.[]()*+?{}|^$\`, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// historyError turns how git log failed into what happened.
func historyError(err error) error {
	var ge *Error
	if !errors.As(err, &ge) {
		return err
	}
	switch {
	case strings.Contains(ge.Stderr, "promisor remote"):
		return ErrNotDownloaded
	case strings.Contains(ge.Stderr, "invalid regex: "):
		_, reason, _ := strings.Cut(ge.Stderr, "invalid regex: ")
		reason, _, _ = strings.Cut(reason, "\n")
		return &PatternError{Reason: patternReason(strings.TrimSpace(reason))}
	}
	return err
}
