package app

import (
	"context"
	"slices"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/links"
	"github.com/git-revenge/gitrevenge/internal/weblinks"
)

// RemoteWeb is a remote's pages on its forge, for the remote's menu.
type RemoteWeb struct {
	Remote string    `json:"remote"`
	Forge  git.Forge `json:"forge"`
	// Repo and Branches are the repository's page and its list of branches.
	Repo     string `json:"repo"`
	Branches string `json:"branches"`
	// DefaultBranch is the branch refs/remotes/<name>/HEAD points at, or "" when git
	// hasn't heard (git remote set-head <name> --auto asks).
	DefaultBranch string `json:"defaultBranch"`
	// CloneHTTPS and CloneSSH are the addresses to clone it from over each protocol, for
	// copying, or "" where that isn't known.
	CloneHTTPS string `json:"cloneHttps"`
	CloneSSH   string `json:"cloneSsh"`
	// Bases are what the remote's branches can be compared with:
	// the remote's own default branch first, then those of remotes it looks like a fork of.
	Bases []WebBase `json:"bases"`
}

// WebBase is one remote's branch a comparison can be made with.
type WebBase struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
}

// BranchWeb is a remote branch's pages: the branch itself, and a comparison with each base.
// A link the forge can't make by address is "".
type BranchWeb struct {
	Branch string          `json:"branch"`
	Bases  []BranchWebBase `json:"bases"`
}

// BranchWebBase is a branch's comparison with one base.
type BranchWebBase struct {
	WebBase
	Compare string `json:"compare"`
}

// remoteSites reads where each remote is on the web, the default branches, and the bases
// each remote's branches can be compared with. Remotes with no known forge are left out.
func (a *App) remoteSites(ctx context.Context, path string) ([]RemoteWeb, map[string]weblinks.Site, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, nil, err
	}
	remotes, err := r.Git.Remotes(ctx)
	if err != nil {
		return nil, nil, err
	}
	cfg := a.linkConfig(ctx, r.Git)
	heads := r.Git.RemoteHeads(ctx)
	sites := map[string]weblinks.Site{}
	var order []string
	for _, rem := range remotes {
		if s, ok := weblinks.For(rem.FetchURL, cfg); ok {
			sites[rem.Name] = s
			order = append(order, rem.Name)
		}
	}
	out := []RemoteWeb{}
	for _, name := range order {
		s := sites[name]
		w := RemoteWeb{Remote: name, Forge: s.Forge, Repo: s.Repo, Branches: s.Branches(), DefaultBranch: heads[name], Bases: []WebBase{}}
		w.CloneHTTPS, w.CloneSSH = s.Clone()
		w.Bases = append(w.Bases, WebBase{Remote: name, Branch: heads[name]})
		for _, other := range order {
			// Only a base the forge can compare with by address is offered.
			if o := sites[other]; other != name && heads[other] != "" && s.ForkOf(o) &&
				s.Compare("x", weblinks.Base{Site: o, Branch: heads[other]}) != "" {
				w.Bases = append(w.Bases, WebBase{Remote: other, Branch: heads[other]})
			}
		}
		out = append(out, w)
	}
	return out, sites, nil
}

// RemoteWebLinks returns the pages of each remote on a forge known by its host or a forge
// rule. Remotes elsewhere, or folders on this machine, aren't listed.
func (a *App) RemoteWebLinks(path string) ([]RemoteWeb, error) {
	out, _, err := a.remoteSites(context.Background(), path)
	return out, err
}

// BranchWebLinks returns a remote branch's pages: branch is its name on the remote, such as
// "feature/x" for origin/feature/x. The result is empty when the remote has no web links.
func (a *App) BranchWebLinks(path, remote, branch string) (BranchWeb, error) {
	remotes, sites, err := a.remoteSites(context.Background(), path)
	res := BranchWeb{Bases: []BranchWebBase{}}
	if err != nil {
		return res, err
	}
	s, ok := sites[remote]
	if !ok {
		return res, nil
	}
	res.Branch = s.Branch(branch)
	for _, w := range remotes {
		if w.Remote != remote {
			continue
		}
		for _, b := range w.Bases {
			base := weblinks.Base{Site: sites[b.Remote], Branch: b.Branch}
			if b.Remote == remote && b.Branch == branch {
				// A branch isn't compared with itself.
				continue
			}
			res.Bases = append(res.Bases, BranchWebBase{WebBase: b, Compare: s.Compare(branch, base)})
		}
	}
	return res, nil
}

// SaveForgeRule writes a forge rule to its config. was is its host before an edit in the
// same config, or "" for a new rule.
func (a *App) SaveForgeRule(path string, rule links.ForgeRule, was string) error {
	ctx := context.Background()
	g, inRepo, err := a.linkGit(ctx, path)
	if err != nil {
		return err
	}
	if rule.Scope == links.Local && !inRepo {
		return userError("Open a repository to keep a forge in its own config.")
	}
	return linkErr(links.SaveForge(ctx, g, rule, was))
}

// RemoveForgeRule deletes the forge rule for a host from its config.
func (a *App) RemoveForgeRule(path, scope, host string) error {
	ctx := context.Background()
	g, _, err := a.linkGit(ctx, path)
	if err != nil {
		return err
	}
	return writeErr(links.RemoveForge(ctx, g, scope, host))
}

// UnknownHosts returns the hosts of a repository's remotes that have no web links: servers
// that aren't a forge known by host and have no forge rule. Settings offers to say what
// forge each is.
func (a *App) UnknownHosts(path string) ([]string, error) {
	out := []string{}
	if path == "" {
		return out, nil
	}
	ctx := context.Background()
	r, err := a.openRepo(path)
	if err != nil {
		return out, err
	}
	remotes, err := r.Git.Remotes(ctx)
	if err != nil {
		return out, err
	}
	cfg := a.linkConfig(ctx, r.Git)
	for _, rem := range remotes {
		loc := git.ParseLocation(rem.FetchURL)
		if loc.Place == git.PlaceLocal || loc.Host == "" {
			continue
		}
		if _, ok := weblinks.For(rem.FetchURL, cfg); ok {
			continue
		}
		if _, ruled := cfg.ForgeFor(loc.Host); ruled || slices.Contains(out, loc.Host) {
			continue
		}
		out = append(out, loc.Host)
	}
	return out, nil
}

// URLPlace is where one remote URL points, for the Remotes panel's header: the kind of place,
// its host, and its forge and repository page when it opens on the web, from a forge known
// by host or a forge rule. Forge is "" when it doesn't, so a mark is shown only where Open on
// the web works.
type URLPlace struct {
	Place git.Place `json:"place"`
	Host  string    `json:"host"`
	Forge git.Forge `json:"forge"`
	Web   string    `json:"web"`
}

// RemotePlaces returns where each of a repository's remote URLs points, fetch and push alike,
// by URL.
func (a *App) RemotePlaces(path string) (map[string]URLPlace, error) {
	out := map[string]URLPlace{}
	ctx := context.Background()
	r, err := a.openRepo(path)
	if err != nil {
		return out, err
	}
	remotes, err := r.Git.Remotes(ctx)
	if err != nil {
		return out, err
	}
	cfg := a.linkConfig(ctx, r.Git)
	for _, rem := range remotes {
		for _, u := range append([]string{rem.FetchURL}, rem.PushURLs...) {
			if _, done := out[u]; done || u == "" {
				continue
			}
			loc := git.ParseLocation(u)
			p := URLPlace{Place: loc.Place, Host: loc.Host}
			if s, ok := weblinks.For(u, cfg); ok {
				p.Forge, p.Web = s.Forge, s.Repo
			}
			out[u] = p
		}
	}
	return out, nil
}
