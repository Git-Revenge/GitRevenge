package app

import (
	"context"
	"errors"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// IgnoreSources lists the ignore files that apply to a repository, in the order git reads
// them.
func (a *App) IgnoreSources(path string) ([]git.IgnoreSource, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	return r.Git.IgnoreSources(context.Background())
}

// ReadIgnoreFile returns an ignore file's text, "" when it doesn't exist yet.
func (a *App) ReadIgnoreFile(path, source string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	return r.Git.ReadIgnoreFile(context.Background(), source)
}

// SaveIgnoreFile writes an ignore file's text. base is the text it had when editing
// began; if someone else changed the file since, nothing is written.
func (a *App) SaveIgnoreFile(path, source, base, text string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	err = r.Git.WriteIgnoreFile(context.Background(), source, base, text)
	if errors.Is(err, git.ErrIgnoreFileChanged) {
		return userError("This file changed on disk since you started editing, so it wasn't saved. Reload it to see the change, then make your edits again.")
	}
	return err
}

// IgnoreStats says what each rule ignores, brings back or still matches among tracked
// files.
func (a *App) IgnoreStats(path string) (git.IgnoreStats, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.IgnoreStats{}, err
	}
	return r.Git.IgnoreStats(context.Background())
}

// CheckIgnorePath says whether the saved rules ignore a path, and which rule decides.
func (a *App) CheckIgnorePath(path, file string) (git.IgnoreCheck, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.IgnoreCheck{}, err
	}
	return r.Git.CheckIgnorePath(context.Background(), file)
}

// SetIgnoreTab remembers which repository the Ignore rules tab shows, for the next launch;
// "" records that the tab is closed.
func (a *App) SetIgnoreTab(repo string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) { s.IgnoreTab = repo })
}
