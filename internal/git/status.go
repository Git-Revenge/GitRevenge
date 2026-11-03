package git

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// FileStatus is one changed path from `git status`.
type FileStatus struct {
	Path string `json:"path"`
	// OrigPath is the source of a staged rename or copy.
	OrigPath string `json:"origPath,omitempty"`
	// Index and WorkTree are git's XY codes: "." (unchanged), M, T, A, D, R, C or U.
	Index    string `json:"index"`
	WorkTree string `json:"workTree"`
	// Untracked files have Index and WorkTree "?".
	Untracked  bool `json:"untracked"`
	Conflicted bool `json:"conflicted"`
	Submodule  bool `json:"submodule"`
	// Added and Deleted count changed lines against HEAD, staged or not (see CountLines).
	Added   int  `json:"added"`
	Deleted int  `json:"deleted"`
	Binary  bool `json:"binary,omitempty"`
}

// Staged reports whether the file has changes in the index.
func (f FileStatus) Staged() bool { return !f.Untracked && !f.Conflicted && f.Index != "." }

// Unstaged reports whether the file has working-tree changes not yet staged.
func (f FileStatus) Unstaged() bool { return f.Untracked || f.Conflicted || f.WorkTree != "." }

// Status is the state of the work tree and index.
type Status struct {
	// Head is the branch name, or empty when detached.
	Head string `json:"head"`
	// Oid is the HEAD commit, or empty before the first commit.
	Oid      string       `json:"oid"`
	Upstream string       `json:"upstream"`
	Ahead    int          `json:"ahead"`
	Behind   int          `json:"behind"`
	Files    []FileStatus `json:"files"`
}

// Status reads the work tree and index state. It never takes the index lock, so it can
// run while the user works in a terminal.
func (r *Repo) Status(ctx context.Context) (Status, error) {
	out, err := r.Read(ctx, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	return parseStatus(out)
}

func parseStatus(out []byte) (Status, error) {
	st := Status{Files: []FileStatus{}}
	recs := bytes.Split(out, []byte{0})
	for i := 0; i < len(recs); i++ {
		rec := string(recs[i])
		if rec == "" {
			continue
		}
		switch rec[0] {
		case '#':
			parseBranchHeader(&st, rec)
		case '1':
			// 1 XY sub mH mI mW hH hI path
			f := strings.SplitN(rec, " ", 9)
			if len(f) != 9 {
				return st, fmt.Errorf("bad status record %q", rec)
			}
			st.Files = append(st.Files, entry(f[1], f[2], f[8]))
		case '2':
			// 2 XY sub mH mI mW hH hI Xscore path NUL origPath
			f := strings.SplitN(rec, " ", 10)
			if len(f) != 10 || i+1 >= len(recs) {
				return st, fmt.Errorf("bad status record %q", rec)
			}
			e := entry(f[1], f[2], f[9])
			i++
			e.OrigPath = string(recs[i])
			st.Files = append(st.Files, e)
		case 'u':
			// u XY sub m1 m2 m3 mW h1 h2 h3 path
			f := strings.SplitN(rec, " ", 11)
			if len(f) != 11 {
				return st, fmt.Errorf("bad status record %q", rec)
			}
			e := entry(f[1], f[2], f[10])
			e.Conflicted = true
			st.Files = append(st.Files, e)
		case '?':
			st.Files = append(st.Files, FileStatus{Path: rec[2:], Index: "?", WorkTree: "?", Untracked: true})
		}
	}
	return st, nil
}

func entry(xy, sub, path string) FileStatus {
	return FileStatus{Path: path, Index: xy[:1], WorkTree: xy[1:2], Submodule: sub[0] == 'S'}
}

func parseBranchHeader(st *Status, rec string) {
	f := strings.Fields(rec)
	if len(f) < 3 {
		return
	}
	switch f[1] {
	case "branch.oid":
		if f[2] != "(initial)" {
			st.Oid = f[2]
		}
	case "branch.head":
		if f[2] != "(detached)" {
			st.Head = f[2]
		}
	case "branch.upstream":
		st.Upstream = f[2]
	case "branch.ab":
		if len(f) == 4 {
			st.Ahead, _ = strconv.Atoi(strings.TrimPrefix(f[2], "+"))
			st.Behind, _ = strconv.Atoi(strings.TrimPrefix(f[3], "-"))
		}
	}
}
