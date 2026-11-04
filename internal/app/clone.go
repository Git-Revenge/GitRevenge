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

// cloneKey registers the clone in the running network operations. Only one clone runs at a
// time; the clone dialog is modal.
const cloneKey = "\x00clone"

// CloneName suggests a folder name for cloning url, as git clone would choose it.
func (a *App) CloneName(url string) string {
	return git.CloneName(url)
}

// Clone clones url into parent/name and returns the new repository's path. Progress is
// reported as "net:progress" events with op "clone"; CancelClone stops it. A folder the
// clone created, or files it left in an empty one, are removed if it fails.
func (a *App) Clone(url, parent, name string) (string, error) {
	url, name = strings.TrimSpace(url), strings.TrimSpace(name)
	if url == "" {
		return "", userError("Enter the URL of the repository to clone.")
	}
	dir, err := newRepoDir(parent, name)
	if err != nil {
		return "", err
	}
	existed, err := emptyDir(dir)
	if err != nil {
		return "", err
	}
	runner, err := a.runner()
	if err != nil {
		return "", err
	}
	err = a.track(cloneKey, dir, "clone", "A clone is already running.", func(ctx context.Context, o git.NetOptions) error {
		return git.Clone(ctx, runner, url, dir, o)
	})
	if err != nil {
		removeClone(dir, existed)
		return "", err
	}
	return dir, nil
}

// CancelClone stops the running clone.
func (a *App) CancelClone() {
	a.cancelNetwork(cloneKey)
}

// Init creates an empty repository in dir (creating the folder if needed) and returns its
// path. branch names the initial branch; empty uses the user's init.defaultBranch.
func (a *App) Init(dir, branch string) (string, error) {
	dir = strings.TrimSpace(dir)
	if !filepath.IsAbs(dir) {
		return "", userError("Choose a folder for the new repository.")
	}
	dir = filepath.Clean(dir)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return "", userError("That folder is already a git repository.")
	}
	runner, err := a.runner()
	if err != nil {
		return "", err
	}
	if err := git.Init(context.Background(), runner, dir, strings.TrimSpace(branch)); err != nil {
		var gerr *git.Error
		if errors.As(err, &gerr) && gerr.Stderr != "" {
			return "", gitMessage(gerr.Stderr)
		}
		return "", err
	}
	return dir, nil
}

// runner returns a git runner for work outside an open repository.
func (a *App) runner() (*git.Runner, error) {
	env := a.Environment()
	if env.Problem != "" {
		return nil, errors.New(env.Problem)
	}
	return git.NewRunner(env.GitPath), nil
}

// newRepoDir checks a parent folder and new folder name and joins them.
func newRepoDir(parent, name string) (string, error) {
	if !filepath.IsAbs(parent) {
		return "", userError("Choose the folder to clone into.")
	}
	if st, err := os.Stat(parent); err != nil || !st.IsDir() {
		return "", userError("The folder to clone into doesn't exist.")
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", userError("Enter a folder name for the clone, without slashes.")
	}
	return filepath.Join(filepath.Clean(parent), name), nil
}

// emptyDir reports whether dir already exists. It fails if dir exists and isn't an empty
// folder, since git can only clone into a new or empty one.
func emptyDir(dir string) (existed bool, err error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || len(entries) > 0 {
		return true, userError(filepath.Base(dir) + " already exists there and isn't an empty folder.")
	}
	return true, nil
}

// removeClone deletes what a failed or cancelled clone left behind: the folder itself if the
// clone created it, otherwise its contents. git cleans up after its own failures but not
// after being killed, and on Windows its child processes may hold files open briefly.
func removeClone(dir string, existed bool) {
	for range 20 {
		var err error
		if existed {
			err = clearDir(dir)
		} else {
			err = os.RemoveAll(dir)
		}
		if err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
