package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// ArchiveFormats are the formats git archive writes that Save as an archive offers (#936).
var ArchiveFormats = []string{"zip", "tar.gz", "tar"}

// ArchivePlan is what an archive of a commit would hold, read before it's written (#936).
type ArchivePlan struct {
	// Commit is the commit's full id.
	Commit string `json:"commit"`
	// Files is how many files go in, and Bytes their size before compressing: -1 in a partial
	// clone, where the sizes of files not downloaded aren't known.
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
	// Ignored is how many files the commit's .gitattributes leave out with export-ignore, and
	// Subst how many have export-subst placeholders filled in.
	Ignored int `json:"ignored"`
	Subst   int `json:"subst"`
	// Missing, in a partial clone, lists the files going in that were never downloaded.
	Missing []string `json:"missing"`
}

// ArchiveOptions are git archive's choices.
type ArchiveOptions struct {
	// Format is one of ArchiveFormats.
	Format string `json:"format"`
	// Prefix goes before every path inside the archive, such as "tidewater-v1.1.0/". One
	// without a trailing slash gets one, so it's always a folder.
	Prefix string `json:"prefix"`
	// Sub, when set, is the one folder to archive; its path inside is kept.
	Sub string `json:"sub"`
}

// ErrNotAFolder is returned when an archive's folder is a file, or isn't in the commit.
var ErrNotAFolder = errors.New("not a folder in that commit")

// CleanSub tidies a folder typed for an archive into git's form: forward slashes, no slashes at
// either end. "" is the whole tree. A path out of the repository is refused.
func CleanSub(sub string) (string, error) {
	s := strings.Trim(strings.ReplaceAll(strings.TrimSpace(sub), `\`, "/"), "/")
	if s == "" || s == "." {
		return "", nil
	}
	for _, part := range strings.Split(s, "/") {
		if part == ".." || part == "" {
			return "", fmt.Errorf("“%s” isn't a folder inside the repository", sub)
		}
	}
	return path.Clean(s), nil
}

// FolderPrefix is Prefix as git archive takes it: a folder, ending in a slash, or nothing.
func FolderPrefix(prefix string) string {
	p := strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(prefix), `\`, "/"), "/")
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// ArchivePlan lists what an archive of rev, or of its folder sub, would hold. Only commits and
// folder listings are read, so a partial clone answers without downloading. Attributes come
// from the commit's own .gitattributes (and the repository's info/attributes), as git archive
// takes them. On the Linux kernel (97,000 files) it takes about 3 s.
func (r *Repo) ArchivePlan(ctx context.Context, rev, sub string) (ArchivePlan, error) {
	var plan ArchivePlan
	if err := CheckRevs(rev); err != nil {
		return plan, err
	}
	sub, err := CleanSub(sub)
	if err != nil {
		return plan, err
	}
	if plan.Commit, err = r.ReadString(ctx, "rev-parse", "--verify", "-q", rev+"^{commit}"); err != nil || plan.Commit == "" {
		return plan, fmt.Errorf("“%s” isn't a commit here", rev)
	}
	if sub != "" {
		kind, err := r.ReadString(ctx, "cat-file", "-t", plan.Commit+":"+sub)
		if err != nil || kind != "tree" {
			return plan, ErrNotAFolder
		}
	}
	partial := r.Partial(ctx)
	args := []string{"ls-tree", "-r", "-z"}
	if !partial {
		// A size reads each file's header: in a partial clone that would download it.
		args = append(args, "-l")
	}
	args = append(args, plan.Commit)
	if sub != "" {
		args = append(args, "--", sub)
	}
	out, err := r.readCmd(ctx, Cmd{Args: args, Env: noLazyFetch})
	if err != nil {
		return plan, err
	}
	type file struct {
		id, path string
		size     int64
	}
	var files []file
	for _, entry := range bytes.Split(out, []byte{0}) {
		meta, name, ok := strings.Cut(string(entry), "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		// A submodule is a commit, not a file: git archive leaves its content out.
		if len(f) < 3 || f[1] != "blob" {
			continue
		}
		var size int64
		if len(f) >= 4 {
			size, _ = strconv.ParseInt(f[3], 10, 64)
		}
		files = append(files, file{f[2], name, size})
	}
	// export-ignore on a folder leaves out everything in it, so each file's folders are asked
	// about too.
	paths := map[string]bool{}
	for _, f := range files {
		for p := f.path; p != "." && p != "/" && p != ""; p = path.Dir(p) {
			if paths[p] {
				break
			}
			paths[p] = true
		}
	}
	ignored, subst, err := r.exportAttrs(ctx, plan.Commit, paths)
	if err != nil {
		return plan, err
	}
	var ids []string
	for _, f := range files {
		gone := false
		for p := f.path; p != "." && p != ""; p = path.Dir(p) {
			if ignored[p] {
				gone = true
				break
			}
		}
		if gone {
			plan.Ignored++
			continue
		}
		plan.Files++
		plan.Bytes += f.size
		if subst[f.path] {
			plan.Subst++
		}
		ids = append(ids, f.id)
	}
	if partial {
		plan.Bytes = -1
	}
	plan.Missing = []string{}
	if partial {
		if missing, err := r.missingOf(ctx, ids); err != nil {
			return plan, err
		} else if missing != nil {
			plan.Missing = missing
		}
	}
	return plan, nil
}

// exportAttrs reads which of paths have export-ignore and export-subst set, from commit's own
// attributes.
func (r *Repo) exportAttrs(ctx context.Context, commit string, paths map[string]bool) (ignored, subst map[string]bool, err error) {
	ignored, subst = map[string]bool{}, map[string]bool{}
	if len(paths) == 0 {
		return ignored, subst, nil
	}
	var in bytes.Buffer
	for p := range paths {
		in.WriteString(p)
		in.WriteByte(0)
	}
	out, err := r.readCmd(ctx, Cmd{Args: []string{"check-attr", "-z", "--stdin", "--source", commit, "export-ignore", "export-subst"}, Stdin: &in, Env: noLazyFetch})
	if err != nil {
		return nil, nil, err
	}
	// Each answer is three fields: the path, the attribute, and its value.
	fields := strings.Split(string(out), "\x00")
	for i := 0; i+2 < len(fields); i += 3 {
		if fields[i+2] != "set" {
			continue
		}
		switch fields[i+1] {
		case "export-ignore":
			ignored[fields[i]] = true
		case "export-subst":
			subst[fields[i]] = true
		}
	}
	return ignored, subst, nil
}

// Archive writes rev, or its folder o.Sub, to file as git archive does, export-ignore and
// export-subst included. In a partial clone, ArchivePlan's Missing must be downloaded first:
// archiving doesn't fetch file by file. Stopping it through ctx leaves file part-written; the
// caller removes it.
func (r *Repo) Archive(ctx context.Context, rev string, o ArchiveOptions, file string) error {
	if err := CheckRevs(rev); err != nil {
		return err
	}
	known := false
	for _, f := range ArchiveFormats {
		known = known || f == o.Format
	}
	if !known {
		return fmt.Errorf("git archive doesn't write %q", o.Format)
	}
	sub, err := CleanSub(o.Sub)
	if err != nil {
		return err
	}
	args := []string{"archive", "--format=" + o.Format, "--prefix=" + FolderPrefix(o.Prefix), "-o", file, rev}
	if sub != "" {
		// git takes a file as a path too: Sub is a folder, as ArchivePlan says.
		if kind, err := r.ReadString(ctx, "cat-file", "-t", rev+":"+sub); err != nil || kind != "tree" {
			return ErrNotAFolder
		}
		args = append(args, "--", sub)
	}
	_, err = r.readCmd(ctx, Cmd{Args: args, Env: noLazyFetch})
	return err
}
