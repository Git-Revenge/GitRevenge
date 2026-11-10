package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// What surrounds a conflicted file in the conflict tool (issue #597): who last changed each
// side of a conflicted place, which files merged without help, and the user's own merge tool.

// cleanFilesCap is the most cleanly merged files listed.
const cleanFilesCap = 200

// PlaceOrigin is the commit that last changed some lines of one side of a conflict.
type PlaceOrigin struct {
	Commit  string `json:"commit"`
	Author  string `json:"author"`
	Subject string `json:"subject"`
}

// conflictRevs returns the commits the three sides of a conflict come from, by the operation
// in progress: what the index calls ours (stage 2), the common ancestor (stage 1) and theirs
// (stage 3). Any of them is "" when it can't be named.
func (r *Repo) conflictRevs(ctx context.Context, gitDir string) (ours, base, theirs string) {
	op, _, _ := DetectOperation(gitDir)
	parent := func(c string) string {
		if c == "" {
			return ""
		}
		p, _ := r.ReadString(ctx, "rev-parse", "-q", "--verify", c+"^")
		return p
	}
	ours = "HEAD"
	switch op {
	case Merge:
		theirs = readLine(gitDir, "MERGE_HEAD")
		if theirs != "" {
			base, _ = r.ReadString(ctx, "merge-base", "HEAD", theirs)
		}
	case Rebase:
		// Each commit is replayed onto HEAD: its parent is where both sides started.
		theirs = readLine(gitDir, "REBASE_HEAD")
		base = parent(theirs)
	case CherryPick:
		theirs = readLine(gitDir, "CHERRY_PICK_HEAD")
		base = parent(theirs)
	case Revert:
		// A revert applies the commit backwards: its parent is the side coming in.
		base = readLine(gitDir, "REVERT_HEAD")
		theirs = parent(base)
	default:
		return "", "", ""
	}
	return ours, base, theirs
}

// PlaceOrigin names the commit that last changed lines start to end (1-based, inclusive) of
// path on one side of the conflict in progress: "ours", "base" or "theirs". It returns a zero
// PlaceOrigin when that can't be worked out, such as for lines a side doesn't have.
func (r *Repo) PlaceOrigin(ctx context.Context, gitDir, path, side string, start, end int) (PlaceOrigin, error) {
	ours, base, theirs := r.conflictRevs(ctx, gitDir)
	rev := map[string]string{"ours": ours, "base": base, "theirs": theirs}[side]
	if rev == "" || start < 1 || end < start {
		return PlaceOrigin{}, nil
	}
	out, err := r.Read(ctx, "blame", "--porcelain", "-L", fmt.Sprintf("%d,%d", start, end), rev, "--", path)
	if err != nil {
		// The file isn't there on that side, or has fewer lines: nothing to name.
		var gerr *Error
		if errors.As(err, &gerr) {
			return PlaceOrigin{}, nil
		}
		return PlaceOrigin{}, err
	}
	// Each commit's details follow its first line; keep the newest by when it was written.
	var best PlaceOrigin
	var bestTime int64 = -1
	var cur PlaceOrigin
	var curTime int64
	keep := func() {
		if cur.Commit != "" && cur.Subject != "" && curTime > bestTime {
			best, bestTime = cur, curTime
		}
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.HasPrefix(line, "\t") {
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch {
		case len(key) == 40 && isHex(key):
			keep()
			if key != cur.Commit {
				cur, curTime = PlaceOrigin{Commit: key}, 0
			}
		case key == "author":
			cur.Author = value
		case key == "author-time":
			curTime, _ = strconv.ParseInt(value, 10, 64)
		case key == "summary":
			cur.Subject = value
		}
	}
	keep()
	return best, nil
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// MergedCleanly lists the files the operation in progress changed without a conflict: what is
// staged, leaving out the files still unmerged. It's capped.
func (r *Repo) MergedCleanly(ctx context.Context) []string {
	out, err := r.Read(ctx, "diff", "--cached", "--name-only", "-z", "--diff-filter=ACDMRT")
	if err != nil {
		return nil
	}
	var files []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" && len(files) < cleanFilesCap {
			files = append(files, p)
		}
	}
	return files
}

// MergeToolName returns the user's merge.tool, or "" when none is set.
func (r *Repo) MergeToolName(ctx context.Context) string {
	name, _ := r.ReadString(ctx, "config", "--get", "merge.tool")
	return name
}

// ErrNoMergeTool means merge.tool isn't set.
var ErrNoMergeTool = errors.New("no merge tool is configured")

// RunMergeTool opens path in the user's merge tool and waits for it to close. git stages the
// file itself when the tool reports success. It never prompts, and doesn't hold the app's
// write lock: the tool can be open for as long as the user likes.
func (r *Repo) RunMergeTool(ctx context.Context, path string) error {
	tool := r.MergeToolName(ctx)
	if tool == "" {
		return ErrNoMergeTool
	}
	// mergetool takes its file as an argument, not a pathspec file: "--" keeps a name that
	// starts with a dash from reading as an option.
	_, err := r.runner.Run(ctx, Cmd{Dir: r.dir, Args: []string{"mergetool", "--no-prompt", "--tool=" + tool, "--", path}})
	return err
}
