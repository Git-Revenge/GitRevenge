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

// HomeDir is the user's home folder, where the clone dialog starts when neither Settings nor
// a recent repository says where. It is "" if the home folder can't be found.
func (a *App) HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// cloneBranchesKey registers a listing of a server's branches for the clone dialog, apart
// from the clone itself.
const cloneBranchesKey = "\x00clone-branches"

// Clone clones url into parent/name with the options chosen, and returns the new
// repository's path. Progress is reported as "net:progress" events with op "clone";
// CancelClone stops it. A folder the clone created, or files it left in an empty one, are
// removed if it fails.
func (a *App) Clone(url, parent, name string, opts git.CloneOptions) (string, error) {
	url, name = strings.TrimSpace(url), strings.TrimSpace(name)
	opts.Branch = strings.TrimSpace(opts.Branch)
	if url == "" {
		return "", userError("Enter the URL of the repository to clone.")
	}
	if opts.Depth < 0 {
		return "", userError("The number of commits to clone has to be 1 or more.")
	}
	if git.CheckRevs(opts.Branch) != nil {
		return "", userError("“" + opts.Branch + "” can't be a branch name.")
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
	hosts := func() []string { return []string{git.RemoteHost(url)} }
	err = a.track(cloneKey, dir, "clone", hosts, "A clone is already running.", func(ctx context.Context, o git.NetOptions) error {
		return git.Clone(ctx, runner, url, dir, opts, o)
	})
	if err != nil {
		removeClone(dir, existed)
		return "", err
	}
	// A clone holds only what git and the user's own settings put in its config and hooks.
	a.trustNew(dir)
	return dir, nil
}

// CancelClone stops the running clone.
func (a *App) CancelClone() {
	a.cancelNetwork(cloneKey)
}

// CloneBranches asks the server at url which branches it has, for the clone dialog's "Only
// one branch", as the clone itself would ask it: so it can prompt for a password the same
// way, and the clone after it won't. parent and name are where the clone would go, which a
// prompt names. A listing still running is stopped.
func (a *App) CloneBranches(url, parent, name string) (git.RemoteHeads, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return git.RemoteHeads{}, userError("Enter the URL of the repository to clone.")
	}
	runner, err := a.runner()
	if err != nil {
		return git.RemoteHeads{}, err
	}
	a.cancelNetwork(cloneBranchesKey)
	dir := filepath.Join(parent, strings.TrimSpace(name))
	hosts := func() []string { return []string{git.RemoteHost(url)} }
	var heads git.RemoteHeads
	err = a.track(cloneBranchesKey, dir, "clone", hosts, "The branches are already being listed.", func(ctx context.Context, o git.NetOptions) error {
		// Nothing shows progress for a listing, and a server that doesn't answer mustn't hold
		// the dialog.
		o.OnProgress = nil
		ctx, cancel := context.WithTimeout(ctx, cloneBranchesWait)
		defer cancel()
		var err error
		heads, err = git.LsRemoteHeads(ctx, runner, url, o)
		return err
	})
	if err != nil {
		return git.RemoteHeads{}, err
	}
	return heads, nil
}

// cloneBranchesWait is how long a listing of a server's branches waits for it.
const cloneBranchesWait = 30 * time.Second

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
	a.trustNew(dir)
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
