package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// FetchRules is how a remote is fetched (#925), as the Remotes dialog shows it. Each is plain
// git config on the remote.
type FetchRules struct {
	// SkipFetchAll leaves the remote out of fetching every remote (remote.<name>.skipFetchAll).
	// It can still be fetched on its own.
	SkipFetchAll bool `json:"skipFetchAll"`
	// Prune is whether a fetch drops the remote's deleted branches, as it will happen:
	// remote.<name>.prune, else fetch.prune, else the app's own default of yes (#923).
	Prune bool `json:"prune"`
	// TagOpt is remote.<name>.tagOpt: "" for tags with their commits, "--tags" for every tag,
	// "--no-tags" for none.
	TagOpt string `json:"tagOpt"`
	// AllBranches is the usual refspec, every branch; otherwise Patterns are the branches
	// fetched, such as "main" and "release/*".
	AllBranches bool     `json:"allBranches"`
	Patterns    []string `json:"patterns"`
	// Other are refspecs that aren't a branch pattern, listed as they are and kept as they
	// are: the app didn't write them and won't rewrite them.
	Other []string `json:"other"`
}

// branchPattern reads a refspec of the usual shape, +refs/heads/<p>:refs/remotes/<name>/<p>,
// as its pattern p, or false for any other refspec.
func branchPattern(name, spec string) (string, bool) {
	src, dst, ok := strings.Cut(strings.TrimPrefix(spec, "+"), ":")
	if !ok || strings.HasPrefix(spec, "^") {
		return "", false
	}
	p, ok1 := strings.CutPrefix(src, "refs/heads/")
	q, ok2 := strings.CutPrefix(dst, "refs/remotes/"+name+"/")
	if !ok1 || !ok2 || p != q || p == "" {
		return "", false
	}
	return p, true
}

// ReadFetchRules reads a remote's fetch rules from its details (RemoteDetails) and fetch.prune
// (nil when no config file sets it).
func ReadFetchRules(rm Remote, fetchPrune *bool) FetchRules {
	fr := FetchRules{SkipFetchAll: rm.SkipFetchAll, TagOpt: rm.TagOpt, Patterns: []string{}, Other: []string{}}
	switch {
	case rm.Prune != nil:
		fr.Prune = *rm.Prune
	case fetchPrune != nil:
		fr.Prune = *fetchPrune
	default:
		fr.Prune = true
	}
	for _, spec := range rm.FetchRefspecs {
		p, ok := branchPattern(rm.Name, spec)
		switch {
		case ok && p == "*":
			fr.AllBranches = true
		case ok:
			fr.Patterns = append(fr.Patterns, p)
		default:
			fr.Other = append(fr.Other, spec)
		}
	}
	// A remote fetching nothing by pattern fetches nothing at all; with no refspecs whatever,
	// git fetches only HEAD, which reads best as every branch not being chosen.
	if len(rm.FetchRefspecs) == 0 {
		fr.AllBranches = false
	}
	return fr
}

// ErrBadPattern means a branch pattern isn't a name git can fetch by.
var ErrBadPattern = errors.New("not a branch pattern git can fetch")

// SetFetchRules writes a remote's fetch rules. Other refspecs are kept as they are. Prune is
// written only where it differs from what fetch.prune and the app's default would do.
func (r *Repo) SetFetchRules(ctx context.Context, name string, fr FetchRules) error {
	if err := CheckRevs(name); err != nil {
		return err
	}
	if !fr.AllBranches {
		if len(fr.Patterns) == 0 && len(fr.Other) == 0 {
			return fmt.Errorf("choose at least one branch to fetch: %w", ErrBadPattern)
		}
		for _, p := range fr.Patterns {
			if p == "" || strings.Count(p, "*") > 1 || strings.ContainsAny(p, " :^\\") {
				return fmt.Errorf("%q: %w", p, ErrBadPattern)
			}
			if _, err := r.Read(ctx, "check-ref-format", "--refspec-pattern", "refs/heads/"+p); err != nil {
				return fmt.Errorf("%q: %w", p, ErrBadPattern)
			}
		}
	}
	key := func(k string) string { return "remote." + name + "." + k }
	set := func(k, v string) error {
		_, err := r.Write(ctx, Cmd{Args: []string{"config", "--local", key(k), v}})
		return err
	}
	unset := func(k string) error {
		_, err := r.Write(ctx, Cmd{Args: []string{"config", "--local", "--unset-all", key(k)}})
		var gerr *Error
		if errors.As(err, &gerr) && gerr.ExitCode == 5 {
			return nil
		}
		return err
	}

	if fr.SkipFetchAll {
		if err := set("skipFetchAll", "true"); err != nil {
			return err
		}
	} else if err := unset("skipFetchAll"); err != nil {
		return err
	}

	// What a fetch would do with no remote.<name>.prune: fetch.prune, else the app's yes.
	without := true
	if v, err := r.ReadString(ctx, "config", "--type=bool", "--get", "fetch.prune"); err == nil {
		without = v == "true"
	}
	if fr.Prune == without {
		if err := unset("prune"); err != nil {
			return err
		}
	} else if err := set("prune", fmt.Sprint(fr.Prune)); err != nil {
		return err
	}

	switch fr.TagOpt {
	case "":
		if err := unset("tagOpt"); err != nil {
			return err
		}
	case "--tags", "--no-tags":
		if err := set("tagOpt", fr.TagOpt); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown tag option %q", fr.TagOpt)
	}

	specs := []string{}
	if fr.AllBranches {
		specs = append(specs, "+refs/heads/*:refs/remotes/"+name+"/*")
	} else {
		for _, p := range fr.Patterns {
			spec := "+refs/heads/" + p + ":refs/remotes/" + name + "/" + p
			if !slices.Contains(specs, spec) {
				specs = append(specs, spec)
			}
		}
	}
	specs = append(specs, fr.Other...)
	if err := unset("fetch"); err != nil {
		return err
	}
	for _, spec := range specs {
		if _, err := r.Write(ctx, Cmd{Args: []string{"config", "--local", "--add", key("fetch"), spec}}); err != nil {
			return err
		}
	}
	return nil
}
