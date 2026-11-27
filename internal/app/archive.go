package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// ArchivePlan says what an archive of rev, or of its folder sub, would hold (#936): how many
// files and how big, how many the commit's .gitattributes leave out or fill in, and, in a
// partial clone, which files to download first. Asking again, or StopArchivePlan, stops the
// one before: on the kernel it reads for about 3 s.
func (a *App) ArchivePlan(path, rev, sub string) (git.ArchivePlan, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.ArchivePlan{}, err
	}
	ctx, done := a.latest.start("archive-plan", r.Path)
	defer done()
	plan, err := r.Git.ArchivePlan(ctx, rev, sub)
	if ctx.Err() != nil {
		return git.ArchivePlan{}, nil // stopped: nobody is waiting for it
	}
	return plan, archiveError(err, sub)
}

// StopArchivePlan stops a plan ArchivePlan is reading, for a dialog that closed.
func (a *App) StopArchivePlan(path string) {
	if r, err := a.openRepo(path); err == nil {
		a.latest.stop("archive-plan", r.Path)
	}
}

// SavedArchive says what Save as an archive wrote, and where.
type SavedArchive struct {
	File   string `json:"file"`
	Folder string `json:"folder"`
	// Exists says a file of that name is already in the folder: nothing was written, and
	// saving again with overwrite replaces it.
	Exists bool `json:"exists,omitempty"`
	// Stopped says StopArchive stopped it: nothing was written.
	Stopped bool `json:"stopped,omitempty"`
}

// SaveArchive writes rev, or its folder o.Sub, into folder as name with git archive (#936). A
// file already there isn't replaced unless overwrite says so. The archive is written beside it
// under another name first, so stopping or failing halfway leaves nothing behind, and a file
// it replaces stays until the new one is whole. On the kernel a zip takes about 30 s.
func (a *App) SaveArchive(path, rev string, o git.ArchiveOptions, folder, name string, overwrite bool) (SavedArchive, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return SavedArchive{}, err
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return SavedArchive{}, userError("That folder doesn't exist: " + folder)
	}
	name = strings.TrimSpace(name)
	// Neither slash, on any system: Linux takes a backslash in a name, but one typed here is
	// meant as a folder, and such a file couldn't be opened on Windows.
	if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return SavedArchive{}, userError("The archive's name is a file's name, without a folder.")
	}
	out := SavedArchive{File: name, Folder: folder}
	target := filepath.Join(folder, name)
	if _, err := os.Stat(target); err == nil && !overwrite {
		out.Exists = true
		return out, nil
	}
	tmp, err := os.CreateTemp(folder, ".gitrevenge-archive-*")
	if err != nil {
		return SavedArchive{}, writeErr(err)
	}
	part := tmp.Name()
	_ = tmp.Close()
	// Renamed into place below; whatever's left, after a failure or a stop, goes.
	defer removeSoon(part)
	ctx, done := a.latest.start("archive", r.Path)
	defer done()
	if err := r.Git.Archive(ctx, rev, o, part); err != nil {
		if ctx.Err() != nil {
			return SavedArchive{File: name, Folder: folder, Stopped: true}, nil
		}
		if r.Git.Partial(context.Background()) {
			return SavedArchive{}, userError("Some of its files aren't downloaded yet, so nothing was saved. Download them first.")
		}
		return SavedArchive{}, archiveError(err, o.Sub)
	}
	if err := os.Rename(part, target); err != nil {
		return SavedArchive{}, writeErr(err)
	}
	return out, nil
}

// removeSoon removes a file, now if it can. A compressor git archive hands its output to
// (tar.<format>.command, such as pigz) holds the file open after git is stopped, until it reads
// the end of what git sent; on Windows the file can't go until then, so it's tried again in
// the background for a minute.
func removeSoon(file string) {
	if err := os.Remove(file); err == nil || errors.Is(err, os.ErrNotExist) {
		return
	}
	go func() {
		for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(250 * time.Millisecond) {
			if err := os.Remove(file); err == nil || errors.Is(err, os.ErrNotExist) {
				return
			}
		}
	}()
}

// StopArchive stops an archive SaveArchive is writing.
func (a *App) StopArchive(path string) {
	if r, err := a.openRepo(path); err == nil {
		a.latest.stop("archive", r.Path)
	}
}

// archiveError says what went wrong in words, for a folder that isn't one.
func archiveError(err error, sub string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, git.ErrNotAFolder):
		return userError("“" + strings.TrimSpace(sub) + "” isn't a folder in that commit.")
	case strings.Contains(err.Error(), "isn't a folder inside the repository"), strings.Contains(err.Error(), "isn't a commit here"):
		return userError(err.Error())
	}
	return friendly(err)
}
