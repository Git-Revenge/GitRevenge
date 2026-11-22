package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"slices"
	"strings"
)

// Trailer is a "Key: value" line at the end of a commit message, as git interpret-trailers
// reads it (Co-authored-by, Signed-off-by and the like), or a closing reference such as
// "Closes #12", kept as Key "Closes" and Value "#12".
type Trailer struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var (
	trailerLine = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*)[ \t]*:[ \t]*(\S.*?)[ \t]*$`)
	// GitHub's and GitLab's closing keywords, which are written without a colon.
	closingLine = regexp.MustCompile(`(?i)^(close[sd]?|fix(?:e[sd])?|resolve[sd]?)[ \t]+(#\d+(?:[ \t]*,[ \t]*#\d+)*)[ \t]*$`)
	paragraphs  = regexp.MustCompile(`\n[ \t]*\n`)
)

// TrailerParserVersion says which reading of a message's trailers this is. The trailer
// index keeps what MessageTrailers returned for every commit, on disk, so raise this
// whenever SplitTrailers or MessageTrailers would answer differently for a message they
// have already read: the index is then read again from the history.
const TrailerParserVersion = 2

// MessageTrailers returns the trailers of a whole commit message: its subject, a blank
// line, and a body. The subject is never a trailer, though "fix: the gauge" looks like one.
func MessageTrailers(message string) []Trailer {
	_, body, _ := strings.Cut(message, "\n")
	_, trailers := SplitTrailers(strings.TrimSpace(body))
	return trailers
}

// SplitTrailers separates a message body's trailers from the rest of it. As in git, the
// trailers are in the last paragraph: every line of it a trailer or an indented
// continuation of one, or, where it has a line git itself writes (see gitWritten), at
// least a quarter of them. The lines of such a paragraph that aren't trailers stay in the
// message, and so does what git doesn't read at the very end (see ignoredTail). Closing
// references count as trailers too, and so does a paragraph of them just before the
// trailers, as in "Closes #12", blank line, "Co-authored-by: …".
func SplitTrailers(body string) (rest string, trailers []Trailer) {
	body = strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), " \t\n")
	cut := ignoredTail(body)
	tail := strings.TrimLeft(body[cut:], "\n")
	paras := paragraphs.Split(strings.TrimRight(body[:cut], " \t\n"), -1)
	keep := len(paras)
	var notes []string
	// Each paragraph's trailers, last paragraph first; put in order once, at the end, so a
	// message of many short paragraphs isn't copied again for each.
	var found [][]Trailer
	n := 0
	for keep > 0 {
		// Only the message's last paragraph may have other lines among its trailers.
		t, others := parseTrailerParagraph(paras[keep-1], keep == len(paras))
		if t == nil {
			break
		}
		found = append(found, t)
		n += len(t)
		keep--
		if others != nil {
			notes = others
		}
	}
	if n == 0 {
		return body, nil
	}
	trailers = make([]Trailer, 0, n)
	for _, t := range slices.Backward(found) {
		trailers = append(trailers, t...)
	}
	left := slices.Clone(paras[:keep])
	if notes != nil {
		// A note written among the trailers is still part of the message, where it was.
		left = append(left, strings.Join(notes, "\n"))
	}
	if tail != "" {
		left = append(left, tail)
	}
	return strings.TrimSpace(strings.Join(left, "\n\n")), trailers
}

// ignoredTail returns where the end of a message that git doesn't read for trailers
// begins: a closing run of comment lines, blank lines and the "Conflicts:" list of files
// that git once wrote into a merge's message, and that some maintainers still write under
// their sign-off. It's len(body) when the message has no such end.
func ignoredTail(body string) int {
	start, conflicts := -1, false
	for at := 0; at < len(body); {
		line, next := body[at:], len(body)
		if nl := strings.IndexByte(line, '\n'); nl >= 0 {
			line, next = line[:nl], at+nl+1
		}
		switch {
		case line == "" || line[0] == '#':
			if start < 0 {
				start = at
			}
		case line == "Conflicts:":
			conflicts = true
			if start < 0 {
				start = at
			}
		case conflicts && line[0] == '\t':
			// A file in the list.
		default:
			start, conflicts = -1, false
		}
		at = next
	}
	if start < 0 {
		return len(body)
	}
	return start
}

// gitWritten are the starts of the lines git itself writes at the end of a message, in
// lower case: a sign-off, and the note a cherry-pick -x leaves. A closing paragraph that has
// one is trailers by git's own rule, though other lines sit among them: the Linux kernel
// puts a bracketed note about who changed what between its sign-offs.
var gitWritten = []string{"signed-off-by:", "(cherry picked from commit "}

// parseTrailerParagraph returns a paragraph's trailers, or nil if it isn't a paragraph of
// them. With mixed, it may hold other lines too, as git allows: comment lines, or, where it
// has a line git itself writes, any lines so long as a quarter of them all are trailers.
// Those other lines are returned as others, in order, each with its own continuation lines.
func parseTrailerParagraph(para string, mixed bool) (out []Trailer, others []string) {
	const none, trailer, other = 0, 1, 2
	last := none
	recognized := false
	// Counted as git counts them: a line git writes is a trailer line though it has no
	// key, and a note's continuation lines are lines that aren't.
	trailerLines, otherLines := 0, 0
	// The lines carrying on the last trailer or other line, joined to it once it ends: adding
	// each as it came would copy all before it again, which for a long run is very slow.
	var more []string
	carried := func() {
		if len(more) == 0 {
			return
		}
		switch last {
		case trailer:
			out[len(out)-1].Value += " " + strings.Join(more, " ")
		case other:
			others[len(others)-1] += "\n" + strings.Join(more, "\n")
		}
		more = more[:0]
	}
	for line := range strings.SplitSeq(para, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// An indented line carries on whatever came before it.
		if line[0] == ' ' || line[0] == '\t' {
			switch last {
			case trailer:
				more = append(more, strings.TrimSpace(line))
				continue
			case other:
				more = append(more, line)
				otherLines++
				continue
			}
		}
		carried()
		// Git reads past a comment line among the trailers: it's neither one of them nor a
		// line against them.
		if line[0] == '#' {
			others = append(others, line)
			last = other
			continue
		}
		lower := strings.ToLower(line)
		written := false
		for _, start := range gitWritten {
			written = written || strings.HasPrefix(lower, start)
		}
		recognized = recognized || written
		if m := closingLine.FindStringSubmatch(line); m != nil {
			out = append(out, Trailer{Key: m[1], Value: m[2]})
			last = trailer
			trailerLines++
			continue
		}
		m := trailerLine.FindStringSubmatch(line)
		// "https://…" on a line of its own looks like a trailer keyed "https"; it isn't.
		if m == nil || strings.HasPrefix(m[2], "//") {
			others = append(others, line)
			last = other
			if written {
				trailerLines++
			} else {
				otherLines++
			}
			continue
		}
		out = append(out, Trailer{Key: m[1], Value: m[2]})
		last = trailer
		trailerLines++
	}
	carried()
	switch {
	case len(others) == 0:
		return out, nil
	case !mixed || len(out) == 0:
		return nil, nil
	case otherLines == 0, recognized && trailerLines*3 >= otherLines:
		return out, others
	}
	return nil, nil
}

// TrailerQuery searches for commits with a trailer: Key, matched without regard to case,
// whose value contains Value (any value when it's empty).
type TrailerQuery struct {
	Key   string
	Value string
}

var trailerQueryRE = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*)[ \t]*:[ \t]*(.*?)[ \t]*$`)

// commonTrailerKeys are trailer keys that don't end in "-by".
var commonTrailerKeys = map[string]bool{
	"close": true, "closes": true, "closed": true, "fixes": true, "fixed": true,
	"resolve": true, "resolves": true, "resolved": true, "link": true, "cc": true, "refs": true,
	"bug": true, "change-id": true,
}

// SplitTrailerQuery splits a search written "Key: value" into its two parts, whatever the
// key is. Whether it's a trailer search is for the caller to say: by ParseTrailerQuery's
// list of keys, or by which keys a repository's commits really write.
func SplitTrailerQuery(q string) (key, value string, ok bool) {
	m := trailerQueryRE.FindStringSubmatch(strings.TrimSpace(q))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// ParseTrailerQuery reads a search like "Co-authored-by: ada" or "Closes: #12" as a trailer
// search. The key must look like a trailer: ending in "-by", or a common one such as
// Closes, Fixes or Link. Conventional-commit searches like "fix: crash" stay text searches.
func ParseTrailerQuery(q string) (TrailerQuery, bool) {
	m := trailerQueryRE.FindStringSubmatch(strings.TrimSpace(q))
	if m == nil {
		return TrailerQuery{}, false
	}
	key := strings.ToLower(m[1])
	if !strings.HasSuffix(key, "-by") && !commonTrailerKeys[key] {
		return TrailerQuery{}, false
	}
	return TrailerQuery{Key: m[1], Value: m[2]}, true
}

// Matches reports whether t is a trailer the query asks for. An issue reference such as
// "#12" must match whole, so it doesn't match "#123".
func (q TrailerQuery) Matches(t Trailer) bool {
	if !strings.EqualFold(t.Key, q.Key) {
		return false
	}
	if q.Value == "" {
		return true
	}
	if issueRef.MatchString(q.Value) {
		return regexp.MustCompile(`(^|\D)` + regexp.QuoteMeta(q.Value) + `(\D|$)`).MatchString(t.Value)
	}
	return strings.Contains(strings.ToLower(t.Value), strings.ToLower(q.Value))
}

var issueRef = regexp.MustCompile(`^#\d+$`)

// ereQuote escapes s for a POSIX extended regular expression.
func ereQuote(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\.^$|?*+()[]{}`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// CommitsWithTrailer returns the IDs of commits in the graph's history that have a trailer
// matching q. git log --grep narrows the history to messages with a matching line, then
// each message's trailers are parsed, so a line elsewhere in a message doesn't count.
func (r *Repo) CommitsWithTrailer(ctx context.Context, q TrailerQuery) ([]string, error) {
	pattern := "^" + ereQuote(q.Key) + "[[:space:]]*:?[[:space:]]*.*" + ereQuote(q.Value)
	args := append([]string{"log", "-i", "-E", "--grep=" + pattern, "--format=%H%x1f%B%x1e", "--no-show-signature"},
		r.logRevs(ctx, LogOptions{})...)
	ids := []string{}
	err := r.Stream(ctx, func(rd io.Reader) error {
		br := bufio.NewReaderSize(rd, 64*1024)
		for {
			rec, err := br.ReadBytes(0x1e)
			if id, msg, ok := strings.Cut(strings.TrimLeft(string(bytes.TrimSuffix(rec, []byte{0x1e})), "\n"), "\x1f"); ok {
				_, body, _ := strings.Cut(msg, "\n")
				_, trailers := SplitTrailers(strings.TrimSpace(body))
				if slices.ContainsFunc(trailers, q.Matches) {
					ids = append(ids, id)
				}
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}, args...)
	if isEmptyRepoLogError(err) {
		return ids, nil
	}
	return ids, err
}
