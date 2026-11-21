package git

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
)

// The Map's Authors mode asks who wrote what. Like Insights, it reads names through the
// .mailmap committed at one branch and never through the user's own mailmap file, so two
// people looking at the same repository see the same people.

// authorPinned returns the -c overrides the author queries run with.
func authorPinned(mailmapAt string) []string {
	return []string{
		"-c", "log.mailmap=true",
		"-c", "mailmap.file=",
		"-c", "mailmap.blob=" + mailmapAt + ":.mailmap",
		"-c", "log.showSignature=false",
		"-c", "i18n.logOutputEncoding=UTF-8",
	}
}

// coAuthorTrailers picks out the trailers that credit someone besides the author: git's and GitHub's
// Co-authored-by, and the Linux kernel's Co-developed-by.
const coAuthorTrailers = "%(trailers:key=Co-authored-by,key=Co-developed-by,valueonly,separator=%x1d)"

// AuthorCommit is one commit as the Authors mode counts it.
type AuthorCommit struct {
	SHA string
	// Name and Email are the author's, as the committed .mailmap resolves them. The author
	// field is read, never the committer: whoever applied, rebased or merged the work isn't
	// who wrote it.
	Name, Email string
	// Committed is the committer date, Unix seconds: when the commit landed where it is.
	Committed int64
	// CoAuthors are the values of its Co-authored-by and Co-developed-by trailers, as written
	// ("Name <email>"), one after another with \x1d between them; "" when it has none.
	CoAuthors string
}

// AuthorLog calls each for every commit in revs that isn't a merge, newest first. revs are
// what git log takes: a ref for everything it reaches, "a..b" for what b has that a doesn't.
// With since above 0, only commits made at or after it (Unix seconds) are read, and the walk
// goes no further back. Names are resolved through the .mailmap committed at mailmapAt.
func (r *Repo) AuthorLog(ctx context.Context, mailmapAt string, since int64, revs []string, each func(AuthorCommit)) error {
	if err := CheckRevs(append([]string{mailmapAt}, revs...)...); err != nil {
		return err
	}
	args := append(authorPinned(mailmapAt), "log", "--no-merges", "--format=%x1e%H%x1f%aN%x1f%aE%x1f%ct%x1f"+coAuthorTrailers)
	if since > 0 {
		args = append(args, "--since="+strconv.FormatInt(since, 10))
	}
	args = append(append(args, revs...), "--")
	err := r.Stream(ctx, func(rd io.Reader) error {
		return eachRecord(rd, func(rec string) {
			f := strings.SplitN(strings.Trim(rec, "\n"), "\x1f", 5)
			if len(f) != 5 {
				return
			}
			at, _ := strconv.ParseInt(f[3], 10, 64)
			each(AuthorCommit{SHA: f[0], Name: f[1], Email: f[2], Committed: at, CoAuthors: strings.TrimSpace(f[4])})
		})
	}, args...)
	if err != nil && !isEmptyRepoLogError(err) {
		return err
	}
	return nil
}

// eachRecord calls each with every \x1e-separated record of rd.
func eachRecord(rd io.Reader, each func(rec string)) error {
	br := bufio.NewReaderSize(rd, 64*1024)
	for {
		rec, err := br.ReadString('\x1e')
		if rec = strings.TrimSuffix(rec, "\x1e"); rec != "" {
			each(rec)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// ParseIdentity reads "Name <email>", as a trailer or a mailmap line gives it. Either part
// may be missing.
func ParseIdentity(s string) Identity {
	s = strings.TrimSpace(s)
	open := strings.LastIndex(s, "<")
	if open < 0 {
		return Identity{Name: s}
	}
	email := s[open+1:]
	if shut := strings.Index(email, ">"); shut >= 0 {
		email = email[:shut]
	}
	return Identity{Name: strings.TrimSpace(s[:open]), Email: strings.TrimSpace(email)}
}

// MailmapIdentities returns each identity as the .mailmap committed at mailmapAt resolves it,
// in the same order. It's for names that don't come from an author field, such as a
// co-author's trailer, which git log doesn't map. An identity with no email can't be looked
// up and comes back as it is.
func (r *Repo) MailmapIdentities(ctx context.Context, mailmapAt string, ids []Identity) ([]Identity, error) {
	if err := CheckRevs(mailmapAt); err != nil {
		out := make([]Identity, len(ids))
		copy(out, ids)
		return out, err
	}
	return r.checkMailmap(ctx, []string{"-c", "mailmap.file=", "-c", "mailmap.blob=" + mailmapAt + ":.mailmap"}, ids)
}

// AuthorHistory is how much one person has written over a repository's whole history.
type AuthorHistory struct {
	// Authored counts the commits they're the author of; CoAuthored the ones that credit them
	// in a trailer and someone else wrote. Merges count for neither.
	Authored   int `json:"authored"`
	CoAuthored int `json:"coAuthored"`
	// First and Last are when their earliest and latest such commits were made, Unix seconds;
	// 0 when there are none.
	First int64 `json:"first"`
	Last  int64 `json:"last"`
}

// AuthorHistory counts every commit reachable from revs that the person behind ids wrote or
// is credited on. ids are the names and emails that are all one person; an identity is
// matched by its email, or by its name when it has no email.
func (r *Repo) AuthorHistory(ctx context.Context, mailmapAt string, revs []string, ids []Identity) (AuthorHistory, error) {
	var out AuthorHistory
	if err := CheckRevs(append([]string{mailmapAt}, revs...)...); err != nil {
		return out, err
	}
	who := personOf(ids)
	if len(who.authorArgs) == 0 {
		return out, nil
	}
	authorArgs, grepArgs, theirs := who.authorArgs, who.grepArgs, who.is
	stdin := strings.Join(revs, "\n") + "\n"
	run := func(match []string, each func(f []string)) error {
		args := append(authorPinned(mailmapAt), "log", "--no-merges", "--fixed-strings", "--regexp-ignore-case", "--format=%x1e%aN%x1f%aE%x1f%ct%x1f"+coAuthorTrailers)
		args = append(append(args, match...), "--stdin")
		err := r.streamCmd(ctx, Cmd{Args: args, Stdin: strings.NewReader(stdin)}, func(rd io.Reader) error {
			return eachRecord(rd, func(rec string) {
				if f := strings.SplitN(strings.Trim(rec, "\n"), "\x1f", 4); len(f) == 4 {
					each(f)
				}
			})
		})
		if err != nil && !isEmptyRepoLogError(err) {
			return err
		}
		return nil
	}
	// Two walks of the whole history, side by side: each keeps its own count and dates.
	type span struct {
		n           int
		first, last int64
	}
	count := func(s *span, stamp string) {
		at, _ := strconv.ParseInt(stamp, 10, 64)
		s.n++
		if s.first == 0 || at < s.first {
			s.first = at
		}
		s.last = max(s.last, at)
	}
	var wrote, credited span
	errs := make(chan error, 2)
	// What they wrote. The pattern is matched anywhere in the author line, so each hit is
	// checked against the identities.
	go func() {
		errs <- run(authorArgs, func(f []string) {
			if theirs(Identity{Name: f[0], Email: f[1]}) {
				count(&wrote, f[2])
			}
		})
	}()
	// What credits them. The pattern is matched anywhere in the message, so only a trailer
	// that names them counts, on a commit somebody else wrote.
	go func() {
		errs <- run(grepArgs, func(f []string) {
			if theirs(Identity{Name: f[0], Email: f[1]}) {
				return
			}
			for value := range strings.SplitSeq(f[3], "\x1d") {
				if value != "" && theirs(ParseIdentity(value)) {
					count(&credited, f[2])
					return
				}
			}
		})
	}()
	for range 2 {
		if err := <-errs; err != nil {
			return AuthorHistory{}, err
		}
	}
	out.Authored, out.CoAuthored = wrote.n, credited.n
	out.First, out.Last = wrote.first, max(wrote.last, credited.last)
	if credited.first != 0 && (out.First == 0 || credited.first < out.First) {
		out.First = credited.first
	}
	return out, nil
}
