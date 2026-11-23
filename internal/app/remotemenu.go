package app

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// The remote's own menu in the Remotes panel (#921): hiding a remote's branches, where pushes
// go by default, and showing a remote that's a folder on this machine.

// HideRemote hides every branch of a remote from the graph and the sidebar, or shows them
// again. It's kept in the app's settings for the repository, never in git config.
func (a *App) HideRemote(path, remote string, hide bool) error {
	path = a.repoKey(path)
	m, err := a.sessions()
	if err != nil {
		return err
	}
	err = a.updateRepoSettings(m, path, func(rs *settings.RepoSettings) {
		rs.HideRemotes = slices.DeleteFunc(rs.HideRemotes, func(n string) bool { return n == remote })
		if hide {
			rs.HideRemotes = append(rs.HideRemotes, remote)
		}
	})
	if err != nil {
		return err
	}
	if r := m.Get(path); r != nil {
		r.ReloadGraph()
	}
	return nil
}

// hiddenRemotes returns the remotes a repository's settings hide.
func (a *App) hiddenRemotes(path string) []string {
	m, err := a.sessions()
	if err != nil {
		return nil
	}
	return m.Settings().Get().Repo(a.repoKey(path)).HideRemotes
}

// followRemote keeps a remote's hidden setting with it when it's renamed (to != "") or
// removed (to == "").
func (a *App) followRemote(path, from, to string) {
	if !slices.Contains(a.hiddenRemotes(path), from) {
		return
	}
	m, err := a.sessions()
	if err != nil {
		return
	}
	_ = a.updateRepoSettings(m, a.repoKey(path), func(rs *settings.RepoSettings) {
		rs.HideRemotes = slices.DeleteFunc(rs.HideRemotes, func(n string) bool { return n == from || n == to })
		if to != "" {
			rs.HideRemotes = append(rs.HideRemotes, to)
		}
	})
}

// SetPushDefault makes a remote where pushes go when a branch names no remote of its own
// (remote.pushDefault in the repository's config), or with remote "" unsets it there.
func (a *App) SetPushDefault(path, remote string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if remote == "" {
		_, err = r.Git.Write(ctx, git.Cmd{Args: []string{"config", "--local", "--unset-all", "remote.pushDefault"}})
		var gerr *git.Error
		// Exit 5: it wasn't set.
		if errors.As(err, &gerr) && gerr.ExitCode == 5 {
			return nil
		}
		return writeErr(err)
	}
	remotes, err := r.Git.Remotes(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(remotes, func(x git.Remote) bool { return x.Name == remote }) {
		return userError("There's no remote called " + remote + ".")
	}
	_, err = r.Git.Write(ctx, git.Cmd{Args: []string{"config", "--local", "remote.pushDefault", remote}})
	return writeErr(err)
}

// remoteFolder returns the folder a remote that's on this machine points at, or "" when it
// isn't one, or isn't there. A relative path is read from the repository's folder, as git
// reads it there.
func remoteFolder(repoDir, remoteURL string) string {
	u := strings.TrimSpace(remoteURL)
	if git.ParseLocation(u).Place != git.PlaceLocal || u == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(u), "file://") {
		parsed, err := url.Parse(u)
		if err != nil {
			return ""
		}
		u = parsed.Path
		// file:///C:/repos/x
		if runtime.GOOS == "windows" && len(u) > 2 && u[0] == '/' && u[2] == ':' {
			u = u[1:]
		}
	}
	if !filepath.IsAbs(u) {
		u = filepath.Join(repoDir, u)
	}
	u = filepath.Clean(u)
	if info, err := os.Stat(u); err != nil || !info.IsDir() {
		return ""
	}
	return u
}

// ShowRemoteFolder opens the folder a remote on this machine points at in the system's file
// manager. Only a remote's own folder can be opened this way.
func (a *App) ShowRemoteFolder(path, remote string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	remotes, err := r.Git.Remotes(context.Background())
	if err != nil {
		return err
	}
	i := slices.IndexFunc(remotes, func(x git.Remote) bool { return x.Name == remote })
	if i < 0 {
		return userError("There's no remote called " + remote + ".")
	}
	dir := remoteFolder(r.Git.Dir(), remotes[i].FetchURL)
	if dir == "" {
		return userError(remote + " isn't a folder on this computer, or the folder is gone.")
	}
	return showFolder(dir)
}

// showFolder opens a folder in the file manager: Explorer, Finder, or whatever xdg-open
// picks.
func showFolder(dir string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return userError("Couldn't open the file manager: " + err.Error())
	}
	// Explorer exits 1 even when it opens the folder, so how it ends says nothing.
	go func() { _ = cmd.Wait() }()
	return nil
}
