package git

import (
	"context"
	"path"
	"strings"
)

// maxCountedFiles is how many changed files get line counts. Past this the counts stop
// being worth reading, and the list still works without them.
const maxCountedFiles = 200

// CountLines fills in line counts for the changed files. They only help scan a list, so a
// failure, as in a repository with no commits yet, just leaves them at zero.
func (r *Repo) CountLines(ctx context.Context, st *Status) {
	if len(st.Files) == 0 || len(st.Files) > maxCountedFiles {
		return
	}
	args := []string{"diff", "HEAD", "--numstat", "-z", "--"}
	for _, f := range st.Files {
		if !f.Untracked {
			args = append(args, f.Path)
		}
	}
	if args[len(args)-1] == "--" {
		return
	}
	out, err := r.Read(ctx, args...)
	if err != nil {
		return
	}
	counts := parseNumstat(out)
	for i := range st.Files {
		if c, ok := counts[st.Files[i].Path]; ok {
			st.Files[i].Added, st.Files[i].Deleted, st.Files[i].Binary = c.added, c.deleted, c.binary
		}
	}
}

// MarkCaseClashes flags changed files whose names differ from another tracked file's only by
// letter case. A case-insensitive filesystem can only hold one of them, so git sees a change
// nobody made. It looks only in the folders of the changed files, so it stays cheap on a big
// repository, and a failure just leaves nothing flagged.
func (r *Repo) MarkCaseClashes(ctx context.Context, st *Status) {
	if len(st.Files) == 0 || len(st.Files) > maxCountedFiles {
		return
	}
	ignore, err := r.ReadString(ctx, "config", "--bool", "core.ignorecase")
	if err != nil || strings.TrimSpace(ignore) != "true" {
		return
	}
	folders := map[string]bool{}
	for _, f := range st.Files {
		if !f.Untracked {
			folders[path.Dir(f.Path)] = true
		}
	}
	if len(folders) == 0 {
		return
	}
	args := []string{"--literal-pathspecs", "ls-files", "-z", "--"}
	for folder := range folders {
		args = append(args, folder)
	}
	out, err := r.ReadString(ctx, args...)
	if err != nil {
		return
	}
	names := map[string]int{}
	for name := range strings.SplitSeq(out, "\x00") {
		if name != "" {
			names[strings.ToLower(name)]++
		}
	}
	for i := range st.Files {
		st.Files[i].CaseClash = !st.Files[i].Untracked && names[strings.ToLower(st.Files[i].Path)] > 1
	}
}
