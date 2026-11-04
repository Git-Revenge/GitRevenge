package app

import (
	"context"
	"net/http"

	"github.com/git-revenge/gitrevenge/internal/settings"
	"github.com/git-revenge/gitrevenge/internal/update"
	"github.com/git-revenge/gitrevenge/internal/version"
)

// UpdateInfo says whether a newer release than this build exists.
type UpdateInfo struct {
	Current string `json:"current"`
	// Latest is the newest release's version, or empty when no check ran.
	Latest    string `json:"latest"`
	URL       string `json:"url"`
	Available bool   `json:"available"`
}

// CheckForUpdate asks GitHub for the newest release. The automatic check at startup is
// skipped when the user turned update checks off; a check the user asks for always runs.
func (a *App) CheckForUpdate(automatic bool) (UpdateInfo, error) {
	info := UpdateInfo{Current: version.Version}
	if automatic {
		if on, err := a.UpdateChecks(); err != nil || !on {
			return info, err
		}
	}
	url := a.updateURL
	if url == "" {
		url = update.LatestURL
	}
	rel, err := update.Latest(context.Background(), http.DefaultClient, url, version.Version)
	if err != nil {
		return info, userError("Couldn't check for updates: " + err.Error())
	}
	info.Latest, info.URL = rel.Version, rel.URL
	info.Available = update.Newer(rel.Version, version.Version)
	return info, nil
}

// UpdateChecks reports whether the app checks for a newer release at startup.
func (a *App) UpdateChecks() (bool, error) {
	m, err := a.sessions()
	if err != nil {
		return false, err
	}
	return !m.Settings().Get().UpdateChecksOff, nil
}

// SetUpdateChecks turns the startup check for a newer release on or off.
func (a *App) SetUpdateChecks(on bool) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) { s.UpdateChecksOff = !on })
}
