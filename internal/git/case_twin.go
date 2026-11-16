package git

import (
	"context"
	"strings"
)

// caseNames finds, for a name with folders ("feature/x", "src/main.c"), a known name that
// differs from it only by letter case, either as a whole or in one of its folders. Windows
// and macOS can't keep such names apart: refs and files both live on their filesystems.
type caseNames struct {
	exact  map[string]bool
	folded map[string]string // lower case → what to show for the first name added
}

func newCaseNames() caseNames {
	return caseNames{exact: map[string]bool{}, folded: map[string]string{}}
}

// add records name and each of its folders. shown is put before what twin returns, such as a
// remote's "origin/".
func (c caseNames) add(name, shown string) {
	for i := 0; i <= len(name); i++ {
		if i < len(name) && name[i] != '/' {
			continue
		}
		part := name[:i]
		if part == "" {
			continue
		}
		c.exact[part] = true
		if i < len(name) {
			part += "/" // a folder shows with its slash
		}
		if _, ok := c.folded[strings.ToLower(name[:i])]; !ok {
			c.folded[strings.ToLower(name[:i])] = shown + part
		}
	}
}

// twin returns the known name, or folder ending in "/", that differs from name or one of its
// folders only by letter case, or "" when there's none.
func (c caseNames) twin(name string) string {
	for i := 0; i <= len(name); i++ {
		if i < len(name) && name[i] != '/' {
			continue
		}
		part := name[:i]
		if part == "" || c.exact[part] {
			continue
		}
		// The first part nobody has decides it: a new folder can't clash further down.
		return c.folded[strings.ToLower(part)]
	}
	return ""
}

// BranchCaseTwin returns an existing branch, local or remote, whose name differs from name
// only by letter case, or "" if none does. A folder twin ends in "/": "Feature/y" is a twin
// of "feature/" when feature/x exists. Remote branches come back with their remote, as in
// "origin/foo". except is a local branch to leave out, the one being renamed.
func (r *Repo) BranchCaseTwin(ctx context.Context, name, except string) (string, error) {
	out, err := r.ReadString(ctx, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return "", err
	}
	known := newCaseNames()
	var remote []string
	for ref := range strings.FieldsSeq(out) {
		if b, ok := strings.CutPrefix(ref, "refs/heads/"); ok && b != except {
			known.add(b, "")
		} else if rest, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
			remote = append(remote, rest)
		}
	}
	// Local branches first: they're the likelier mix-up and the plainer thing to name.
	for _, rest := range remote {
		if from, b, ok := strings.Cut(rest, "/"); ok && b != "HEAD" {
			known.add(b, from+"/")
		}
	}
	return known.twin(name), nil
}

// markCaseTwins flags new files, untracked or added, whose path differs from a tracked one's
// only by letter case, folders included. This filesystem holds both, but Windows and macOS
// can't, so committing one breaks checkouts there. Only the new paths are looked up, and a
// failure just leaves nothing flagged.
func (r *Repo) markCaseTwins(ctx context.Context, st *Status) {
	// A staged rename or copy brings a new name too.
	isNew := func(f FileStatus) bool { return f.Untracked || f.Index == "A" || f.Index == "R" || f.Index == "C" }
	args := []string{"ls-files", "-z", "--"}
	added := map[string]bool{}
	for _, f := range st.Files {
		if isNew(f) {
			args = append(args, ":(icase,literal)"+f.Path)
			added[f.Path] = true
		}
	}
	if len(args) == 3 {
		return
	}
	files, err := r.ReadString(ctx, args...)
	if err != nil {
		return
	}
	known := newCaseNames()
	for name := range strings.SplitSeq(files, "\x00") {
		// A staged new file is in the index too, but it can't be its own twin.
		if name != "" && !added[name] {
			known.add(name, "")
		}
	}
	// Tracked folders, for a new file in Src/ beside a tracked src/. None on an unborn branch.
	if dirs, err := r.ReadString(ctx, "ls-tree", "-r", "-d", "-z", "--name-only", "HEAD"); err == nil {
		for name := range strings.SplitSeq(dirs, "\x00") {
			if name != "" {
				known.add(name, "")
			}
		}
	}
	// Two new files can be twins of each other, too.
	newFolded := map[string]int{}
	for p := range added {
		newFolded[strings.ToLower(p)]++
	}
	for i := range st.Files {
		p := st.Files[i].Path
		st.Files[i].CaseTwin = isNew(st.Files[i]) && (known.twin(p) != "" || newFolded[strings.ToLower(p)] > 1)
	}
}
