package app

import (
	"path/filepath"

	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// Every commit's trailers are read once, in the background, and kept in a file of the
// app's own (see trailerindex): what Smart filters asks is answered from that, never by
// reading the history again. This file is the app's side of it: where the files go, and
// telling the window how far a read has got.

// trailerDir is where trailer indexes are kept: the operating system's cache folder. With
// the settings somewhere of a test's choosing, they go beside those instead, so a test
// never writes to the real cache.
func (a *App) trailerDir() string {
	if a.settingsPath != "" {
		return filepath.Join(filepath.Dir(a.settingsPath), "trailers")
	}
	return trailerindex.Dir()
}

// TrailerProgress is the payload of the "trailers:progress" event, and what TrailerStatus
// returns: how far reading a repository's trailers has got.
type TrailerProgress struct {
	Path string `json:"path"`
	trailerindex.Status
}

func (a *App) emitTrailerProgress(path string, st trailerindex.Status) {
	a.emit("trailers:progress", TrailerProgress{Path: path, Status: st})
}

// TrailerStatus says how far reading the trailers of the repository at path has got, for a
// window that wasn't listening when the read began.
func (a *App) TrailerStatus(path string) (TrailerProgress, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return TrailerProgress{Path: path}, err
	}
	return TrailerProgress{Path: r.Path, Status: r.Trailers().Status()}, nil
}
