package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/links"
)

// linkConfig reads the smart links that apply to a repository. A read that fails gives no
// rules, so links never stop the rest of the app working.
func (a *App) linkConfig(ctx context.Context, g *git.Repo) links.Config {
	c, err := links.Load(ctx, g, true)
	if err != nil {
		return links.Config{Rules: []links.Rule{}, Web: true}
	}
	return c
}

// linkGit is where link rules are read and written: the repository at path, or, with no
// path, a place outside any repository where only the global config applies.
func (a *App) linkGit(ctx context.Context, path string) (*git.Repo, bool, error) {
	if path != "" {
		g, _, err := a.repoGit(ctx, path)
		return g, true, err
	}
	c, err := a.gitConfig()
	if err != nil {
		return nil, false, err
	}
	return git.OpenRepo(c.runner, c.workDir), false, nil
}

// LinkRules returns the smart links for a repository, its own and global ones, or only the
// global ones when path is "".
func (a *App) LinkRules(path string) (links.Config, error) {
	ctx := context.Background()
	g, inRepo, err := a.linkGit(ctx, path)
	if err != nil {
		return links.Config{}, err
	}
	return links.Load(ctx, g, inRepo)
}

// SaveLinkRule writes a rule to its config. was is its name before an edit in the same
// config, or "" for a new rule.
func (a *App) SaveLinkRule(path string, rule links.Rule, was string) error {
	ctx := context.Background()
	g, inRepo, err := a.linkGit(ctx, path)
	if err != nil {
		return err
	}
	if rule.Scope == links.Local && !inRepo {
		return userError("Open a repository to keep a link in its own config.")
	}
	return linkErr(links.Save(ctx, g, rule, was))
}

// RemoveLinkRule deletes a rule from its config.
func (a *App) RemoveLinkRule(path, scope, name string) error {
	ctx := context.Background()
	g, _, err := a.linkGit(ctx, path)
	if err != nil {
		return err
	}
	return writeErr(links.Remove(ctx, g, scope, name))
}

// ReorderLinkRules puts one config's rules in the given order.
func (a *App) ReorderLinkRules(path, scope string, names []string) error {
	ctx := context.Background()
	g, _, err := a.linkGit(ctx, path)
	if err != nil {
		return err
	}
	return writeErr(links.Reorder(ctx, g, scope, names))
}

// SetWebLinks turns links for web addresses on or off, in the global config.
func (a *App) SetWebLinks(path string, on bool) error {
	ctx := context.Background()
	g, _, err := a.linkGit(ctx, path)
	if err != nil {
		return err
	}
	return writeErr(links.SetWeb(ctx, g, on))
}

// LinkTry is what a rule would match in a repository's recent history.
type LinkTry struct {
	// Scanned is how many recent commits were read; Commits how many had a match.
	Scanned int           `json:"scanned"`
	Commits int           `json:"commits"`
	Matches []links.Match `json:"matches"`
	Problem string        `json:"problem,omitempty"`
}

// tryCommits is how far back TryLinkRule looks.
const tryCommits = 500

// TryLinkRule runs a rule over the last commits' messages, without saving it, and returns
// how many matched and a few distinct matches with their addresses.
func (a *App) TryLinkRule(path string, rule links.Rule) (LinkTry, error) {
	if err := links.Check(rule); err != nil {
		return LinkTry{Problem: err.Error(), Matches: []links.Match{}}, nil
	}
	res := LinkTry{Matches: []links.Match{}}
	if path == "" {
		return res, nil
	}
	ctx := context.Background()
	g, _, err := a.repoGit(ctx, path)
	if err != nil {
		return res, err
	}
	out, err := g.Read(ctx, "log", fmt.Sprintf("-%d", tryCommits), "--format=%B%x00", "--")
	if err != nil {
		// A repository with no commits yet has nothing to try.
		return res, nil
	}
	rule.Enabled, rule.Shadowed, rule.Problem = true, false, ""
	rule.In = links.AllPlaces
	c := links.Config{Rules: []links.Rule{rule}}
	seen := map[string]bool{}
	for msg := range strings.SplitSeq(string(out), "\x00") {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		res.Scanned++
		found := c.Find(msg, links.InMessage)
		if len(found) > 0 {
			res.Commits++
		}
		for _, m := range found {
			if !seen[m.Text] && len(res.Matches) < 5 {
				seen[m.Text] = true
				res.Matches = append(res.Matches, m)
			}
		}
	}
	return res, nil
}

// ImportLinkRules reads rules exported as JSON from a file and saves them into one config,
// replacing rules of the same name there. It returns how many it saved.
func (a *App) ImportLinkRules(path, file, scope string) (int, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	var rules []links.Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return 0, userError("That file isn't a list of links exported from Git Revenge.")
	}
	ctx := context.Background()
	g, inRepo, err := a.linkGit(ctx, path)
	if err != nil {
		return 0, err
	}
	if scope == links.Local && !inRepo {
		return 0, userError("Open a repository to keep links in its own config.")
	}
	have, err := links.Load(ctx, g, inRepo)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rules {
		r.Scope, r.Shadowed, r.Problem = scope, false, ""
		was := ""
		for _, h := range have.Rules {
			if h.Scope == scope && h.Name == r.Name {
				was = r.Name
			}
		}
		if err := links.Save(ctx, g, r, was); err != nil {
			return n, userError(fmt.Sprintf("Saved %d of %d. %q couldn't be saved: %v", n, len(rules), r.Name, err))
		}
		n++
	}
	return n, nil
}

func linkErr(err error) error {
	if errors.Is(err, links.ErrNameTaken) {
		return userError("There's already a link with that name in that config. Choose another name.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) || err == nil {
		return writeErr(err)
	}
	return userError(capitalize(err.Error()) + ".")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
