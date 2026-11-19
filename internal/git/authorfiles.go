package git

import (
	"context"
	"io"
	"strconv"
	"strings"
	"sync"
)

// person is one person as git log is asked for them: the names and emails that are all
// theirs, and the arguments that narrow a log to commits that might be.
type person struct {
	emails, names map[string]bool
	// authorArgs match their commits by the author line, grepArgs the commits whose message
	// mentions them, each as fixed strings without regard to case. Both match more than is
	// theirs (any author line or message holding the text), so every hit is checked with is.
	authorArgs, grepArgs []string
}

// personOf gathers ids into one person. An identity is known by its email, or by its name
// when it has no email.
func personOf(ids []Identity) person {
	p := person{emails: map[string]bool{}, names: map[string]bool{}}
	for _, id := range ids {
		switch {
		case id.Email != "":
			if !p.emails[strings.ToLower(id.Email)] {
				p.emails[strings.ToLower(id.Email)] = true
				p.authorArgs = append(p.authorArgs, "--author=<"+id.Email+">")
				p.grepArgs = append(p.grepArgs, "--grep=<"+id.Email+">")
			}
		case id.Name != "":
			if !p.names[strings.ToLower(id.Name)] {
				p.names[strings.ToLower(id.Name)] = true
				p.authorArgs = append(p.authorArgs, "--author="+id.Name+" <")
				p.grepArgs = append(p.grepArgs, "--grep="+id.Name)
			}
		}
	}
	return p
}

// is reports whether id is one of the person's identities.
func (p person) is(id Identity) bool {
	if id.Email != "" {
		return p.emails[strings.ToLower(id.Email)]
	}
	return p.names[strings.ToLower(id.Name)]
}

// credits reports whether one of the co-author trailer values, as AuthorLog gives them,
// names the person.
func (p person) credits(coAuthors string) bool {
	for value := range strings.SplitSeq(coAuthors, "\x1d") {
		if value != "" && p.is(ParseIdentity(value)) {
			return true
		}
	}
	return false
}

// AuthorFilesCommit is one of a person's commits with the files it changed.
type AuthorFilesCommit struct {
	SHA     string
	Subject string
	// Committed is the committer date, Unix seconds.
	Committed int64
	// Paths are the files the commit added, changed or removed, with forward slashes. A
	// moved file is both: the path it left and the path it came to.
	Paths []string
}

// AuthorFiles calls each for every commit in revs that the person behind ids wrote or is
// credited on, with the files it changed, in no particular order. Merges are left out. With
// since above 0, only commits made at or after it (Unix seconds) are read. revs are refs and
// ranges as git log takes them, and a commit several of them reach is given once.
//
// Only that person's commits are compared with their parents, which is what keeps this quick
// on a long history: the files of everyone's commits would take minutes to read.
func (r *Repo) AuthorFiles(ctx context.Context, mailmapAt string, since int64, revs []string, ids []Identity, each func(AuthorFilesCommit)) error {
	if err := CheckRevs(append([]string{mailmapAt}, revs...)...); err != nil {
		return err
	}
	who := personOf(ids)
	if len(who.authorArgs) == 0 {
		return nil
	}
	stdin := strings.Join(revs, "\n") + "\n"
	var mu sync.Mutex
	run := func(match []string, wanted func(author Identity, coAuthors string) bool) error {
		// Names come as they are, not quoted, and a move isn't looked for: it would cost a
		// comparison of every added file with every removed one.
		args := append(authorPinned(mailmapAt), "-c", "core.quotePath=false", "log", "--no-merges", "--fixed-strings", "--regexp-ignore-case",
			"--name-only", "--no-renames", "--no-textconv", "--no-ext-diff",
			"--format=%x1e%H%x1f%aN%x1f%aE%x1f%ct%x1f%s%x1f"+coAuthorTrailers+"%x1c")
		if since > 0 {
			args = append(args, "--since="+strconv.FormatInt(since, 10))
		}
		args = append(append(args, match...), "--stdin")
		err := r.streamCmd(ctx, Cmd{Args: args, Stdin: strings.NewReader(stdin)}, func(rd io.Reader) error {
			return eachRecord(rd, func(rec string) {
				head, files, ok := strings.Cut(rec, "\x1c")
				if !ok {
					return
				}
				f := strings.SplitN(strings.Trim(head, "\n"), "\x1f", 6)
				if len(f) != 6 || !wanted(Identity{Name: f[1], Email: f[2]}, f[5]) {
					return
				}
				at, _ := strconv.ParseInt(f[3], 10, 64)
				c := AuthorFilesCommit{SHA: f[0], Subject: f[4], Committed: at}
				for line := range strings.SplitSeq(files, "\n") {
					if line = strings.TrimRight(line, "\r"); line != "" {
						c.Paths = append(c.Paths, line)
					}
				}
				mu.Lock()
				defer mu.Unlock()
				each(c)
			})
		})
		if err != nil && !isEmptyRepoLogError(err) {
			return err
		}
		return nil
	}
	// Two walks, side by side: what they wrote, and what somebody else wrote and credits them
	// on. No commit is in both.
	errs := make(chan error, 2)
	go func() {
		errs <- run(who.authorArgs, func(author Identity, _ string) bool { return who.is(author) })
	}()
	go func() {
		errs <- run(who.grepArgs, func(author Identity, coAuthors string) bool { return !who.is(author) && who.credits(coAuthors) })
	}()
	var first error
	for range 2 {
		if err := <-errs; err != nil && first == nil {
			first = err
		}
	}
	return first
}
