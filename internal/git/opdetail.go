package git

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// What an operation in progress is about (issue #593): which two sides, which commit is in
// hand, and where the branch goes back to if it's aborted. All of it is read from the state
// git keeps, so it's the same for an operation started in a terminal.

// writtenCap is the most rewritten commits a rebase in progress lists.
const writtenCap = 200

// mergeSubject matches the first line git prepares for a merge of one branch, tag or commit.
var mergeSubject = regexp.MustCompile(`^Merge (?:remote-tracking )?(?:branch|tag|commit) '([^']+)'`)

func readLine(gitDir, name string) string {
	b, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(name)))
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(line)
}

// nameOf names a commit the way a person would: a branch, remote branch or tag that points at
// it, or failing that its short id.
func (r *Repo) nameOf(ctx context.Context, commit string) string {
	if commit == "" {
		return ""
	}
	out, err := r.ReadString(ctx, "for-each-ref", "--points-at", commit, "--format=%(refname:short)", "refs/heads", "refs/remotes", "refs/tags")
	if err == nil && out != "" {
		name, _, _ := strings.Cut(out, "\n")
		return name
	}
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// describe fills in what st's operation is about.
func (r *Repo) describe(ctx context.Context, gitDir string, st *OperationState) {
	branch, _ := r.ReadString(ctx, "symbolic-ref", "--short", "-q", "HEAD")
	head, _ := r.ReadString(ctx, "rev-parse", "-q", "--verify", "HEAD")
	switch st.Op {
	case Rebase:
		dir := "rebase-merge"
		if _, err := os.Stat(filepath.Join(gitDir, dir)); err != nil {
			dir = "rebase-apply"
		}
		// A rebase of a detached HEAD has no branch: head-name says "detached HEAD".
		if name, ok := strings.CutPrefix(readLine(gitDir, dir+"/head-name"), "refs/heads/"); ok {
			st.Ours = name
		}
		onto := readLine(gitDir, dir+"/onto")
		st.Theirs = r.nameOf(ctx, onto)
		st.OrigHead = readLine(gitDir, dir+"/orig-head")
		st.Commit = readLine(gitDir, "REBASE_HEAD")
		if st.Commit == "" {
			st.Commit = readLine(gitDir, dir+"/stopped-sha")
		}
		if onto != "" {
			if out, err := r.ReadString(ctx, "rev-list", "--max-count="+strconv.Itoa(writtenCap), onto+"..HEAD"); err == nil && out != "" {
				st.Written = strings.Split(out, "\n")
			}
		}
	case Merge:
		st.Ours, st.OrigHead = branch, head
		st.Commit = readLine(gitDir, "MERGE_HEAD")
		// A merge asked for by id is named by the id in git's message: a ref that points there
		// reads better.
		if m := mergeSubject.FindStringSubmatch(readLine(gitDir, "MERGE_MSG")); m != nil && !strings.HasPrefix(st.Commit, m[1]) {
			st.Theirs = m[1]
		} else {
			st.Theirs = r.nameOf(ctx, st.Commit)
		}
	case CherryPick, Revert:
		st.Ours, st.OrigHead = branch, head
		// A run of several keeps where it began.
		if first := readLine(gitDir, "sequencer/head"); first != "" {
			st.OrigHead = first
		}
		if st.Op == CherryPick {
			st.Commit = readLine(gitDir, "CHERRY_PICK_HEAD")
		} else {
			st.Commit = readLine(gitDir, "REVERT_HEAD")
		}
	default:
		return
	}
	if st.Commit != "" {
		st.Subject, _ = r.ReadString(ctx, "log", "-1", "--format=%s", st.Commit, "--")
	}
}
