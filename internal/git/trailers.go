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
const TrailerParserVersion = 1

// MessageTrailers returns the trailers of a whole commit message: its subject, a blank
// line, and a body. The subject is never a trailer, though "fix: the gauge" looks like one.
func MessageTrailers(message string) []Trailer {
	_, body, _ := strings.Cut(message, "\n")
	_, trailers := SplitTrailers(strings.TrimSpace(body))
	return trailers
}

// SplitTrailers separates a message body's trailers from the rest of it. As in git, the
// trailers are the last paragraph when every line in it is a trailer or an indented
// continuation of one. Closing references count as trailers too, and so does a paragraph
// of them just before the trailers, as in "Closes #12", blank line, "Co-authored-by: …".
func SplitTrailers(body string) (rest string, trailers []Trailer) {
	body = strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), " \t\n")
	paras := paragraphs.Split(body, -1)
	keep := len(paras)
	for keep > 0 {
		t := parseTrailerParagraph(paras[keep-1])
		if t == nil {
			break
		}
		trailers = append(t, trailers...)
		keep--
	}
	if len(trailers) == 0 {
		return body, nil
	}
	return strings.TrimSpace(strings.Join(paras[:keep], "\n\n")), trailers
}

// parseTrailerParagraph returns a paragraph's trailers, or nil if any line isn't one.
func parseTrailerParagraph(para string) []Trailer {
	var out []Trailer
	for line := range strings.SplitSeq(para, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if (line[0] == ' ' || line[0] == '\t') && len(out) > 0 {
			out[len(out)-1].Value += " " + strings.TrimSpace(line)
			continue
		}
		if m := closingLine.FindStringSubmatch(line); m != nil {
			out = append(out, Trailer{Key: m[1], Value: m[2]})
			continue
		}
		m := trailerLine.FindStringSubmatch(line)
		// "https://…" on a line of its own looks like a trailer keyed "https"; it isn't.
		if m == nil || strings.HasPrefix(m[2], "//") {
			return nil
		}
		out = append(out, Trailer{Key: m[1], Value: m[2]})
	}
	return out
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
