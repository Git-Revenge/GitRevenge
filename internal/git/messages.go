package git

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"slices"
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
	// Skip leaves out that many commits from the start of the walk, and Max, when above 0,
	// stops after that many: a slice of the walk, which several readers can take side by side.
	Skip int
	Max  int
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
	if m.Max > 0 {
		args = append(args, "--max-count="+strconv.Itoa(m.Max))
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

// CountMessages returns how many commits CommitMessages would read for the range, Skip and
// Max aside. It's quick where the repository has a commit-graph file.
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

// CommitIDs says which of ids, full or abbreviated, name a commit in the repository, and
// returns the full id of each that does. One that names nothing, something that isn't a
// commit, or more than one object is left out.
func (r *Repo) CommitIDs(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, id := range ids {
		if !isHex(id) {
			return nil, errors.New("not an object id: " + strconv.Quote(id))
		}
	}
	// git takes about a tenth of a millisecond to place an abbreviated id, which for the
	// hundred thousand in the Linux kernel's Fixes trailers is eleven seconds. Past a
	// couple of thousand it's quicker to list every commit once and look them up here.
	if len(ids) > commitIDsOneByOne {
		return r.commitIDsByList(ctx, ids)
	}
	// git answers a line for each line asked, in order: "<full id> <type>" for an object
	// it has, and the id as asked with "missing" or "ambiguous" otherwise.
	raw, err := r.readCmd(ctx, Cmd{Args: []string{"cat-file", "--batch-check=%(objectname) %(objecttype)"}, Stdin: strings.NewReader(strings.Join(ids, "\n") + "\n")})
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != len(ids) {
		return nil, errors.New("git answered " + strconv.Itoa(len(lines)) + " lines for " + strconv.Itoa(len(ids)) + " ids")
	}
	for i, line := range lines {
		if full, what, _ := strings.Cut(line, " "); what == "commit" {
			out[ids[i]] = full
		}
	}
	return out, nil
}

// commitIDsOneByOne is how many ids CommitIDs asks git about one at a time.
const commitIDsOneByOne = 2000

// commitIDsByList answers CommitIDs from a list of every commit any ref reaches: the ids
// sorted, and each one asked about found by its place among them. A commit no ref reaches
// isn't found this way, which is right for what the graph can show.
func (r *Repo) commitIDsByList(ctx context.Context, ids []string) (map[string]string, error) {
	var all []byte
	size := 0
	err := r.Stream(ctx, func(rd io.Reader) error {
		sc := bufio.NewScanner(rd)
		for sc.Scan() {
			line := sc.Bytes()
			if size == 0 {
				size = len(line) / 2
			}
			if len(line) != size*2 {
				continue
			}
			at := len(all)
			all = append(all, make([]byte, size)...)
			if _, err := hex.Decode(all[at:], line); err != nil {
				all = all[:at]
			}
		}
		return sc.Err()
	}, "rev-list", "--all")
	if err != nil && !isEmptyRepoLogError(err) {
		return nil, err
	}
	out := map[string]string{}
	if size == 0 {
		return out, nil
	}
	n := len(all) / size
	order := make([]uint32, n)
	for i := range order {
		order[i] = uint32(i)
	}
	id := func(i uint32) []byte { return all[int(i)*size : (int(i)+1)*size] }
	slices.SortFunc(order, func(a, b uint32) int { return bytes.Compare(id(a), id(b)) })

	// An abbreviation names every id from itself followed by zeros to itself followed by
	// fs. It's a commit when exactly one id is in that span.
	low, high := make([]byte, size), make([]byte, size)
	pad := make([]byte, size*2)
	for _, asked := range ids {
		if len(asked) > size*2 {
			continue
		}
		short := strings.ToLower(asked)
		span := func(fill byte, into []byte) {
			copy(pad, short)
			for i := len(short); i < len(pad); i++ {
				pad[i] = fill
			}
			_, _ = hex.Decode(into, pad)
		}
		span('0', low)
		span('f', high)
		first, _ := slices.BinarySearchFunc(order, low, func(i uint32, want []byte) int { return bytes.Compare(id(i), want) })
		if first >= n || bytes.Compare(id(order[first]), high) > 0 {
			continue
		}
		if first+1 < n && bytes.Compare(id(order[first+1]), high) <= 0 {
			continue // two commits start this way
		}
		out[asked] = hex.EncodeToString(id(order[first]))
	}
	return out, nil
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
