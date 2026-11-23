// Package weblinks builds the addresses of a remote's pages on its forge: the repository,
// its branches, one branch and a comparison of two branches. They're built from the remote
// URL and nothing else; nothing here contacts the forge. There are no links that start a
// pull or merge request: the app doesn't take part in them, and a link that seemed to would
// only confuse.
//
// A forge is known by its host (github.com, gitlab.com and the like) or from a forge rule in
// git config, never guessed from a host's name. A remote on a host with neither has no web
// links.
package weblinks

import (
	"net/url"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/links"
)

// Site is a repository's home on a forge.
type Site struct {
	Forge git.Forge `json:"forge"`
	// Repo is the repository's page, such as https://github.com/owner/repo.
	Repo string `json:"repo"`
	// Owner is who the repository belongs to there, the path before its name, for naming a
	// fork's branch in a comparison across forks.
	Owner string `json:"owner"`
	// Name is the repository's own name, the last part of its path.
	Name string `json:"name"`
	host string
}

// For returns where a remote URL's repository is on the web, or false when its forge isn't
// known by its host or a rule in cfg.
func For(remoteURL string, cfg links.Config) (Site, bool) {
	loc := git.ParseLocation(remoteURL)
	if loc.Place == git.PlaceLocal || loc.Path == "" {
		return Site{}, false
	}
	path := loc.Path
	// A URL's path is written escaped, as in Azure's "My%20Project": it's escaped again,
	// one part at a time, below.
	if p, err := url.PathUnescape(path); err == nil {
		path = p
	}
	scheme := "https://"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(remoteURL)), "http://") {
		scheme = "http://"
	}
	base := scheme + loc.Host
	if loc.Port != "" {
		base += ":" + loc.Port
	}
	forge := loc.Forge
	if rule, ok := cfg.ForgeFor(loc.Host); ok {
		forge = rule.Forge
		if rule.Web != "" {
			base = rule.Web
			// A forge served under a path is cloned from under it over https: the path
			// is in the remote URL already.
			if u, err := url.Parse(rule.Web); err == nil {
				if prefix := strings.Trim(u.Path, "/"); prefix != "" {
					path = strings.TrimPrefix(path, prefix+"/")
				}
			}
		}
		if forge == git.ForgeBitbucketServer {
			if path = bitbucketServerPath(path); path == "" {
				return Site{}, false
			}
		}
	} else if loc.Forge == git.ForgeNone {
		return Site{}, false
	} else if loc.Place != git.PlaceWeb {
		base = "https://" + loc.Host
	}
	s := Site{Forge: forge, Repo: base + "/" + escapePath(path), host: loc.Host}
	if i := strings.LastIndex(loc.Path, "/"); i >= 0 {
		s.Owner, s.Name = loc.Path[:i], loc.Path[i+1:]
	} else {
		s.Name = loc.Path
	}
	if forge == git.ForgeBitbucketServer || forge == git.ForgeAzure {
		// Neither compares across forks by owner, and their paths aren't owner/name.
		s.Owner = ""
	}
	return s, true
}

// bitbucketServerPath turns a Bitbucket Data Center clone path, scm/PROJ/repo over https or
// proj/repo over SSH, into its web path, projects/PROJ/repos/repo. A personal repository,
// ~user/repo, lives under users/user. It's "" for a path that's neither.
func bitbucketServerPath(p string) string {
	p = strings.TrimPrefix(p, "scm/")
	owner, name, ok := strings.Cut(p, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return ""
	}
	if user, ok := strings.CutPrefix(owner, "~"); ok {
		return "users/" + user + "/repos/" + name
	}
	return "projects/" + strings.ToUpper(owner) + "/repos/" + name
}

// escapePath escapes each part of a slash-separated path, so a branch named "feature/a b"
// keeps its slash and has its space escaped.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// Branches returns the page that lists the repository's branches.
func (s Site) Branches() string {
	if s.Forge == git.ForgeGitLab {
		return s.Repo + "/-/branches"
	}
	return s.Repo + "/branches"
}

// Branch returns a branch's page.
func (s Site) Branch(branch string) string {
	switch s.Forge {
	case git.ForgeGitHub:
		return s.Repo + "/tree/" + escapePath(branch)
	case git.ForgeGitLab:
		return s.Repo + "/-/tree/" + escapePath(branch)
	case git.ForgeGitea, git.ForgeForgejo, git.ForgeCodeberg:
		return s.Repo + "/src/branch/" + escapePath(branch)
	case git.ForgeBitbucket:
		return s.Repo + "/src/" + escapePath(branch)
	case git.ForgeBitbucketServer:
		return s.Repo + "/browse?at=" + url.QueryEscape("refs/heads/"+branch)
	case git.ForgeAzure:
		return s.Repo + "?version=GB" + url.QueryEscape(branch)
	}
	return ""
}

// Base is what a branch is compared with: a branch of the
// remote's own repository, or of another remote's on the same forge that it's a fork of.
type Base struct {
	Site   Site
	Branch string
}

// crossFork says whether a comparison of a branch of s with base spans two repositories,
// and whether the forge can show that from the base's side. Only forges that name a fork's
// branch as owner:branch can.
func (s Site) crossFork(base Base) (cross, ok bool) {
	if base.Site.Repo == "" || base.Site.Repo == s.Repo {
		return false, true
	}
	switch s.Forge {
	case git.ForgeGitHub, git.ForgeGitea, git.ForgeForgejo, git.ForgeCodeberg:
		return true, s.Owner != "" && base.Site.Forge == s.Forge
	}
	return true, false
}

// Compare returns the page comparing a branch with a base, or "" when the forge can't show
// it: no base branch is known, or the base is another repository the forge can't compare
// with by address.
func (s Site) Compare(branch string, base Base) string {
	if base.Branch == "" {
		return ""
	}
	cross, ok := s.crossFork(base)
	if !ok {
		return ""
	}
	if cross {
		return base.Site.Repo + "/compare/" + escapePath(base.Branch) + "..." + url.PathEscape(s.Owner) + ":" + escapePath(branch)
	}
	switch s.Forge {
	case git.ForgeGitHub, git.ForgeGitea, git.ForgeForgejo, git.ForgeCodeberg:
		return s.Repo + "/compare/" + escapePath(base.Branch) + "..." + escapePath(branch)
	case git.ForgeGitLab:
		return s.Repo + "/-/compare/" + escapePath(base.Branch) + "..." + escapePath(branch)
	case git.ForgeBitbucket:
		return s.Repo + "/branches/compare/" + escapePath(branch) + "%0D" + escapePath(base.Branch)
	case git.ForgeBitbucketServer:
		return s.Repo + "/compare/commits?sourceBranch=" + url.QueryEscape("refs/heads/"+branch) + "&targetBranch=" + url.QueryEscape("refs/heads/"+base.Branch)
	case git.ForgeAzure:
		return s.Repo + "/branchCompare?baseVersion=GB" + url.QueryEscape(base.Branch) + "&targetVersion=GB" + url.QueryEscape(branch)
	}
	return ""
}

// Issues returns an address for the repository's issues with "{n}" for the number, or ""
// for a forge whose issues aren't numbered pages of the repository.
func (s Site) Issues() string {
	switch s.Forge {
	case git.ForgeGitHub, git.ForgeGitea, git.ForgeForgejo, git.ForgeCodeberg:
		return s.Repo + "/issues/{n}"
	case git.ForgeGitLab:
		return s.Repo + "/-/issues/{n}"
	}
	return ""
}

// ForkOf says whether s looks like a fork of other: another repository with the same name on
// the same forge and host.
func (s Site) ForkOf(other Site) bool {
	return s.Repo != other.Repo && s.Forge == other.Forge && s.host == other.host && s.Name != "" && strings.EqualFold(s.Name, other.Name)
}
