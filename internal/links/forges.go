package links

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// ForgeSection holds one forge rule per host: what kind of forge a self-hosted server is, so
// its remotes get web links. A host is never guessed from its name; without a rule a
// self-hosted server has no web links.
//
//	[gitrevenge-forge "git.acme.com"]
//		type = gitlab
//		web = https://git.acme.com/gitlab
//
// web is optional: where the forge's pages are when that isn't https:// and the remote's
// host, such as an SSH alias from ~/.ssh/config, or a forge served under a path. A rule in
// the repository's config replaces a global rule for the same host.
const ForgeSection = "gitrevenge-forge"

// ForgeKinds are the kinds of forge a rule can name. Bitbucket Data Center is the self-hosted Bitbucket,
// whose pages differ from bitbucket.org's; Azure DevOps Server is the self-hosted Azure.
var ForgeKinds = []git.Forge{git.ForgeGitHub, git.ForgeGitLab, git.ForgeGitea, git.ForgeForgejo, git.ForgeBitbucketServer, git.ForgeAzure}

// ForgeRule says what forge a host is.
type ForgeRule struct {
	// Host is the remote URL's host, as written there, in lower case.
	Host  string    `json:"host"`
	Forge git.Forge `json:"forge"`
	// Web is where the forge's pages are, or "" for https:// and the host.
	Web      string `json:"web,omitempty"`
	Scope    string `json:"scope"`
	Shadowed bool   `json:"shadowed,omitempty"`
	Problem  string `json:"problem,omitempty"`
}

var hostName = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`)

// CheckForge says what's wrong with a forge rule, or nil.
func CheckForge(f ForgeRule) error {
	switch {
	case !hostName.MatchString(f.Host):
		return errors.New("the host is a name like git.example.com, without a scheme, port or path")
	case !slices.Contains(ForgeKinds, f.Forge):
		return fmt.Errorf("%q isn't a forge this app knows the pages of", f.Forge)
	}
	if f.Web == "" {
		return nil
	}
	w := strings.ToLower(f.Web)
	if !strings.HasPrefix(w, "https://") && !strings.HasPrefix(w, "http://") {
		return errors.New("the web address must start with https:// or http://")
	}
	if strings.ContainsAny(f.Web, " \t\n?#{}") {
		return errors.New("the web address is the forge's own address, without spaces, a query or placeholders")
	}
	return nil
}

// ForgeFor returns the rule for a host, from the rules in force, or false.
func (c Config) ForgeFor(host string) (ForgeRule, bool) {
	host = strings.ToLower(host)
	for _, f := range c.Forges {
		if f.Host == host && !f.Shadowed && f.Problem == "" {
			return f, true
		}
	}
	return ForgeRule{}, false
}

// readForges returns one config file's forge rules.
func readForges(ctx context.Context, g *git.Repo, scope string) ([]ForgeRule, error) {
	out, err := g.Read(ctx, "config", scope, "-z", "--get-regexp", `^`+regexp.QuoteMeta(ForgeSection)+`\.`)
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.ExitCode == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rules []*ForgeRule
	byHost := map[string]*ForgeRule{}
	for entry := range strings.SplitSeq(string(out), "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
		if key == "" || first < 0 || first == last {
			continue
		}
		host, field := strings.ToLower(key[first+1:last]), strings.ToLower(key[last+1:])
		f := byHost[host]
		if f == nil {
			f = &ForgeRule{Host: host, Scope: strings.TrimPrefix(scope, "--")}
			byHost[host] = f
			rules = append(rules, f)
		}
		switch field {
		case "type":
			f.Forge = git.Forge(strings.ToLower(strings.TrimSpace(value)))
		case "web":
			f.Web = strings.TrimRight(strings.TrimSpace(value), "/")
		}
	}
	res := make([]ForgeRule, 0, len(rules))
	for _, f := range rules {
		if err := CheckForge(*f); err != nil {
			f.Problem = err.Error()
		}
		res = append(res, *f)
	}
	return res, nil
}

// loadForges reads the global rules and, in a repository, its own, marking global rules a
// repository rule replaces.
func loadForges(ctx context.Context, g *git.Repo, inRepo bool) ([]ForgeRule, error) {
	global, err := readForges(ctx, g, "--global")
	if err != nil {
		return nil, err
	}
	var local []ForgeRule
	if inRepo {
		if local, err = readForges(ctx, g, "--local"); err != nil {
			return nil, err
		}
	}
	for i := range global {
		if slices.ContainsFunc(local, func(f ForgeRule) bool { return f.Host == global[i].Host }) {
			global[i].Shadowed = true
		}
	}
	return append(append([]ForgeRule{}, local...), global...), nil
}

// SaveForge writes a forge rule to its scope, replacing any rule for the same host there.
// With was set to the host it had before in the same scope, that rule is removed first.
func SaveForge(ctx context.Context, g *git.Repo, f ForgeRule, was string) error {
	f.Host = strings.ToLower(strings.TrimSpace(f.Host))
	f.Web = strings.TrimRight(strings.TrimSpace(f.Web), "/")
	if err := CheckForge(f); err != nil {
		return err
	}
	flag, err := scopeFlag(f.Scope)
	if err != nil {
		return err
	}
	if was != "" && strings.ToLower(was) != f.Host {
		if err := RemoveForge(ctx, g, f.Scope, was); err != nil {
			return err
		}
	}
	key := func(field string) string { return ForgeSection + "." + f.Host + "." + field }
	if err := config(ctx, g, flag, "--replace-all", key("type"), string(f.Forge)); err != nil {
		return err
	}
	if f.Web == "" {
		return unset(ctx, g, flag, key("web"))
	}
	return config(ctx, g, flag, "--replace-all", key("web"), f.Web)
}

// RemoveForge deletes the rule for a host from a scope. A host with no rule there is fine.
func RemoveForge(ctx context.Context, g *git.Repo, scope, host string) error {
	flag, err := scopeFlag(scope)
	if err != nil {
		return err
	}
	err = config(ctx, g, flag, "--remove-section", ForgeSection+"."+strings.ToLower(host))
	var gerr *git.Error
	// Exit 128: no such section.
	if errors.As(err, &gerr) && gerr.ExitCode == 128 && strings.Contains(gerr.Stderr, "no such section") {
		return nil
	}
	return err
}
