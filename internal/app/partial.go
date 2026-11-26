package app

import (
	"context"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Downloads in a partial clone (#1003). A file's contents arrive the first time something
// reads them, and git fetches one at a time, behind whatever asked. So reads run with
// downloads off and say what's missing, and the app downloads it in one request: by itself
// for one file a view opened, after asking for many (blame), and never for reads across the
// history.

// downloadKey is a download's network operation, apart from the repository's fetch, pull or
// push, so neither waits for the other. Each download has its own, so two views can fetch at
// once and each one's Stop stops only its own.
func downloadKey(token string) string { return "\x00download\x00" + token }

// DownloadFiles fetches objects from a partial clone's remote in one request: the versions a
// view found missing. token names the download, for CancelDownload. Progress comes as
// "net:progress" with op "download", and a password prompt says it's asked during a download.
func (a *App) DownloadFiles(path, token string, ids []string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	if token == "" {
		return userError("A download needs a name.")
	}
	for _, id := range ids {
		if !isObjectID(id) {
			return userError("“" + id + "” isn't a file's ID.")
		}
	}
	hosts := func() []string { return remoteHosts(context.Background(), r.Git) }
	err = a.track(downloadKey(token), r.Path, "download", hosts, "That download is already running.",
		func(ctx context.Context, o git.NetOptions) error { return r.Git.Download(ctx, ids, o) })
	return ownLinksOnly(err, hosts)
}

// InsightsDownload fetches, in one request, the files a partial clone's Insights read for
// these inputs and hasn't got (#1010): both sides of the window's changes, for its lines, and
// each open branch's changes and dry-run merge. Insights reads none of them by itself; this is
// for when someone asks ("Download two weeks of changes"). Then the results are read again.
func (a *App) InsightsDownload(path, token string, in InsightsInputs) error {
	if token == "" {
		return userError("A download needs a name.")
	}
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	g, root, err := a.repoGit(ctx, path)
	if err != nil {
		return err
	}
	rules := a.branchRules(root)
	w, err := resolveInputs(ctx, g, in, rules)
	if err != nil {
		return err
	}
	ids, err := g.WindowNeeds(ctx, w.query())
	if err != nil {
		return err
	}
	open, err := g.OpenBranches(git.NoDownloads(ctx), w.Target, git.MaxSignalBranches, endStates(rules))
	if err != nil {
		return err
	}
	for _, b := range open {
		more, err := g.BranchNeeds(ctx, w.Target, b)
		if err != nil {
			return err
		}
		ids = append(ids, more...)
	}
	if len(ids) > 0 {
		slices.Sort(ids)
		ids = slices.Compact(ids)
		hosts := func() []string { return remoteHosts(context.Background(), r.Git) }
		err = a.track(downloadKey(token), r.Path, "download", hosts, "That download is already running.",
			func(ctx context.Context, o git.NetOptions) error { return r.Git.Download(ctx, ids, o) })
		if err := ownLinksOnly(err, hosts); err != nil {
			return err
		}
	}
	a.insights.clear()
	return nil
}

// CancelDownload stops the download token names.
func (a *App) CancelDownload(token string) {
	a.cancelNetwork(downloadKey(token))
}

// isObjectID reports whether s is a full object ID, SHA-1 or SHA-256.
func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	return strings.Trim(s, "0123456789abcdef") == ""
}

// BlameNeeds is what blaming a file in a partial clone would download first.
type BlameNeeds struct {
	// Partial is false for a clone that has everything: blame needs nothing.
	Partial bool `json:"partial"`
	// Versions counts the file's versions in the history blamed.
	Versions int `json:"versions"`
	// Missing are the versions never downloaded, to fetch in one request before blaming.
	Missing []string `json:"missing"`
}

// BlameNeeds says what blaming file at rev would download, so the view can ask first:
// blame reads every version of a file, and git would fetch each one on its own (blame on
// the kernel's Makefile made 1,888 downloads in 15 minutes; its versions in one request
// take seconds).
func (a *App) BlameNeeds(path, file, rev string) (BlameNeeds, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return BlameNeeds{}, err
	}
	ctx := context.Background()
	if !r.Git.Partial(ctx) {
		return BlameNeeds{Missing: []string{}}, nil
	}
	ids, err := r.Git.FileVersions(ctx, file, rev)
	if err != nil {
		return BlameNeeds{}, err
	}
	missing, err := r.Git.MissingObjects(ctx, ids)
	if err != nil {
		return BlameNeeds{}, err
	}
	if missing == nil {
		missing = []string{}
	}
	return BlameNeeds{Partial: true, Versions: len(ids), Missing: missing}, nil
}
