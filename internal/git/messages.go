package git

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
)

// Reading every commit's message, for what is worked out from messages once and kept (the
// trailer index). The commits are named by id, never by ref, so the same question can be
// asked again later and git walks them in the same order: a read that was cut short picks
// up with Skip.

// MessageRange says which commits to read: everything reachable from Tips and from none
// of Not, newest first.
type MessageRange struct {
	// Tips and Not are full object ids, of commits or of tags.
	Tips []string
	Not  []string
	// Skip leaves out that many commits from the start of the walk.
	Skip int
}

// stdin is the range as git reads revisions from standard input: more ids than a command
// line holds on Windows fit this way.
func (m MessageRange) stdin() (io.Reader, error) {
	var b strings.Builder
	for _, set := range [][]string{m.Tips, m.Not} {
		for _, id := range set {
			if !isHex(id) {
				return nil, errors.New("a message range takes object ids, and got " + strconv.Quote(id))
			}
		}
	}
	for _, id := range m.Tips {
		b.WriteString(id)
		b.WriteByte('\n')
	}
	for _, id := range m.Not {
		b.WriteByte('^')
		b.WriteString(id)
		b.WriteByte('\n')
	}
	return strings.NewReader(b.String()), nil
}

// CommitMessages calls each with every commit in the range and its message, as written,
// newest first. It stops with each's error if it returns one. A range with no tips has no
// commits.
func (r *Repo) CommitMessages(ctx context.Context, m MessageRange, each func(id, message string) error) error {
	if len(m.Tips) == 0 {
		return nil
	}
	in, err := m.stdin()
	if err != nil {
		return err
	}
	// %B is the message as written. -z ends each commit with a NUL, which no message holds.
	args := []string{"-c", "log.showSignature=false", "-c", "i18n.logOutputEncoding=UTF-8", "log", "-z", "--format=%H%n%B", "--stdin"}
	if m.Skip > 0 {
		args = append(args, "--skip="+strconv.Itoa(m.Skip))
	}
	return r.streamCmd(ctx, Cmd{Args: args, Stdin: in}, func(rd io.Reader) error {
		br := bufio.NewReaderSize(rd, 256*1024)
		for {
			rec, err := br.ReadString(0)
			if rec = strings.TrimSuffix(rec, "\x00"); rec != "" {
				id, message, _ := strings.Cut(rec, "\n")
				if e := each(id, message); e != nil {
					return e
				}
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	})
}

// CountMessages returns how many commits CommitMessages would read for the range, Skip
// aside. It's quick where the repository has a commit-graph file.
func (r *Repo) CountMessages(ctx context.Context, m MessageRange) (int, error) {
	if len(m.Tips) == 0 {
		return 0, nil
	}
	in, err := m.stdin()
	if err != nil {
		return 0, err
	}
	out, err := r.readCmd(ctx, Cmd{Args: []string{"rev-list", "--count", "--stdin"}, Stdin: in})
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// HistoryTips returns the object id of every branch, remote-tracking branch and tag, and of
// HEAD when it points at a commit: where the history a repository shows starts from. Each id
// is listed once, in the order git lists the refs.
func (r *Repo) HistoryTips(ctx context.Context) ([]string, error) {
	out, err := r.ReadString(ctx, "for-each-ref", "--format=%(objectname)", "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(out)
	// A detached HEAD may be on a commit no ref reaches.
	if head, err := r.ReadString(ctx, "rev-parse", "--verify", "-q", "HEAD"); err == nil && head != "" {
		ids = append(ids, head)
	}
	seen := make(map[string]bool, len(ids))
	tips := ids[:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			tips = append(tips, id)
		}
	}
	return tips, nil
}

// MissingObjects returns which of ids the repository no longer has, such as commits a
// garbage collection removed after their branch was deleted.
func (r *Repo) MissingObjects(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	for _, id := range ids {
		if !isHex(id) {
			return nil, errors.New("not an object id: " + strconv.Quote(id))
		}
	}
	out, err := r.readCmd(ctx, Cmd{Args: []string{"cat-file", "--batch-check=%(objectname) %(objecttype)"}, Stdin: strings.NewReader(strings.Join(ids, "\n") + "\n")})
	if err != nil {
		return nil, err
	}
	var missing []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		// git answers "<id> missing" for an object it doesn't have.
		if id, what, _ := strings.Cut(line, " "); what == "missing" {
			missing = append(missing, id)
		}
	}
	return missing, nil
}
