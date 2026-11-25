package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Find in files asks where a text is in the files, now or at any commit, with git grep (#979).
// Results stream: each line reaches the caller as git prints it, so the first ones show while
// git is still reading a big tree.

// GrepOptions is what to find, and where.
type GrepOptions struct {
	// Text is what to find: as written, or a regular expression (POSIX extended, as
	// `git grep -E` reads it) when Regex is set. It can't hold a line break.
	Text      string `json:"text"`
	MatchCase bool   `json:"matchCase"`
	// WholeWord finds Text only where it isn't part of a longer word.
	WholeWord bool `json:"wholeWord"`
	Regex     bool `json:"regex"`
	// Untracked searches untracked files too, though not ignored ones. The working tree only.
	Untracked bool `json:"untracked"`
	// Rev is the commit to search, as named (a branch, tag or SHA), or "" for the working tree.
	Rev string `json:"rev"`
	// Paths narrows the search, as typed in the Files box: "src/", "*.js", and "!*.min.js"
	// to leave files out. Empty searches everything.
	Paths []string `json:"paths"`
}

// GrepLine is a line that matches.
type GrepLine struct {
	File string `json:"file"`
	Line int    `json:"line"`
	// Text is the line, or the part of it around its first match when it's long; CutStart
	// and CutEnd say which ends were cut.
	Text     string `json:"text"`
	CutStart bool   `json:"cutStart"`
	CutEnd   bool   `json:"cutEnd"`
	// Matches are the stretches of Text that match, [From, To) in UTF-16 units. Empty when
	// the pattern uses something git's regular expressions have and Go's don't, such as a
	// back-reference: the line is still found, only not marked.
	Matches []GrepSpan `json:"matches"`
}

// GrepSpan is [From, To) of a line, in UTF-16 units.
type GrepSpan struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// GrepSummary is how a search ended.
type GrepSummary struct {
	Lines int `json:"lines"`
	Files int `json:"files"`
	// Truncated says the search stopped at its limit, with more to find.
	Truncated bool `json:"truncated"`
}

// GrepLimit is how many lines Find shows at most: past that, a search wants narrowing more
// than reading (the ruling on #932).
const GrepLimit = 10_000

// grepCutAt is how long a line can be, in bytes, before only the part around its first
// match is kept: a minified file's one line can be a megabyte.
const grepCutAt = 400

// PatternError is a regular expression git can't read. Reason says what's wrong in words.
type PatternError struct {
	Reason string
}

func (e *PatternError) Error() string { return "not a regular expression: " + e.Reason }

// ErrNotDownloaded means the commit searched is in a partial clone and some of its files
// were never downloaded. A search never downloads them.
var ErrNotDownloaded = errors.New("some of the commit's files were never downloaded")

// ErrFindText means there's no text to find, or it spans lines, which git grep can't find.
var ErrFindText = errors.New("find looks for text within one line")

// ErrUnknownRevision means Rev names no commit.
var ErrUnknownRevision = errors.New("no such branch, tag or commit")

// ErrLazyFetch means searching the commit would download files: a partial clone, with a git
// older than 2.44, which can't be told not to.
var ErrLazyFetch = errors.New("searching this commit would download files")

// noLazyFetch is the git that understands GIT_NO_LAZY_FETCH.
var noLazyFetch = Version{2, 44, 0}

// Grep finds the lines that match o, handing each to found as git prints it, and stops after
// limit lines (0 for no limit), saying so in the summary.
func (r *Repo) Grep(ctx context.Context, o GrepOptions, limit int, found func(GrepLine)) (GrepSummary, error) {
	c, err := r.grepCmd(ctx, o, false)
	if err != nil {
		return GrepSummary{}, err
	}
	match := grepMatcher(o)
	var sum GrepSummary
	last := ""
	errLimit := errors.New("limit")
	err = r.streamCmd(ctx, c, func(out io.Reader) error {
		br := bufio.NewReaderSize(out, 64*1024)
		for {
			rec, err := br.ReadBytes('\n')
			if len(rec) > 0 {
				l, col, ok := parseGrepLine(rec, o.Rev)
				if ok {
					if limit > 0 && sum.Lines == limit {
						sum.Truncated = true
						return errLimit
					}
					markGrepLine(&l, col, match, o.WholeWord)
					sum.Lines++
					if l.File != last {
						sum.Files++
						last = l.File
					}
					found(l)
				}
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
		}
	})
	if errors.Is(err, errLimit) {
		return sum, nil
	}
	return sum, grepError(err)
}

// GrepCount counts the lines and files that match o, without reading them.
func (r *Repo) GrepCount(ctx context.Context, o GrepOptions) (GrepSummary, error) {
	c, err := r.grepCmd(ctx, o, true)
	if err != nil {
		return GrepSummary{}, err
	}
	out, err := r.readCmd(ctx, c)
	if err = grepError(err); err != nil {
		return GrepSummary{}, err
	}
	var sum GrepSummary
	// -c -z writes the file, a NUL, and its count of matching lines.
	for rec := range bytes.SplitSeq(out, []byte("\n")) {
		_, n, ok := bytes.Cut(rec, []byte{0})
		if !ok {
			continue
		}
		if k, err := strconv.Atoi(string(n)); err == nil && k > 0 {
			sum.Lines += k
			sum.Files++
		}
	}
	return sum, nil
}

// grepCmd is the git grep command for o, counting or listing lines.
func (r *Repo) grepCmd(ctx context.Context, o GrepOptions, count bool) (Cmd, error) {
	if o.Text == "" || strings.ContainsAny(o.Text, "\r\n") {
		return Cmd{}, ErrFindText
	}
	if strings.HasPrefix(o.Rev, "-") {
		return Cmd{}, fmt.Errorf("%w: %q", ErrUnknownRevision, o.Rev)
	}
	// -z ends the file name and the line number with NUL, so no path is quoted; -I skips
	// binary files.
	args := []string{"grep", "-z", "-I", "--no-color"}
	if count {
		args = append(args, "-c")
	} else {
		args = append(args, "-n", "--column")
	}
	if !o.MatchCase {
		args = append(args, "-i")
	}
	if o.WholeWord {
		args = append(args, "-w")
	}
	if o.Regex {
		args = append(args, "-E")
	} else {
		args = append(args, "-F")
	}
	if o.Untracked && o.Rev == "" {
		args = append(args, "--untracked")
	}
	args = append(args, "-e", o.Text)
	c := Cmd{}
	if o.Rev != "" {
		args = append(args, o.Rev)
		// A commit's files may never have been downloaded, in a partial clone. Git would
		// fetch each one it reads; told not to, it stops, and the search says why.
		c.Env = []string{"GIT_NO_LAZY_FETCH=1"}
		if r.runner.version().Less(noLazyFetch) && r.partialClone(ctx) {
			return Cmd{}, ErrLazyFetch
		}
	}
	args = append(args, "--")
	args = append(args, grepPathspecs(o.Paths)...)
	c.Args = args
	return c, nil
}

// grepPathspecs turns what the Files box holds into pathspecs: "!x" leaves x out.
func grepPathspecs(paths []string) []string {
	var out []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		switch {
		case p == "" || p == "!":
		case strings.HasPrefix(p, "!"):
			out = append(out, ":(exclude)"+p[1:])
		default:
			out = append(out, p)
		}
	}
	return out
}

// partialClone reports whether some remote is a promisor, so objects may be missing.
func (r *Repo) partialClone(ctx context.Context) bool {
	out, err := r.ReadString(ctx, "config", "--get-regexp", `^remote\..*\.promisor$`)
	return err == nil && strings.Contains(out, "true")
}

// parseGrepLine reads one record of `git grep -z -n --column`: the file, a NUL, the line
// number, a NUL, the column of the first match, a NUL and the line. A search of a commit
// names files "REV:path". It returns the line and the byte offset of its first match.
func parseGrepLine(rec []byte, rev string) (GrepLine, int, bool) {
	rec = bytes.TrimSuffix(rec, []byte("\n"))
	f := bytes.SplitN(rec, []byte{0}, 4)
	if len(f) != 4 {
		return GrepLine{}, 0, false
	}
	n, err := strconv.Atoi(string(f[1]))
	if err != nil {
		return GrepLine{}, 0, false
	}
	col, _ := strconv.Atoi(string(f[2]))
	file := string(f[0])
	if rev != "" {
		file = strings.TrimPrefix(file, rev+":")
	}
	text := string(bytes.TrimSuffix(f[3], []byte("\r")))
	at := min(max(col-1, 0), len(text))
	// The column counts bytes of the line as it is; keep it pointing at the same place once
	// bytes that aren't UTF-8 are replaced.
	if !utf8.ValidString(text) {
		at = len(strings.ToValidUTF8(text[:at], "�"))
		text = strings.ToValidUTF8(text, "�")
	}
	return GrepLine{File: file, Line: n, Text: text}, at, true
}

// grepMatcher is o's pattern for Go, to mark every match in a line: git says only where the
// first one starts. Nil when Go can't read a pattern git can.
func grepMatcher(o GrepOptions) *regexp.Regexp {
	p := o.Text
	if !o.Regex {
		p = regexp.QuoteMeta(p)
	}
	if !o.MatchCase {
		p = "(?i)" + p
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil
	}
	// POSIX regular expressions take the longest match where Go's take the first.
	re.Longest()
	return re
}

// markGrepLine finds every match in l, cuts a long line down to the part around the first,
// and turns byte offsets into UTF-16 ones. first is where git says the first match starts,
// in bytes, which places the cut when Go can't read the pattern.
func markGrepLine(l *GrepLine, first int, match *regexp.Regexp, wholeWord bool) {
	var spans [][2]int
	if match != nil {
		for _, m := range match.FindAllStringIndex(l.Text, -1) {
			if m[0] == m[1] || (wholeWord && !wordAlone(l.Text, m[0], m[1])) {
				continue
			}
			spans = append(spans, [2]int{m[0], m[1]})
		}
	}
	if len(spans) > 0 {
		first = spans[0][0]
	}
	text, from := l.Text, 0
	if len(text) > grepCutAt {
		from = max(first-grepCutAt/4, 0)
		to := min(from+grepCutAt, len(text))
		for from > 0 && !utf8.RuneStart(text[from]) {
			from--
		}
		for to < len(text) && !utf8.RuneStart(text[to]) {
			to++
		}
		l.CutStart, l.CutEnd = from > 0, to < len(text)
		text = text[from:to]
	}
	l.Text = text
	l.Matches = []GrepSpan{}
	for _, s := range spans {
		a, b := s[0]-from, s[1]-from
		if b <= 0 || a >= len(text) {
			continue
		}
		a, b = max(a, 0), min(b, len(text))
		l.Matches = append(l.Matches, GrepSpan{From: utf16Units(text[:a]), To: utf16Units(text[:b])})
	}
}

// wordAlone reports whether text[from:to] isn't part of a longer word, as `git grep -w`
// asks: the characters either side, if any, aren't letters, digits or underscores.
func wordAlone(text string, from, to int) bool {
	if from > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:from])
		if isWordRune(r) {
			return false
		}
	}
	if to < len(text) {
		r, _ := utf8.DecodeRuneInString(text[to:])
		if isWordRune(r) {
			return false
		}
	}
	return true
}

// isWordRune is git's word character: an ASCII letter, digit or underscore. Git reads bytes,
// so a letter outside ASCII is a word character too, as each of its bytes is.
func isWordRune(r rune) bool {
	return r == '_' || r >= 0x80 || ('0' <= r && r <= '9') || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

// utf16Units is the length of s in UTF-16 units, as the window counts.
func utf16Units(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// grepError turns how git grep failed into what happened. It exits 1 when nothing matched.
func grepError(err error) error {
	var ge *Error
	if !errors.As(err, &ge) {
		return err
	}
	stderr := ge.Stderr
	switch {
	case ge.ExitCode == 1 && strings.TrimSpace(stderr) == "":
		return nil
	case strings.Contains(stderr, "promisor remote"):
		return ErrNotDownloaded
	case strings.Contains(stderr, "unable to resolve revision") || strings.Contains(stderr, "invalid object name") || strings.Contains(stderr, "bad revision"):
		return ErrUnknownRevision
	}
	// fatal: -e option, 'kmalloc_array(': Unmatched ( or \(
	if _, after, ok := strings.Cut(stderr, "-e option, '"); ok {
		if i := strings.Index(after, "': "); i >= 0 {
			return &PatternError{Reason: patternReason(strings.TrimSpace(after[i+3:]))}
		}
	}
	return err
}

// patternReason puts the regular expression library's complaint into words.
func patternReason(s string) string {
	switch {
	case strings.HasPrefix(s, "Unmatched ( or"):
		return "a ( is never closed"
	case strings.HasPrefix(s, "Unmatched ) or"):
		return "a ) has no ( before it"
	case strings.HasPrefix(s, "Unmatched [") || strings.HasPrefix(s, "brackets ([ ]) not balanced"):
		return "a [ is never closed"
	case strings.HasPrefix(s, "Unmatched \\{") || strings.HasPrefix(s, "braces not balanced"):
		return "a { is never closed"
	case strings.HasPrefix(s, "Invalid preceding regular expression") || strings.HasPrefix(s, "repetition-operator operand invalid"):
		return "a *, + or ? has nothing before it to repeat"
	case strings.HasPrefix(s, "Trailing backslash"):
		return "it ends with a \\ that escapes nothing"
	case strings.HasPrefix(s, "Invalid range end"):
		return "a range in [ ] runs backwards"
	}
	return s
}
