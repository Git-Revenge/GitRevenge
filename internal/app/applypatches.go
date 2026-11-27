package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Applying a series of patches with git am, previewed first (#1022).

// maxPatchFile is the largest file of patches read: a list archive's thread of a big series
// is a few megabytes.
const maxPatchFile = 64 << 20

// alreadyAmong is how many of HEAD's commits a patch that's in already is looked for among.
const alreadyAmong = 1000

// patchSet is a file of patches read for the preview: its messages split out into a folder of
// their own, kept until the preview closes.
type patchSet struct {
	repo string
	dir  string
	msgs []git.MailMessage

	mu     sync.Mutex
	cancel context.CancelFunc
}

// patchSets holds the files of patches open in previews, by token.
type patchSets struct {
	mu   sync.Mutex
	sets map[string]*patchSet
	n    int
}

func (s *patchSets) add(p *patchSet) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sets == nil {
		s.sets = map[string]*patchSet{}
	}
	s.n++
	token := "patches-" + strconv.Itoa(s.n)
	s.sets[token] = p
	return token
}

func (s *patchSets) get(token string) *patchSet {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sets[token]
}

func (s *patchSets) drop(token string) {
	s.mu.Lock()
	p := s.sets[token]
	delete(s.sets, token)
	s.mu.Unlock()
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()
	_ = os.RemoveAll(p.dir)
}

// PatchFile is a file of patches as the preview shows it.
type PatchFile struct {
	// Token names it for TryPatches, ApplyPatches and ClosePatches.
	Token string `json:"token"`
	// Name is the file's name.
	Name string `json:"name"`
	// Kind is git.PatchMail, git.PatchDiff, or "" for neither.
	Kind string `json:"kind"`
	// Messages are every message of mail, in the file's order: patches, cover letters and
	// replies alike.
	Messages []git.MailMessage `json:"messages"`
	// Staged says the index has staged changes: git am needs none.
	Staged bool `json:"staged"`
	// Branch is the checked-out branch, or "" when HEAD is detached.
	Branch string `json:"branch"`
	// Missing, in a partial clone, lists the versions of the files the patches change, as the
	// checked-out commit has them, that were never downloaded: the preview needs them (#1003).
	Missing []string `json:"missing,omitempty"`
}

// ChoosePatchFile shows a file picker for patches or a diff, and returns the file chosen, or
// "" if cancelled.
func (a *App) ChoosePatchFile() (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Apply patches or a diff",
		Filters: []runtime.FileFilter{
			{DisplayName: "Patches and diffs (*.patch, *.diff, *.mbox, *.eml)", Pattern: "*.patch;*.diff;*.mbox;*.eml"},
			{DisplayName: "Any file", Pattern: "*.*"},
		},
	})
	if err == nil {
		a.chosen.add(file)
	}
	return file, err
}

// ReadPatches reads a file of patches someone chose, for the preview: what it holds and, for
// mail, each message. Only a file picked in a file dialog is read.
func (a *App) ReadPatches(path, file string) (PatchFile, error) {
	if !a.chosen.has(file) {
		return PatchFile{}, userError("Choose the file of patches first.")
	}
	r, err := a.openRepo(path)
	if err != nil {
		return PatchFile{}, err
	}
	info, err := os.Stat(file)
	if err != nil {
		return PatchFile{}, userError("That file can't be read: " + file)
	}
	if info.Size() > maxPatchFile {
		return PatchFile{}, userError("That file is too big to read as patches: " + file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return PatchFile{}, writeErr(err)
	}
	out := PatchFile{Name: filepath.Base(file), Kind: git.PatchFileKind(data), Messages: []git.MailMessage{}}
	if out.Kind != git.PatchMail {
		return out, nil
	}
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "gitrevenge-am-")
	if err != nil {
		return PatchFile{}, writeErr(err)
	}
	msgs, err := r.Git.ReadMailbox(ctx, file, dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return PatchFile{}, patchError(err)
	}
	out.Token = a.patches.add(&patchSet{repo: r.Path, dir: dir, msgs: msgs})
	out.Messages = msgs
	out.Staged = r.Git.HasStaged(ctx)
	out.Branch = r.Git.Head(ctx).Branch
	var paths []string
	for _, m := range msgs {
		for _, f := range m.Files {
			paths = append(paths, f.Path)
		}
	}
	if out.Missing, err = r.Git.MissingAt(ctx, "HEAD", paths); err != nil {
		return PatchFile{}, patchError(err)
	}
	return out, nil
}

// PatchTry is one patch's row in the preview once its try finishes. Each is sent as the
// "patches:tried" event as it comes, and all of them are TryPatches' answer.
type PatchTry struct {
	Token string `json:"token"`
	// Run is the try's number, for the window to tell its rows from an older try's.
	Run int `json:"run"`
	// Index is the message's place in the file.
	Index int `json:"index"`
	git.TryResult
	// Already names the commit a patch that's in already came in as, when it's among the
	// checked-out commit's last alreadyAmong.
	Already string `json:"already,omitempty"`
	// Waits says an earlier patch's conflict stops this one being tried until it's resolved.
	Waits bool `json:"waits,omitempty"`
}

// TryPatches tries the picked messages, in order, on a throwaway copy of the index made from
// HEAD: as they are, else by merging. The working tree and the index stay as they are. A new
// try of the same file stops the one before.
func (a *App) TryPatches(path, token string, picked []int, run int) ([]PatchTry, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	p := a.patches.get(token)
	if p == nil || p.repo != r.Path {
		return nil, userError("That file of patches isn't open any more. Choose it again.")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = cancel
	p.mu.Unlock()

	dir := filepath.Join(p.dir, "try-"+strconv.Itoa(run))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, writeErr(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	trial, err := r.Git.StartTrial(ctx, dir)
	if err != nil {
		return nil, patchError(err)
	}
	var known map[string]string
	out := []PatchTry{}
	stuck := false
	for _, i := range picked {
		if ctx.Err() != nil {
			return out, nil
		}
		if i < 0 || i >= len(p.msgs) || !p.msgs[i].HasPatch {
			continue
		}
		row := PatchTry{Token: token, Run: run, Index: i}
		if stuck {
			row.Waits = true
		} else {
			row.TryResult = trial.Try(ctx, p.msgs[i].Patch)
			stuck = row.Status == git.TryConflicts
			if row.Status == git.TryApplied {
				if known == nil {
					known, _ = r.Git.PatchCommitIDs(ctx, "HEAD", alreadyAmong)
				}
				if id, err := r.Git.PatchID(ctx, p.msgs[i].Patch); err == nil {
					row.Already = known[id]
				}
			}
		}
		if ctx.Err() != nil {
			return out, nil
		}
		out = append(out, row)
		a.emit("patches:tried", row)
	}
	return out, nil
}

// ApplyPatches applies the picked messages, in order, as commits with git am --3way, keeping
// each one's author, date and message: onto a new branch from here when branch names one,
// else onto the branch checked out. A conflict stops it there, for the operation strip.
func (a *App) ApplyPatches(path, token string, picked []int, branch string, signoff bool) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	p := a.patches.get(token)
	if p == nil || p.repo != r.Path {
		return userError("That file of patches isn't open any more. Choose it again.")
	}
	ctx := context.Background()
	if r.Git.HasStaged(ctx) {
		return userError("git am needs nothing staged. Commit or unstage your staged changes first.")
	}
	var mbox []byte
	n := 0
	for _, i := range picked {
		if i < 0 || i >= len(p.msgs) || !p.msgs[i].HasPatch {
			continue
		}
		text, err := os.ReadFile(p.msgs[i].File)
		if err != nil {
			return writeErr(err)
		}
		if len(mbox) > 0 && mbox[len(mbox)-1] != '\n' {
			mbox = append(mbox, '\n')
		}
		mbox = append(mbox, text...)
		n++
	}
	if n == 0 {
		return userError("Tick at least one patch to apply.")
	}
	file := filepath.Join(p.dir, "apply.mbox")
	if err := os.WriteFile(file, mbox, 0o644); err != nil {
		return writeErr(err)
	}
	label := "Apply " + strconv.Itoa(n) + " patches"
	if n == 1 {
		label = "Apply a patch"
	}
	err = a.record(r, label, kindMove, "", func() error {
		if branch != "" {
			if err := r.Git.CreateBranch(ctx, branch, "", true); err != nil {
				return err
			}
		}
		return r.Git.Am(ctx, file, signoff)
	})
	if errors.Is(err, git.ErrConflicts) {
		return userError("A patch stopped on a conflict. Resolve the files, stage them, then Continue in the banner; or Skip that patch, or Abort.")
	}
	if errors.Is(err, git.ErrInvalidName) {
		return userError("“" + branch + "” can't be a branch's name.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.Stderr != "" {
		return gitMessage(gerr.Stderr)
	}
	return writeErr(err)
}

// ClosePatches forgets a file of patches when its preview closes, stopping any try.
func (a *App) ClosePatches(token string) {
	a.patches.drop(token)
}
