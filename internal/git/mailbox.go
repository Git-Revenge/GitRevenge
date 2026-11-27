package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Applying patches with git am, previewed first (#1022).

// What a file of patches holds.
const (
	// PatchMail is patches as format-patch writes them, one or many, a list archive saves
	// and GitHub serves at a pull request's .patch: applied as commits with git am.
	PatchMail = "mail"
	// PatchDiff is a plain diff with no mail headers: applied to the files with git apply.
	PatchDiff = "diff"
)

// PatchFileKind says what a file holds: PatchMail, PatchDiff, or "" for neither. Mail starts
// with an mbox "From " line, or with From: and Subject: headers; a diff has git's diff header
// or ---/+++ lines.
func PatchFileKind(text []byte) string {
	head := text[:min(len(text), 64<<10)]
	sc := bufio.NewScanner(bytes.NewReader(head))
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	from, subject, minus := false, false, false
	for n := 0; sc.Scan(); n++ {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case n == 0 && strings.HasPrefix(line, "From "):
			return PatchMail
		case strings.HasPrefix(line, "From: "):
			from = true
		case strings.HasPrefix(line, "Subject: "):
			subject = true
		case strings.HasPrefix(line, "diff --git "):
			if from && subject {
				return PatchMail
			}
			return PatchDiff
		case strings.HasPrefix(line, "--- "):
			minus = true
			continue
		case minus && strings.HasPrefix(line, "+++ "):
			if from && subject {
				return PatchMail
			}
			return PatchDiff
		}
		minus = false
		if line == "" && from && subject {
			return PatchMail
		}
	}
	return ""
}

// NumStat is a file a patch changes, with how many lines.
type NumStat struct {
	Path    string `json:"path"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary,omitempty"`
}

// MailMessage is one message of a mailbox, as mailinfo reads it.
type MailMessage struct {
	// File is the message, split out; Patch its patch part, empty for a reply or a cover
	// letter.
	File  string `json:"-"`
	Patch string `json:"-"`
	// Author, Email, Date and Subject are mailinfo's: the subject without its [PATCH …].
	Author  string `json:"author"`
	Email   string `json:"email"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
	// Number and Total are n and m of [PATCH n/m], or 0; Version is v2's 2, or 1.
	Number  int `json:"number"`
	Total   int `json:"total"`
	Version int `json:"version"`
	// Cover is a cover letter: 0/m.
	Cover bool `json:"cover"`
	// HasPatch says the message holds a patch; Files are what it changes.
	HasPatch bool      `json:"hasPatch"`
	Files    []NumStat `json:"files"`
}

var bracket = regexp.MustCompile(`^\s*(?:(?i:re|fwd?):\s*)*\[([^\]]*)\]`)

// subjectTags reads n/m and vN from a subject's [PATCH v2 3/5].
func subjectTags(subject string) (n, m, version int) {
	version = 1
	b := bracket.FindStringSubmatch(subject)
	if b == nil {
		return 0, 0, 1
	}
	for _, tag := range strings.Fields(b[1]) {
		if a, c, ok := strings.Cut(tag, "/"); ok {
			if x, err := strconv.Atoi(a); err == nil {
				if y, err := strconv.Atoi(c); err == nil {
					n, m = x, y
				}
			}
		} else if v, ok := strings.CutPrefix(strings.ToLower(tag), "v"); ok {
			if x, err := strconv.Atoi(v); err == nil && x > 0 {
				version = x
			}
		}
	}
	return n, m, version
}

// rawSubject is a message's Subject header as written, unfolded.
func rawSubject(message []byte) string {
	var subject string
	in := false
	for line := range strings.SplitSeq(string(message), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			break
		}
		switch {
		case strings.HasPrefix(line, "Subject:"):
			subject, in = strings.TrimSpace(strings.TrimPrefix(line, "Subject:")), true
		case in && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")):
			subject += " " + strings.TrimSpace(line)
		default:
			in = false
		}
	}
	return subject
}

// ReadMailbox splits a file of patches into dir, one message a file, and reads each: who
// wrote it, when, its subject, its place in a series, and what it changes. A file that is a
// single mail without an mbox "From " line counts as one message.
func (r *Repo) ReadMailbox(ctx context.Context, file, dir string) ([]MailMessage, error) {
	if _, err := r.readCmd(ctx, Cmd{Args: []string{"mailsplit", "-b", "-o" + dir, file}}); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && !strings.Contains(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	var out []MailMessage
	for _, name := range names {
		m := MailMessage{File: filepath.Join(dir, name), Patch: filepath.Join(dir, name+".patch")}
		raw, err := os.ReadFile(m.File)
		if err != nil {
			return nil, err
		}
		info, err := r.readCmd(ctx, Cmd{Args: []string{"mailinfo", filepath.Join(dir, name+".msg"), m.Patch}, Stdin: bytes.NewReader(raw)})
		if err != nil {
			return nil, err
		}
		for line := range strings.SplitSeq(string(info), "\n") {
			k, v, _ := strings.Cut(strings.TrimRight(line, "\r"), ": ")
			switch k {
			case "Author":
				m.Author = v
			case "Email":
				m.Email = v
			case "Date":
				m.Date = v
			case "Subject":
				m.Subject = v
			}
		}
		m.Number, m.Total, m.Version = subjectTags(rawSubject(raw))
		m.Cover = m.Number == 0 && m.Total > 0
		if stat, err := os.Stat(m.Patch); err == nil && stat.Size() > 0 {
			m.HasPatch = true
			m.Files, _ = r.patchNumStat(ctx, m.Patch)
		}
		if m.Files == nil {
			m.Files = []NumStat{}
		}
		out = append(out, m)
	}
	return out, nil
}

// patchNumStat lists what a patch changes: `apply --numstat`.
func (r *Repo) patchNumStat(ctx context.Context, patch string) ([]NumStat, error) {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"apply", "--numstat", "-z", patch}})
	if err != nil {
		return nil, err
	}
	var stats []NumStat
	// -z: added TAB deleted TAB path NUL; a rename's path is "" NUL old NUL new NUL.
	fields := strings.Split(string(out), "\x00")
	for i := 0; i < len(fields); i++ {
		f := strings.SplitN(fields[i], "\t", 3)
		if len(f) < 3 {
			continue
		}
		s := NumStat{Path: f[2]}
		if f[2] == "" && i+2 < len(fields) {
			s.Path = fields[i+2]
			i += 2
		}
		if f[0] == "-" {
			s.Binary = true
		} else {
			s.Added, _ = strconv.Atoi(f[0])
			s.Deleted, _ = strconv.Atoi(f[1])
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// How a patch tried on a throwaway index went.
const (
	TryApplies   = "applies"   // as it is
	TryMerges    = "merges"    // only by a three-way merge, cleanly
	TryConflicts = "conflicts" // a three-way merge left files unmerged
	TryApplied   = "applied"   // it doesn't apply, but its reverse does: it's in already
	TryFails     = "fails"     // none of those
)

// TryResult is how one patch went.
type TryResult struct {
	Status string `json:"status"`
	// Files are the ones a conflict left unmerged.
	Files []string `json:"files,omitempty"`
	// Error is git's first word on why it failed.
	Error string `json:"error,omitempty"`
}

// PatchTrial tries patches one after another on a throwaway copy of the index, made from
// HEAD: the working tree and the real index stay as they are.
type PatchTrial struct {
	r     *Repo
	index string
}

// StartTrial makes the throwaway index, at dir/index, from HEAD.
func (r *Repo) StartTrial(ctx context.Context, dir string) (*PatchTrial, error) {
	t := &PatchTrial{r: r, index: filepath.Join(dir, "index")}
	_ = os.Remove(t.index)
	if _, err := r.readCmd(ctx, t.cmd("read-tree", "HEAD")); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *PatchTrial) cmd(args ...string) Cmd {
	return Cmd{Args: args, Env: append([]string{"GIT_INDEX_FILE=" + t.index}, noLazyFetch...)}
}

// Try applies a patch to the throwaway index: as it is, or else by merging. One that only
// applies in reverse is in already. A conflict leaves the index unmerged, so the patches after
// it can't be tried until it's resolved.
func (t *PatchTrial) Try(ctx context.Context, patch string) TryResult {
	if _, err := t.r.readCmd(ctx, t.cmd("apply", "--cached", patch)); err == nil {
		return TryResult{Status: TryApplies}
	}
	if _, err := t.r.readCmd(ctx, t.cmd("apply", "--cached", "-R", "--check", patch)); err == nil {
		return TryResult{Status: TryApplied}
	}
	_, err := t.r.readCmd(ctx, t.cmd("apply", "--cached", "--3way", patch))
	if err == nil {
		return TryResult{Status: TryMerges}
	}
	if out, uerr := t.r.readCmd(ctx, t.cmd("ls-files", "-u", "-z")); uerr == nil && len(out) > 0 {
		var files []string
		for entry := range strings.SplitSeq(string(out), "\x00") {
			if _, path, ok := strings.Cut(entry, "\t"); ok && !slices.Contains(files, path) {
				files = append(files, path)
			}
		}
		return TryResult{Status: TryConflicts, Files: files}
	}
	return TryResult{Status: TryFails, Error: firstLine(err)}
}

// firstLine is git's first line of complaint.
func firstLine(err error) string {
	var gerr *Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		line, _, _ := strings.Cut(strings.TrimSpace(gerr.Stderr), "\n")
		return strings.TrimPrefix(line, "error: ")
	}
	return err.Error()
}

// PatchCommitIDs maps the patch IDs of rev's last n commits, merges left out, to the commits:
// to say which commit a patch that's in already came in as. Patch IDs ignore line numbers and
// whitespace, so a patch matches its commit wherever the lines moved to.
func (r *Repo) PatchCommitIDs(ctx context.Context, rev string, n int) (map[string]string, error) {
	if err := CheckRevs(rev); err != nil {
		return nil, err
	}
	diffs, err := r.readCmd(ctx, Cmd{Args: []string{"log", "-p", "--no-merges", "--format=commit %H", "-n", strconv.Itoa(n), rev}, Env: noLazyFetch})
	if err != nil {
		return nil, err
	}
	return r.patchIDs(ctx, diffs)
}

// PatchID is a patch file's patch ID.
func (r *Repo) PatchID(ctx context.Context, patch string) (string, error) {
	text, err := os.ReadFile(patch)
	if err != nil {
		return "", err
	}
	ids, err := r.patchIDs(ctx, append([]byte("commit 0000000000000000000000000000000000000000\n"), text...))
	if err != nil {
		return "", err
	}
	for id := range ids {
		return id, nil
	}
	return "", nil
}

func (r *Repo) patchIDs(ctx context.Context, diffs []byte) (map[string]string, error) {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"patch-id", "--stable"}, Stdin: bytes.NewReader(diffs)})
	if err != nil {
		return nil, err
	}
	ids := map[string]string{}
	for line := range strings.SplitSeq(string(out), "\n") {
		if id, commit, ok := strings.Cut(strings.TrimSpace(line), " "); ok {
			ids[id] = commit
		}
	}
	return ids, nil
}

// HasStaged reports whether the index differs from HEAD: git am needs it not to.
func (r *Repo) HasStaged(ctx context.Context) bool {
	_, err := r.Read(ctx, "diff", "--cached", "--quiet")
	return err != nil
}

// Am applies a mailbox's patches as commits with `git am --3way`, keeping each one's author,
// date and message, and adding Signed-off-by with signoff. A conflict stops it, as a rebase
// stops: ErrConflicts.
func (r *Repo) Am(ctx context.Context, mbox string, signoff bool) error {
	args := []string{"am", "--3way"}
	if signoff {
		args = append(args, "--signoff")
	}
	return r.conflictAware(ctx, Cmd{Args: append(args, mbox), Env: []string{"GIT_EDITOR=true"}})
}
