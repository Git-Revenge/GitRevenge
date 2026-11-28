package git

import (
	"bytes"
	"context"
	"slices"
	"strconv"
	"strings"
)

// FileVersion is one version of a file on the blame view's track (#1078): a commit that
// changed it, as git log -- <file> lists them.
type FileVersion struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Time    int64  `json:"time"`
	Subject string `json:"subject"`
	// Added and Removed count the lines the commit changed in the file, against its first
	// parent. Both are 0 for a merge, which git log shows no diff for, and for a binary
	// version, which Binary marks.
	Added   int  `json:"added"`
	Removed int  `json:"removed"`
	Binary  bool `json:"binary,omitempty"`
	// Merge marks a merge commit: its new lines may come from another parent, not from it.
	Merge bool `json:"merge,omitempty"`
}

// MaxFileVersions caps how many versions Versions returns: the newest ones.
const MaxFileVersions = 5000

// Versions lists the commits that changed path in rev's history (HEAD for ""), oldest first.
// It follows no renames: --follow took 203 s on the Linux kernel's mm/slub.c, against 3 s
// without, so the track stops where the file got its name (see RenamedAt). It downloads
// nothing.
func (r *Repo) Versions(ctx context.Context, path, rev string) ([]FileVersion, error) {
	if err := CheckRevs(rev); err != nil {
		return nil, err
	}
	if rev == "" {
		rev = "HEAD"
	}
	out, err := r.readCmd(ctx, Cmd{
		Args: []string{
			"--literal-pathspecs", "log", "--numstat", "--no-renames",
			"--max-count=" + strconv.Itoa(MaxFileVersions),
			"--format=%x1e%H%x1f%P%x1f%aN%x1f%at%x1f%s",
			rev, "--", path,
		},
		Env: noLazyFetch,
	})
	if isEmptyRepoLogError(err) {
		return []FileVersion{}, nil
	}
	if err != nil {
		return nil, err
	}
	versions := parseVersions(out)
	slices.Reverse(versions)
	return versions, nil
}

// parseVersions reads records of: RS sha US parents US author US time US subject, then numstat lines
// (added TAB removed TAB path, or - TAB - TAB path for a binary file).
func parseVersions(out []byte) []FileVersion {
	versions := []FileVersion{}
	for rec := range bytes.SplitSeq(out, []byte{0x1e}) {
		lines := strings.Split(strings.TrimSpace(string(rec)), "\n")
		h := strings.SplitN(lines[0], "\x1f", 5)
		if len(h) != 5 {
			continue
		}
		t, _ := strconv.ParseInt(h[3], 10, 64)
		v := FileVersion{ID: h[0], Author: h[2], Time: t, Subject: h[4], Merge: len(strings.Fields(h[1])) > 1}
		for _, l := range lines[1:] {
			f := strings.SplitN(l, "\t", 3)
			if len(f) != 3 {
				continue
			}
			if f[0] == "-" && f[1] == "-" {
				v.Binary = true
				continue
			}
			a, _ := strconv.Atoi(f[0])
			d, _ := strconv.Atoi(f[1])
			v.Added += a
			v.Removed += d
		}
		versions = append(versions, v)
	}
	return versions
}

// Uncommitted reports whether path in the work tree differs from HEAD's version.
func (r *Repo) Uncommitted(ctx context.Context, path string) (bool, error) {
	out, err := r.readCmd(ctx, Cmd{Args: []string{"--literal-pathspecs", "diff", "--name-only", "HEAD", "--", path}, Env: noLazyFetch})
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}
