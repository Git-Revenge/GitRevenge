package git

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Signature is an author or committer line.
type Signature struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	// Time is a Unix timestamp in seconds.
	Time int64 `json:"time"`
	// TZ is the offset as written by git, e.g. "+0200".
	TZ string `json:"tz"`
}

// CommitDetail is a commit's metadata and full message.
type CommitDetail struct {
	ID        string    `json:"id"`
	Parents   []string  `json:"parents"`
	Author    Signature `json:"author"`
	Committer Signature `json:"committer"`
	// Subject is the first line of the message; Body is the rest, trimmed, without the
	// trailers, which are in Trailers.
	Subject  string    `json:"subject"`
	Body     string    `json:"body"`
	Trailers []Trailer `json:"trailers"`
	Signed   bool      `json:"signed"`
}

// ReadCommit reads a commit through the persistent object reader.
func ReadCommit(o *ObjectReader, rev string) (CommitDetail, error) {
	info, data, err := o.Contents(rev)
	if err != nil {
		return CommitDetail{}, err
	}
	if info.Type != "commit" {
		return CommitDetail{}, fmt.Errorf("%s is a %s, not a commit", rev, info.Type)
	}
	c := parseCommit(data)
	c.ID = info.ID
	return c, nil
}

func parseCommit(data []byte) CommitDetail {
	c := CommitDetail{Parents: []string{}}
	header, msg, _ := bytes.Cut(data, []byte("\n\n"))
	for line := range strings.SplitSeq(string(header), "\n") {
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "parent":
			c.Parents = append(c.Parents, val)
		case "author":
			c.Author = parseSignature(val)
		case "committer":
			c.Committer = parseSignature(val)
		case "gpgsig", "gpgsig-sha256":
			c.Signed = true
		}
	}
	subject, body, _ := strings.Cut(string(msg), "\n")
	c.Subject = strings.TrimSpace(subject)
	c.Body, c.Trailers = SplitTrailers(strings.TrimSpace(body))
	if c.Trailers == nil {
		c.Trailers = []Trailer{} // JSON [] rather than null
	}
	return c
}

// parseSignature parses "Name <email> 1700000000 +0200".
func parseSignature(s string) Signature {
	var sig Signature
	lt, gt := strings.LastIndex(s, "<"), strings.LastIndex(s, ">")
	if lt < 0 || gt < lt {
		sig.Name = s
		return sig
	}
	sig.Name = strings.TrimSpace(s[:lt])
	sig.Email = s[lt+1 : gt]
	f := strings.Fields(s[gt+1:])
	if len(f) > 0 {
		sig.Time, _ = strconv.ParseInt(f[0], 10, 64)
	}
	if len(f) > 1 {
		sig.TZ = f[1]
	}
	return sig
}

// FileChange is one file changed by a commit.
type FileChange struct {
	// Status is A (added), M (modified), D (deleted), R (renamed), C (copied) or T (type change).
	Status string `json:"status"`
	Path   string `json:"path"`
	// OldPath is set for renames and copies.
	OldPath string `json:"oldPath,omitempty"`
	// Added and Deleted count changed lines. Both are 0 for a binary file, or if they
	// couldn't be read: they're only there to help scan a list.
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
	// Binary means the file has no lines to count.
	Binary bool `json:"binary,omitempty"`
}

// ChangedFiles lists the files a commit changed relative to its first parent (or, for a
// root commit, everything it added). Renames are detected.
func (r *Repo) ChangedFiles(ctx context.Context, c CommitDetail) ([]FileChange, error) {
	args := []string{"diff-tree", "-r", "-z", "--name-status", "-M", "--no-commit-id"}
	if len(c.Parents) == 0 {
		args = append(args, "--root", c.ID)
	} else {
		args = append(args, c.Parents[0], c.ID)
	}
	out, err := r.Read(ctx, args...)
	if err != nil {
		return nil, err
	}
	changes := parseNameStatus(out)
	args[slices.Index(args, "--name-status")] = "--numstat"
	if counts, err := r.Read(ctx, args...); err == nil {
		applyNumstat(changes, counts)
	}
	return changes, nil
}

// applyNumstat fills in line counts from `--numstat -z` output: ADDED TAB DELETED TAB path
// NUL, or for renames and copies ADDED TAB DELETED TAB NUL old NUL new NUL. A binary file has
// - for both counts.
func applyNumstat(changes []FileChange, out []byte) {
	byPath := make(map[string]*FileChange, len(changes))
	for i := range changes {
		byPath[changes[i].Path] = &changes[i]
	}
	f := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	for i := 0; i < len(f); i++ {
		added, rest, ok := strings.Cut(f[i], "\t")
		if !ok {
			continue
		}
		deleted, path, ok := strings.Cut(rest, "\t")
		if !ok {
			continue
		}
		if path == "" && i+2 < len(f) {
			path = f[i+2]
			i += 2
		}
		c := byPath[path]
		if c == nil {
			continue
		}
		c.Binary = added == "-" && deleted == "-"
		c.Added, _ = strconv.Atoi(added)
		c.Deleted, _ = strconv.Atoi(deleted)
	}
}

// parseNameStatus parses `--name-status -z` output: STATUS NUL path NUL, or for renames
// and copies STATUS NUL old NUL new NUL.
func parseNameStatus(out []byte) []FileChange {
	f := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	changes := []FileChange{}
	for i := 0; i < len(f); {
		status := f[i]
		if status == "" {
			i++
			continue
		}
		code := status[:1] // R100 → R
		if (code == "R" || code == "C") && i+2 < len(f) {
			changes = append(changes, FileChange{Status: code, OldPath: f[i+1], Path: f[i+2]})
			i += 3
			continue
		}
		if i+1 < len(f) {
			changes = append(changes, FileChange{Status: code, Path: f[i+1]})
		}
		i += 2
	}
	return changes
}
