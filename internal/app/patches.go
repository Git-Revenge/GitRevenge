package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Saving commits as patches, and copying them (#1021).

// maxPatchListed is how many of a series' commits the dialog lists.
const maxPatchListed = 500

// PatchSeries is what the Save as patches dialog shows of the commits it would save.
type PatchSeries struct {
	// Commits are the first maxPatchListed, oldest first, merges left out.
	Commits []git.PatchCommit `json:"commits"`
	// Count is how many there are in all.
	Count int `json:"count"`
	// Merges is how many merges are left out, as format-patch leaves them out.
	Merges int `json:"merges"`
	// Edge says one is a shallow clone's edge commit, whose patch would add every file: such
	// a series isn't saved (#1006).
	Edge bool `json:"edge"`
	// Missing, in a partial clone, lists the file versions the patches need that were never
	// downloaded (#1003): one download brings them.
	Missing []string `json:"missing,omitempty"`
	// Base is the commit they apply to: the oldest one's parent, or "".
	Base string `json:"base"`
}

// PatchSeries lists the commits p names, for the dialog to show before anything is written.
func (a *App) PatchSeries(path string, p git.PatchRevs) (PatchSeries, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return PatchSeries{}, err
	}
	ctx := context.Background()
	commits, merges, err := r.Git.PatchList(ctx, p)
	if err != nil {
		return PatchSeries{}, patchError(err)
	}
	out := PatchSeries{Commits: commits[:min(len(commits), maxPatchListed)], Count: len(commits), Merges: merges}
	if out.Commits == nil {
		out.Commits = []git.PatchCommit{}
	}
	for _, c := range commits {
		out.Edge = out.Edge || c.Edge
	}
	if len(commits) > 0 && !commits[0].Edge {
		out.Base = commits[0].First
	}
	if len(commits) > 0 {
		if out.Missing, err = r.Git.PatchNeeds(ctx, p); err != nil {
			return PatchSeries{}, patchError(err)
		}
	}
	return out, nil
}

// BranchPatchBase is where a branch's series starts: where it left its base. That's its stack
// parent when it has one, else its remote's default branch (origin/main); "" when neither is
// known.
func (a *App) BranchPatchBase(path, branch, stackParent string) (string, error) {
	if stackParent != "" {
		return stackParent, nil
	}
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	if err := git.CheckRevs(branch); err != nil {
		return "", userError(err.Error())
	}
	ctx := context.Background()
	remote := "origin"
	if up, err := r.Git.ReadString(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", branch+"@{upstream}"); err == nil {
		if name, _, ok := strings.Cut(up, "/"); ok {
			remote = name
		}
	}
	if def, err := r.Git.ReadString(ctx, "symbolic-ref", "--short", "-q", "refs/remotes/"+remote+"/HEAD"); err == nil && def != "" {
		return def, nil
	}
	for _, guess := range []string{remote + "/main", remote + "/master"} {
		if _, err := r.Git.ReadString(ctx, "rev-parse", "-q", "--verify", "refs/remotes/"+guess); err == nil {
			return guess, nil
		}
	}
	return "", nil
}

// SavedPatches says what was written, and where.
type SavedPatches struct {
	// Files are the names written, in order: a cover letter first, or the one mailbox.
	Files  []string `json:"files"`
	Folder string   `json:"folder"`
	// Exists lists files already in the folder that saving would replace. When it's set,
	// nothing was written: saved again with overwrite, they're replaced.
	Exists []string `json:"exists,omitempty"`
}

// SavePatches writes p's patches into folder: one file per commit, or, when mbox names a file,
// all of them in that one mailbox. Files already there aren't replaced unless overwrite says
// so; then nothing is written and Exists names them. In a partial clone, PatchSeries says what
// to download first.
func (a *App) SavePatches(path string, p git.PatchRevs, o git.PatchOptions, folder, mbox string, overwrite bool) (SavedPatches, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return SavedPatches{}, err
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return SavedPatches{}, userError("That folder doesn't exist: " + folder)
	}
	ctx := context.Background()
	commits, _, err := r.Git.PatchList(ctx, p)
	if err != nil {
		return SavedPatches{}, patchError(err)
	}
	if len(commits) == 0 {
		return SavedPatches{}, userError("There are no commits to save: merges are left out, as format-patch leaves them out.")
	}
	out := SavedPatches{Files: []string{}, Folder: folder}
	if mbox != "" {
		if filepath.Base(mbox) != mbox || mbox == "." || mbox == ".." {
			return SavedPatches{}, userError("A mailbox's name is a file's name, without a folder.")
		}
		target := filepath.Join(folder, mbox)
		if _, err := os.Stat(target); err == nil && !overwrite {
			return SavedPatches{Files: []string{}, Folder: folder, Exists: []string{mbox}}, nil
		}
		text, err := r.Git.FormatPatchText(ctx, p, len(commits), o)
		if err != nil {
			return SavedPatches{}, patchError(err)
		}
		if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
			return SavedPatches{}, writeErr(err)
		}
		out.Files = []string{mbox}
		return out, nil
	}
	// Written into a folder of their own beside the others first, on the same drive, so none
	// is replaced unless asked and the move into place can't fail halfway for space.
	tmp, err := os.MkdirTemp(folder, ".gitrevenge-patches-")
	if err != nil {
		return SavedPatches{}, writeErr(err)
	}
	// Emptied by the moves below; whatever's left, after a failure, goes with it.
	defer func() { _ = os.RemoveAll(tmp) }()
	names, err := r.Git.FormatPatches(ctx, p, len(commits), o, tmp)
	if err != nil {
		return SavedPatches{}, patchError(err)
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(folder, name)); err == nil {
			out.Exists = append(out.Exists, name)
		}
	}
	if len(out.Exists) > 0 && !overwrite {
		return out, nil
	}
	out.Exists = nil
	for _, name := range names {
		if err := os.Rename(filepath.Join(tmp, name), filepath.Join(folder, name)); err != nil {
			return SavedPatches{}, writeErr(err)
		}
	}
	out.Files = names
	return out, nil
}

// CopyPatches returns p's patches as one mailbox's text, for the clipboard: Copy as a patch.
func (a *App) CopyPatches(path string, p git.PatchRevs, o git.PatchOptions) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	commits, _, err := r.Git.PatchList(ctx, p)
	if err != nil {
		return "", patchError(err)
	}
	if len(commits) == 0 {
		return "", userError("There are no commits to copy: merges are left out, as format-patch leaves them out.")
	}
	text, err := r.Git.FormatPatchText(ctx, p, len(commits), o)
	if err != nil {
		return "", patchError(err)
	}
	return text, nil
}

// ShowFolder opens a folder in the system's file manager: where patches were saved.
func (a *App) ShowFolder(dir string) error {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return userError("That folder doesn't exist: " + dir)
	}
	return showFolder(dir)
}

// patchError says what went wrong saving patches in words.
func patchError(err error) error {
	if errors.Is(err, git.ErrTooManyPatches) {
		return userError("That's more commits than can be named one by one. Save them as a range, from their branch or with Since, instead.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		return gitMessage(gerr.Stderr)
	}
	return writeErr(err)
}
