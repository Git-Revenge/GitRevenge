package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Saving commits as patches with format-patch (#1021).

// PatchRevs names the commits to save as patches: since..tip, the last Last commits up to
// tip, or these IDs, oldest first. Merges are left out, as format-patch leaves them out.
type PatchRevs struct {
	Since string   `json:"since"`
	Tip   string   `json:"tip"`
	Last  int      `json:"last"`
	IDs   []string `json:"ids"`
}

// maxPatchIDs is how many commits named one by one fit on a command line on Windows, whose
// limit is 32,767 characters, with room to spare for SHA-256's longer IDs.
const maxPatchIDs = 400

// ErrTooManyPatches means more commits were named one by one than fit on a command line.
var ErrTooManyPatches = errors.New("too many commits named one by one")

// revs are the revisions format-patch takes for p.
func (p PatchRevs) revs() ([]string, error) {
	switch {
	case len(p.IDs) > maxPatchIDs:
		return nil, ErrTooManyPatches
	case len(p.IDs) > 0:
		if err := CheckRevs(p.IDs...); err != nil {
			return nil, err
		}
		return append([]string{"--no-walk"}, p.IDs...), nil
	case p.Tip == "":
		return nil, errors.New("no commits named")
	case p.Since == "":
		if err := CheckRevs(p.Tip); err != nil {
			return nil, err
		}
		return []string{"-" + strconv.Itoa(max(p.Last, 1)), p.Tip}, nil
	}
	if err := CheckRevs(p.Since, p.Tip); err != nil {
		return nil, err
	}
	return []string{p.Since + ".." + p.Tip}, nil
}

// PatchCommit is a commit a series of patches holds.
type PatchCommit struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
	// First is its first parent, or "" for a first commit.
	First string `json:"first"`
	// Edge marks a shallow clone's edge commit: its parent isn't here, so its patch would add
	// every file.
	Edge bool `json:"edge,omitempty"`
}

// PatchList lists the commits p names, oldest first, without merges, and how many merges it
// leaves out. Named one by one, they keep the order given.
func (r *Repo) PatchList(ctx context.Context, p PatchRevs) (commits []PatchCommit, merges int, err error) {
	var args []string
	switch {
	case len(p.IDs) > 0:
		if err := CheckRevs(p.IDs...); err != nil {
			return nil, 0, err
		}
		args = append([]string{"--no-walk=unsorted"}, p.IDs...)
	case p.Tip == "":
		return nil, 0, errors.New("no commits named")
	case p.Since == "":
		if err := CheckRevs(p.Tip); err != nil {
			return nil, 0, err
		}
		// As format-patch -N does: the newest N that aren't merges, oldest first.
		args = []string{"--no-merges", "-n", strconv.Itoa(max(p.Last, 1)), "--reverse", p.Tip}
	default:
		if err := CheckRevs(p.Since, p.Tip); err != nil {
			return nil, 0, err
		}
		args = []string{"--reverse", p.Since + ".." + p.Tip}
	}
	out, err := r.readCmd(ctx, Cmd{Args: append([]string{"log", "-z", "--format=%H%x1f%P%x1f%an%x1f%s"}, args...), Env: noLazyFetch})
	if err != nil {
		return nil, 0, err
	}
	edge := r.shallowSet(ctx)
	for rec := range strings.SplitSeq(string(out), "\x00") {
		f := strings.Split(strings.TrimSpace(rec), "\x1f")
		if len(f) < 4 {
			continue
		}
		parents := strings.Fields(f[1])
		if len(parents) > 1 {
			merges++
			continue
		}
		c := PatchCommit{ID: f[0], Subject: f[3], Author: f[2], Edge: edge[f[0]]}
		if len(parents) == 1 {
			c.First = parents[0]
		}
		commits = append(commits, c)
	}
	return commits, merges, nil
}

// PatchNeeds lists the file versions p's patches read that a partial clone hasn't got: both
// sides of each file they change. Elsewhere it's nil.
func (r *Repo) PatchNeeds(ctx context.Context, p PatchRevs) ([]string, error) {
	if !r.Partial(ctx) {
		return nil, nil
	}
	revs, err := p.revs()
	if err != nil {
		return nil, err
	}
	if p.Since == "" && len(p.IDs) == 0 {
		// log takes the last N as -n N, where format-patch takes -N.
		revs = []string{"--no-merges", "-n", strings.TrimPrefix(revs[0], "-"), revs[1]}
	}
	ids, err := r.versionsOf(ctx, revs)
	if err != nil {
		return nil, err
	}
	return r.missingOf(ctx, ids)
}

// PatchOptions are format-patch's choices.
type PatchOptions struct {
	// Prefix is the subject's prefix, PATCH when "": "RFC PATCH", "PATCH net-next".
	Prefix string `json:"prefix"`
	// Version above 1 is -v: "PATCH v2", and v2- before each file's name.
	Version int `json:"version"`
	// Numbered numbers a series of more than one, [PATCH 2/3], as git does by default.
	Numbered bool `json:"numbered"`
	// Cover writes a cover letter first, with CoverSubject and CoverText in place of the
	// placeholders git leaves in it.
	Cover        bool   `json:"cover"`
	CoverSubject string `json:"coverSubject"`
	CoverText    string `json:"coverText"`
	// Signoff adds a Signed-off-by line, as -s does.
	Signoff bool `json:"signoff"`
	// Base adds a base-commit line naming the commit they apply to.
	Base string `json:"base"`
}

func (o PatchOptions) args(count int) ([]string, error) {
	prefix := strings.TrimSpace(o.Prefix)
	if prefix == "" {
		prefix = "PATCH"
	}
	args := []string{"--subject-prefix=" + prefix}
	if o.Version > 1 {
		args = append(args, "-v", strconv.Itoa(o.Version))
	}
	switch {
	case !o.Numbered:
		args = append(args, "-N")
	case count > 1:
		args = append(args, "-n")
	}
	if o.Cover {
		args = append(args, "--cover-letter")
	}
	if o.Signoff {
		args = append(args, "-s")
	}
	if o.Base != "" {
		if err := CheckRevs(o.Base); err != nil {
			return nil, err
		}
		args = append(args, "--base="+o.Base)
	}
	return args, nil
}

// The placeholders git writes in a cover letter.
const (
	coverSubject = "*** SUBJECT HERE ***"
	coverBlurb   = "*** BLURB HERE ***"
)

// fillCover puts the cover letter's subject and text where git left its placeholders. Either
// left empty keeps git's placeholder, for the sender to fill in.
func fillCover(text string, o PatchOptions) string {
	if s := strings.TrimSpace(o.CoverSubject); s != "" {
		text = strings.Replace(text, coverSubject, s, 1)
	}
	if b := strings.TrimSpace(o.CoverText); b != "" {
		text = strings.Replace(text, coverBlurb, b, 1)
	}
	return text
}

// FormatPatches writes p's patches into dir, one file per commit, and returns their names in
// order, a cover letter first. count is how many commits there are, for the numbering. It
// reads with downloads off: in a partial clone, PatchNeeds says what to download first.
func (r *Repo) FormatPatches(ctx context.Context, p PatchRevs, count int, o PatchOptions, dir string) ([]string, error) {
	revs, err := p.revs()
	if err != nil {
		return nil, err
	}
	opts, err := o.args(count)
	if err != nil {
		return nil, err
	}
	args := append(append([]string{"format-patch", "-o", dir}, opts...), revs...)
	out, err := r.readCmd(ctx, Cmd{Args: args, Env: noLazyFetch})
	if err != nil {
		return nil, err
	}
	var names []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, filepath.Base(line))
		}
	}
	if o.Cover && len(names) > 0 && strings.Contains(names[0], "0000-cover-letter") {
		file := filepath.Join(dir, names[0])
		text, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(file, []byte(fillCover(string(text), o)), 0o644); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// FormatPatchText returns p's patches as one mailbox's text: for a .mbox file, or the
// clipboard.
func (r *Repo) FormatPatchText(ctx context.Context, p PatchRevs, count int, o PatchOptions) (string, error) {
	revs, err := p.revs()
	if err != nil {
		return "", err
	}
	opts, err := o.args(count)
	if err != nil {
		return "", err
	}
	out, err := r.readCmd(ctx, Cmd{Args: append(append([]string{"format-patch", "--stdout"}, opts...), revs...), Env: noLazyFetch})
	if err != nil {
		return "", err
	}
	text := string(out)
	if o.Cover {
		text = fillCover(text, o)
	}
	return text, nil
}
