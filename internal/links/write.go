package links

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

func scopeFlag(scope string) (string, error) {
	switch scope {
	case Local:
		return "--local", nil
	case Global:
		return "--global", nil
	}
	return "", fmt.Errorf("unknown config %q", scope)
}

func key(name, field string) string { return Section + "." + name + "." + field }

func config(ctx context.Context, g *git.Repo, args ...string) error {
	_, err := g.Write(ctx, git.Cmd{Args: append([]string{"config"}, args...)})
	return err
}

// ErrNameTaken means a rule with that name is already in the config it's going to.
var ErrNameTaken = errors.New("a link with that name is already there")

// Save writes a rule to its scope. With was set to the name it had before in the same
// scope, it's renamed in place, keeping its position; a new rule goes last.
func Save(ctx context.Context, g *git.Repo, r Rule, was string) error {
	if err := Check(r); err != nil {
		return err
	}
	flag, err := scopeFlag(r.Scope)
	if err != nil {
		return err
	}
	existing, err := read(ctx, g, flag)
	if err != nil {
		return err
	}
	has := func(n string) bool { return slices.ContainsFunc(existing, func(x Rule) bool { return x.Name == n }) }
	switch {
	case was != "" && was != r.Name && has(was):
		if has(r.Name) {
			return ErrNameTaken
		}
		if err := config(ctx, g, flag, "--rename-section", Section+"."+was, Section+"."+r.Name); err != nil {
			return err
		}
	case was == "" && has(r.Name):
		return ErrNameTaken
	}
	return writeFields(ctx, g, flag, r)
}

func writeFields(ctx context.Context, g *git.Repo, flag string, r Rule) error {
	in := r.In
	if len(in) == 0 {
		in = AllPlaces
	}
	sets := [][2]string{
		{"pattern", r.Pattern},
		{"url", strings.TrimSpace(r.URL)},
		{"in", strings.Join(in, " ")},
		{"shipped", fmt.Sprint(r.Shipped)},
	}
	for _, s := range sets {
		if err := config(ctx, g, flag, "--replace-all", key(r.Name, s[0]), s[1]); err != nil {
			return err
		}
	}
	if r.Enabled {
		// Unset: on is the default. Exit 5 means it wasn't set.
		return unset(ctx, g, flag, key(r.Name, "enabled"))
	}
	return config(ctx, g, flag, "--replace-all", key(r.Name, "enabled"), "false")
}

// Remove deletes a rule from its scope.
func Remove(ctx context.Context, g *git.Repo, scope, name string) error {
	flag, err := scopeFlag(scope)
	if err != nil {
		return err
	}
	return config(ctx, g, flag, "--remove-section", Section+"."+name)
}

// Reorder rewrites one scope's rules in the given order of names. Rules not named keep
// their place after them. Comments inside the rules' sections are lost.
func Reorder(ctx context.Context, g *git.Repo, scope string, names []string) error {
	flag, err := scopeFlag(scope)
	if err != nil {
		return err
	}
	rules, err := read(ctx, g, flag)
	if err != nil {
		return err
	}
	pos := func(n string) int {
		if i := slices.Index(names, n); i >= 0 {
			return i
		}
		return len(names)
	}
	slices.SortStableFunc(rules, func(a, b Rule) int { return pos(a.Name) - pos(b.Name) })
	for _, r := range rules {
		if err := config(ctx, g, flag, "--remove-section", Section+"."+r.Name); err != nil {
			return err
		}
	}
	for _, r := range rules {
		r.Problem = ""
		if err := writeFields(ctx, g, flag, r); err != nil {
			return err
		}
	}
	return nil
}

// SetWeb turns links for web addresses on or off in the global config.
func SetWeb(ctx context.Context, g *git.Repo, on bool) error {
	if on {
		return unset(ctx, g, "--global", WebKey)
	}
	return config(ctx, g, "--global", "--replace-all", WebKey, "false")
}

// unset removes a key; a key that wasn't set (git's exit 5) is fine.
func unset(ctx context.Context, g *git.Repo, flag, k string) error {
	err := config(ctx, g, flag, "--unset-all", k)
	var gerr *git.Error
	if errors.As(err, &gerr) && gerr.ExitCode == 5 {
		return nil
	}
	return err
}
