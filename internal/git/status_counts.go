package git

import "context"

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
