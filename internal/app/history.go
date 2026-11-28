package app

import (
	"context"
	"errors"
	"slices"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// FileHistoryResult is a file's history.
type FileHistoryResult struct {
	Commits []git.FileCommit `json:"commits"`
	// Stop, in a partial clone, says where following the file's renames stopped: at a rename
	// that compares files never downloaded (#1010). The view offers to download them.
	Stop *git.HistoryStop `json:"stop,omitempty"`
}

// FileHistory lists commits that changed file, newest first, following renames. Asking for
// another file's history stops this one, which then answers with nothing.
func (a *App) FileHistory(path, file string) (FileHistoryResult, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return FileHistoryResult{}, err
	}
	ctx, done := a.latest.start("history", r.Path)
	defer done()
	commits, stop, err := r.Git.FileHistory(ctx, file)
	if errors.Is(err, context.Canceled) {
		return FileHistoryResult{Commits: []git.FileCommit{}}, nil // superseded, or the repository closed
	}
	if err != nil {
		return FileHistoryResult{}, err
	}
	return FileHistoryResult{Commits: commits, Stop: stop}, nil
}

// BlameTrack is the blame view's track through a file's versions (#1078).
type BlameTrack struct {
	// Versions are the commits that changed the file, oldest first.
	Versions []git.FileVersion `json:"versions"`
	// RenamedFrom is what the oldest version renamed the file from, or "": the track stops
	// there, and the view offers to carry on under the old name.
	RenamedFrom string `json:"renamedFrom"`
	// WorkTree says the file has changes not committed yet, a last step after the versions.
	WorkTree bool `json:"workTree"`
	// Home is the version the view's own blame is of, as an index into Versions (#1083): the
	// newest version in rev's history. Blamed at a commit, the track still runs to the
	// newest version, with the handle on that commit's.
	Home int `json:"home"`
}

// BlameTrack lists the versions of file in rev's history ("" for HEAD, and then the work
// tree), for the blame view's track. Each version's text and what it changed come from
// FileDiff, DiffCommit, or DiffWorkTree for the last step. Asking for another file's track
// stops this one, which then answers with nothing.
func (a *App) BlameTrack(path, file, rev string) (BlameTrack, error) {
	empty := BlameTrack{Versions: []git.FileVersion{}}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	ctx, done := a.latest.start("blame-track", r.Path)
	defer done()
	// Blamed at a commit in HEAD's history, the track runs from the first version to the
	// newest, with home on the commit's. One HEAD hasn't got (another branch's) has a track
	// of its own history only.
	versions, err := r.Git.Versions(ctx, file, "")
	if errors.Is(err, context.Canceled) {
		return empty, nil
	}
	if err != nil {
		return empty, err
	}
	track := BlameTrack{Versions: versions, Home: len(versions) - 1}
	fromHead := true
	if rev != "" {
		home, _, err := r.Git.LastTouched(ctx, rev, file)
		at := slices.IndexFunc(versions, func(v git.FileVersion) bool { return v.ID == home })
		if err == nil && at >= 0 {
			track.Home = at
		} else {
			fromHead = false
			if versions, err = r.Git.Versions(ctx, file, rev); err != nil {
				return empty, err
			}
			track = BlameTrack{Versions: versions, Home: len(versions) - 1}
		}
	}
	if len(track.Versions) > 0 {
		track.RenamedFrom, _ = r.Git.RenamedAt(ctx, track.Versions[0].ID, file)
	}
	// The work tree is the step after HEAD's newest version, so only a track from HEAD has it.
	if fromHead {
		track.WorkTree, _ = r.Git.Uncommitted(ctx, file)
	}
	return track, nil
}

// BlameStep blames file at one of the track's versions (#1079): every line, or with ranges
// only those, which the view asks for when it works out the rest from a version next to it.
// Asking for another step stops this one, which then answers with nothing; the view's own
// blame goes on. It downloads nothing: in a partial clone, a version it lacks is an error.
func (a *App) BlameStep(path, file, commit string, ranges []git.BlameRange) ([]git.BlameLine, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	if commit == "" {
		return nil, userError("Which version to blame?")
	}
	ctx, done := a.latest.start("blame-step", r.Path)
	defer done()
	lines, err := r.Git.BlameRanges(ctx, file, commit, ranges)
	if errors.Is(err, context.Canceled) {
		return []git.BlameLine{}, nil
	}
	return lines, err
}

// BlameBlock lists the versions of lines from to to of file at commit (#1081): Rewind these
// lines, in the blame view. Newest first, following the lines as they move. Asking for
// another block stops this one, which then answers with nothing.
func (a *App) BlameBlock(path, file, commit string, from, to int) ([]git.BlockVersion, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	if commit == "" {
		return nil, userError("Lines not committed yet have no history to rewind.")
	}
	ctx, done := a.latest.start("blame-block", r.Path)
	defer done()
	versions, err := r.Git.BlockHistory(ctx, file, commit, from, to)
	if errors.Is(err, context.Canceled) {
		return []git.BlockVersion{}, nil
	}
	return versions, err
}

// Blame attributes each line of file at rev ("" for the work tree) to a commit. Asking for
// another blame stops this one, which then answers with nothing. In a partial clone it
// downloads nothing: BlameNeeds says what it would need, and the view asks before
// BlameDownloading (#1003).
func (a *App) Blame(path, file, rev string) ([]git.BlameLine, error) {
	return a.blame(path, file, rev, false)
}

// BlameDownloading blames as Blame does, letting git download any version a partial clone
// still lacks: for after the view has asked, and the versions BlameNeeds named have come in
// one download.
func (a *App) BlameDownloading(path, file, rev string) ([]git.BlameLine, error) {
	return a.blame(path, file, rev, true)
}

func (a *App) blame(path, file, rev string, mayDownload bool) ([]git.BlameLine, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	ctx, done := a.latest.start("blame", r.Path)
	defer done()
	lines, err := r.Git.Blame(ctx, file, rev, mayDownload)
	if errors.Is(err, context.Canceled) {
		return []git.BlameLine{}, nil // superseded, or the repository closed
	}
	if errors.Is(err, git.ErrNotDownloaded) {
		return nil, userError("Some versions of " + file + " were never downloaded to this partial clone, and blame doesn't download them by itself.")
	}
	if lines == nil {
		lines = []git.BlameLine{}
	}
	return lines, err
}
