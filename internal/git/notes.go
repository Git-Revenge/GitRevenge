package git

import (
	"context"
	"errors"
	"strings"
)

// Note is a git note attached to a commit, from one notes ref.
type Note struct {
	// Ref is the full notes ref, such as refs/notes/commits.
	Ref  string `json:"ref"`
	Text string `json:"text"`
}

// DefaultNotesRef is where git notes add writes unless told otherwise.
const DefaultNotesRef = "refs/notes/commits"

// Notes returns the commit's notes from every notes ref, the default ref first.
func (r *Repo) Notes(ctx context.Context, commit string) ([]Note, error) {
	out, err := r.ReadString(ctx, "for-each-ref", "--format=%(refname)", "refs/notes/")
	if err != nil {
		return nil, err
	}
	refs := strings.Fields(out)
	notes := []Note{}
	for _, ref := range refs {
		text, err := r.ReadString(ctx, "notes", "--ref="+ref, "show", commit)
		var gerr *Error
		if errors.As(err, &gerr) && strings.Contains(strings.ToLower(gerr.Stderr), "no note found") {
			continue
		}
		if err != nil {
			return nil, err
		}
		n := Note{Ref: ref, Text: strings.TrimRight(text, "\n")}
		if ref == DefaultNotesRef {
			notes = append([]Note{n}, notes...)
		} else {
			notes = append(notes, n)
		}
	}
	return notes, nil
}

// NotedCommits lists every commit that has a note in any notes ref, each once. It asks each
// ref for its whole list in one call, so the cost follows the number of notes refs, not the
// number of commits.
func (r *Repo) NotedCommits(ctx context.Context) ([]string, error) {
	out, err := r.ReadString(ctx, "for-each-ref", "--format=%(refname)", "refs/notes/")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, ref := range strings.Fields(out) {
		list, err := r.ReadString(ctx, "notes", "--ref="+ref, "list")
		if err != nil {
			return nil, err
		}
		// Each line is the note's own object, then the object it's attached to.
		for line := range strings.Lines(list) {
			_, id, ok := strings.Cut(strings.TrimSpace(line), " ")
			if ok && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}

// SetNote adds or replaces the commit's note in ref, or removes it when text is empty.
func (r *Repo) SetNote(ctx context.Context, ref, commit, text string) error {
	if !strings.HasPrefix(ref, "refs/notes/") {
		ref = "refs/notes/" + ref
	}
	if strings.TrimSpace(text) == "" {
		_, err := r.Write(ctx, Cmd{Args: []string{"notes", "--ref=" + ref, "remove", "--ignore-missing", commit}})
		return err
	}
	_, err := r.Write(ctx, Cmd{Args: []string{"notes", "--ref=" + ref, "add", "-f", "-F", "-", commit}, Stdin: strings.NewReader(text)})
	return err
}

// notesMirror is where a remote's notes refs are fetched to, so fetching never overwrites
// notes written here: refs/notes-remote/<remote>/commits mirrors <remote>'s
// refs/notes/commits. Local refs then move only by fast-forward (IntegrateNotes).
func notesMirror(remote string) string { return "refs/notes-remote/" + remote + "/" }

// FetchNotes fetches remote's notes refs into their mirror under refs/notes-remote/. The
// mirror is forced: it's a copy of the remote, not a place anyone writes.
func (r *Repo) FetchNotes(ctx context.Context, remote string, o NetOptions) error {
	c := o.cmd("fetch", "--progress", "--prune", "--no-tags", remote, "+refs/notes/*:"+notesMirror(remote)+"*")
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}

// IntegrateNotes brings each notes ref fetched from remote into refs/notes/: created when
// missing here, fast-forwarded when behind. A ref that has notes written here which the
// remote doesn't have, and the remote has notes this one doesn't, has diverged: it's
// left alone and returned, since merging notes is for the user to choose.
func (r *Repo) IntegrateNotes(ctx context.Context, remote string) (diverged []string, err error) {
	mirror := notesMirror(remote)
	out, err := r.ReadString(ctx, "for-each-ref", "--format=%(refname) %(objectname)", mirror)
	if err != nil {
		return nil, err
	}
	for line := range strings.Lines(out) {
		name, theirs, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		local := "refs/notes/" + strings.TrimPrefix(name, mirror)
		ours, rerr := r.ReadString(ctx, "rev-parse", "--verify", "-q", local)
		ours = strings.TrimSpace(ours)
		switch {
		case rerr != nil || ours == "":
			// Not here yet.
		case ours == theirs:
			continue
		case r.isAncestor(ctx, theirs, ours):
			continue // ours is ahead; push sends it
		case !r.isAncestor(ctx, ours, theirs):
			diverged = append(diverged, local)
			continue
		}
		args := []string{"update-ref", "-m", "notes: sync from " + remote, local, theirs}
		if ours != "" {
			args = append(args, ours) // only if it hasn't moved meanwhile
		}
		if _, err := r.Write(ctx, Cmd{Args: args}); err != nil {
			return diverged, err
		}
	}
	return diverged, nil
}

// isAncestor reports whether a is an ancestor of b (or the same commit).
func (r *Repo) isAncestor(ctx context.Context, a, b string) bool {
	_, err := r.Read(ctx, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// PushNotes pushes every notes ref to remote, without forcing: a ref the remote has moved
// on from is rejected rather than overwritten. Refs already up to date send nothing.
func (r *Repo) PushNotes(ctx context.Context, remote string, o NetOptions) error {
	if out, err := r.ReadString(ctx, "for-each-ref", "--count=1", "--format=%(refname)", "refs/notes/"); err != nil || strings.TrimSpace(out) == "" {
		return err
	}
	c := o.cmd("push", "--progress", "--", remote, "refs/notes/*:refs/notes/*")
	c.Dir = r.dir
	_, err := r.runner.Run(ctx, c)
	return err
}
