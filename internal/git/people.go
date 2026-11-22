package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HistoryIdentity is one name and address a history uses, as its commits have them, with
// how often.
type HistoryIdentity struct {
	Identity
	// Authored counts the commits written under it, merges included: the graph shows a
	// merge's author too. CoAuthored counts the commits that credit it in a Co-authored-by
	// or Co-developed-by trailer, each once.
	Authored   int `json:"authored"`
	CoAuthored int `json:"coAuthored"`
}

// HistoryIdentities reads every identity the history uses, with the number of commits read.
// The history is what the graph can show: every branch, remote-tracking branch and tag,
// and HEAD. Names are as written: no mailmap is applied, so the caller can apply one that
// isn't saved yet.
//
// It reads every commit, which takes about twenty seconds for the Linux kernel's 1.5
// million, so the caller keeps the answer until a ref moves.
func (r *Repo) HistoryIdentities(ctx context.Context) ([]HistoryIdentity, int, error) {
	type count struct{ authored, coAuthored int }
	seen := map[Identity]*count{}
	var order []Identity
	at := func(id Identity) *count {
		c := seen[id]
		if c == nil {
			c = &count{}
			seen[id] = c
			order = append(order, id)
		}
		return c
	}
	commits := 0
	args := []string{
		"-c", "log.showSignature=false", "-c", "i18n.logOutputEncoding=UTF-8",
		"log", "--format=%x1e%an%x1f%ae%x1f" + coAuthorTrailers,
	}
	args = append(args, r.logRevs(ctx, LogOptions{})...)
	err := r.Stream(ctx, func(rd io.Reader) error {
		return eachRecord(rd, func(rec string) {
			f := strings.SplitN(strings.Trim(rec, "\n"), "\x1f", 3)
			if len(f) != 3 {
				return
			}
			commits++
			author := Identity{Name: f[0], Email: f[1]}
			at(author).authored++
			if f[2] == "" {
				return
			}
			var credited []Identity
			for value := range strings.SplitSeq(f[2], "\x1d") {
				id := ParseIdentity(value)
				if (id.Name == "" && id.Email == "") || id == author {
					continue
				}
				dup := false
				for _, c := range credited {
					dup = dup || c == id
				}
				if !dup {
					credited = append(credited, id)
					at(id).coAuthored++
				}
			}
		})
	}, args...)
	if err != nil && !isEmptyRepoLogError(err) {
		return nil, 0, err
	}
	out := make([]HistoryIdentity, len(order))
	for i, id := range order {
		out[i] = HistoryIdentity{Identity: id, Authored: seen[id].authored, CoAuthored: seen[id].coAuthored}
	}
	return out, commits, nil
}

// The mailmap sources, in the order git reads them. A later one replaces what an earlier
// one says.
const (
	MailmapWorkTree = ".mailmap"
	MailmapBlob     = "mailmap.blob"
	MailmapFile     = "mailmap.file"
)

// MailmapSource is one place git reads a mailmap from.
type MailmapSource struct {
	// Source is MailmapWorkTree, MailmapBlob or MailmapFile.
	Source string `json:"source"`
	// Path is where it is: the file's path, or what mailmap.blob names, such as
	// "HEAD:.mailmap".
	Path string `json:"path"`
	// Exists is false for a .mailmap that hasn't been created, or a setting that points at
	// nothing.
	Exists bool `json:"exists"`
	// Editable is true for the work tree's .mailmap only.
	Editable bool `json:"editable"`
	// Lines counts its mapping lines.
	Lines int `json:"lines"`
}

// MailmapSources lists where git reads this repository's mailmap from, in the order it
// reads them: the work tree's .mailmap always, whether or not it exists, then
// mailmap.blob and mailmap.file when they're set.
func (r *Repo) MailmapSources(ctx context.Context) ([]MailmapSource, error) {
	out := []MailmapSource{{Source: MailmapWorkTree, Path: filepath.Join(r.dir, MailmapWorkTree), Editable: true}}
	if blob := r.mailmapSetting(ctx, MailmapBlob, false); blob != "" {
		out = append(out, MailmapSource{Source: MailmapBlob, Path: blob})
	}
	if file := r.mailmapSetting(ctx, MailmapFile, true); file != "" {
		if !filepath.IsAbs(file) {
			file = filepath.Join(r.dir, file) // as git reads it: from the top of the work tree
		}
		out = append(out, MailmapSource{Source: MailmapFile, Path: file})
	}
	for i := range out {
		text, exists, err := r.readMailmapSource(ctx, out[i])
		if err != nil {
			return nil, err
		}
		out[i].Exists = exists
		for _, l := range ParseMailmap(text) {
			if l.Kind == MailmapMapping {
				out[i].Lines++
			}
		}
	}
	return out, nil
}

// mailmapSetting reads one mailmap setting, "" when it isn't set.
func (r *Repo) mailmapSetting(ctx context.Context, key string, isPath bool) string {
	args := []string{"config", "--get", key}
	if isPath {
		args = []string{"config", "--type=path", "--get", key}
	}
	v, _ := r.ReadString(ctx, args...) // exit status 1 is not set
	return strings.TrimSpace(v)
}

// ReadMailmapSource returns a source's text with "\n" line endings, "" when there's
// nothing there.
func (r *Repo) ReadMailmapSource(ctx context.Context, source string) (string, error) {
	sources, err := r.MailmapSources(ctx)
	if err != nil {
		return "", err
	}
	for _, s := range sources {
		if s.Source == source {
			text, _, err := r.readMailmapSource(ctx, s)
			return text, err
		}
	}
	return "", nil
}

func (r *Repo) readMailmapSource(ctx context.Context, s MailmapSource) (text string, exists bool, err error) {
	if s.Source == MailmapBlob {
		// A name git would take for an option is no blob.
		if strings.HasPrefix(s.Path, "-") {
			return "", false, nil
		}
		out, err := r.Read(ctx, "cat-file", "blob", s.Path)
		if err != nil {
			return "", false, nil // it names nothing, which git reads as no mailmap
		}
		return normalizeMailmapText(string(out)), true, nil
	}
	if !mailmapFile(s) {
		return "", false, nil
	}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return normalizeMailmapText(string(data)), true, nil
}

// mailmapFile reports whether a source on disk is a file git would read a mailmap from.
// The work tree's .mailmap is committed with the repository, and git opens it without
// following a link (since 2.32), so a .mailmap that is a link is no mailmap: it could lead
// anywhere outside the repository. mailmap.file is the user's own setting and may be a link,
// as a file kept with their dotfiles often is, but whatever it leads to must be a file: a
// device such as /dev/zero would be read for ever.
func mailmapFile(s MailmapSource) bool {
	if s.Source == MailmapWorkTree && isLink(s.Path) {
		return false
	}
	info, err := os.Stat(s.Path)
	return err != nil || info.Mode().IsRegular() // a missing file is read as missing
}

// normalizeMailmapText gives text "\n" line endings and one final newline, or "" for none.
func normalizeMailmapText(text string) string {
	text = strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if text == "" {
		return ""
	}
	return text + "\n"
}

// ReadMailmap reads every source into one Mailmap, as git reads them. With edited set,
// text stands in for the work tree's .mailmap: what someone is editing and hasn't saved.
func (r *Repo) ReadMailmap(ctx context.Context, text string, edited bool) (*Mailmap, error) {
	sources, err := r.MailmapSources(ctx)
	if err != nil {
		return nil, err
	}
	m := NewMailmap()
	for _, s := range sources {
		if s.Source == MailmapWorkTree && edited {
			m.Add(s.Source, text)
			continue
		}
		saved, _, err := r.readMailmapSource(ctx, s)
		if err != nil {
			return nil, err
		}
		m.Add(s.Source, saved)
	}
	return m, nil
}

var (
	// ErrMailmapChanged means the .mailmap changed on disk after it was read for editing.
	ErrMailmapChanged = errors.New("the mailmap changed on disk")
	// ErrNoMailmap means the repository has no .mailmap and nobody said to create one.
	ErrNoMailmap = errors.New("the repository has no mailmap, and creating one wasn't agreed to")
)

// WriteMailmap replaces the text of the work tree's .mailmap, the only source the app
// writes. base is the text it had when editing began; if the file no longer holds it,
// nothing is written and ErrMailmapChanged is returned. The file keeps its line endings.
//
// A repository works without a mailmap, so one is never created in passing: when there's
// no .mailmap, text is written only with create set, which says the user agreed to the new
// file. Otherwise ErrNoMailmap is returned and nothing is written.
//
// A .mailmap that is a symbolic link is refused with ErrSymlink: git doesn't read one, and
// writing to it would change the file it points at, which may be outside the repository.
// Anything else that isn't a file, such as a folder, is refused too.
func (r *Repo) WriteMailmap(_ context.Context, base, text string, create bool) error {
	p := filepath.Join(r.dir, MailmapWorkTree)
	if isLink(p) {
		return fmt.Errorf("%s %w, which git doesn't read as a mailmap", MailmapWorkTree, ErrSymlink)
	}
	if info, err := os.Stat(p); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%s isn't a file", MailmapWorkTree)
	}
	lines, eol, err := readLines(p)
	if err != nil {
		return err
	}
	_, statErr := os.Stat(p)
	exists := statErr == nil
	now := ""
	if len(lines) > 0 {
		now = normalizeMailmapText(strings.Join(lines, "\n"))
	}
	if now != normalizeMailmapText(base) {
		return ErrMailmapChanged
	}
	text = normalizeMailmapText(text)
	if !exists {
		if text == "" {
			return nil // nothing to say, so still no file
		}
		if !create {
			return ErrNoMailmap
		}
	}
	if text == "" {
		return writeLines(p, nil, eol)
	}
	return writeLines(p, strings.Split(strings.TrimSuffix(text, "\n"), "\n"), eol)
}
