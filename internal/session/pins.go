package session

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/risadams/gitrevenge/internal/branchkind"
	"github.com/risadams/gitrevenge/internal/git"
)

// maxPins is how many Trunk lanes the graph holds at the left (ADR 0009).
const maxPins = 2

// releaseDocsTips stops a Docs branch muting a lane it shares with real work: a Docs tip that
// an ordinary or Feature branch also points at takes that branch's kind instead. Ordinary
// branches aren't among the refs the patterns name, so this reads them all, but only when
// there's a Docs tip to check.
func releaseDocsTips(ctx context.Context, g *git.Repo, rules branchkind.Rules, remotes []string, visible func(string) bool, tips map[string]branchkind.Kind) {
	if !slices.Contains(slices.Collect(maps.Values(tips)), branchkind.Docs) {
		return
	}
	args := []string{"for-each-ref", "--format=%(refname) %(objectname)", "refs/heads"}
	for _, r := range remotes {
		args = append(args, "refs/remotes/"+r)
	}
	out, err := g.ReadString(ctx, args...)
	if err != nil {
		return
	}
	for line := range strings.Lines(out) {
		ref, commit, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || tips[commit] != branchkind.Docs && tips[commit] != branchkind.Feature {
			continue
		}
		name, isLocal := strings.CutPrefix(ref, "refs/heads/")
		if !isLocal {
			name = branchkind.Strip(strings.TrimPrefix(ref, "refs/remotes/"), remotes)
			if name == "HEAD" {
				continue
			}
		}
		if !visible(name) {
			continue
		}
		switch rules.Kind(name) {
		case branchkind.None:
			if tips[commit] == branchkind.Docs {
				delete(tips, commit)
			}
		case branchkind.Feature:
			if tips[commit] == branchkind.Docs {
				tips[commit] = branchkind.Feature
			}
		}
	}
}

// TrunkTips returns the commits of the Trunk branches to pin in the graph, in pattern
// order (see KindTips).
func TrunkTips(ctx context.Context, g *git.Repo, rules branchkind.Rules, remotes []string, show bool) []string {
	pins, _ := KindTips(ctx, g, rules, remotes, show)
	return pins
}

// KindTips reads the branches the kind patterns name, and returns the Trunk tips to pin
// (for each Trunk pattern in order, the local branch if there is one, else a remote's; at
// most maxPins) and the kind of every kinded branch tip, for the graph's lanes. Hidden
// branches are left out unless they're being shown. Where branches share a tip, the kind
// earliest in branchkind.Order wins. It reads only the refs the patterns name,
// so it stays fast with thousands of branches, unless a Docs tip needs checking.
func KindTips(ctx context.Context, g *git.Repo, rules branchkind.Rules, remotes []string, show bool) (pins []string, kinds map[string]string) {
	var patterns []string
	for _, k := range branchkind.Order {
		patterns = append(patterns, rules.Patterns(k)...)
	}
	if len(patterns) == 0 {
		return nil, nil
	}
	args := []string{"for-each-ref", "--format=%(refname) %(objectname)"}
	for _, p := range patterns {
		args = append(args, "refs/heads/"+p)
		for _, r := range remotes {
			args = append(args, "refs/remotes/"+r+"/"+p)
		}
	}
	out, err := g.ReadString(ctx, args...)
	if err != nil {
		return nil, nil
	}
	type ref struct{ name, commit string }
	var local, remote []ref
	for line := range strings.Lines(out) {
		name, commit, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(name, "refs/heads/"):
			local = append(local, ref{strings.TrimPrefix(name, "refs/heads/"), commit})
		case strings.HasPrefix(name, "refs/remotes/"):
			remote = append(remote, ref{branchkind.Strip(strings.TrimPrefix(name, "refs/remotes/"), remotes), commit})
		}
	}
	visible := func(name string) bool { return show || !rules.Hidden(name) }

	kinds = map[string]string{}
	tips := map[string]branchkind.Kind{}
	for _, list := range [][]ref{remote, local} {
		for _, r := range list {
			k := rules.Kind(r.name)
			if k == branchkind.None || !visible(r.name) {
				continue
			}
			if cur, ok := tips[r.commit]; !ok || slices.Index(branchkind.Order, k) < slices.Index(branchkind.Order, cur) {
				tips[r.commit] = k
			}
		}
	}
	releaseDocsTips(ctx, g, rules, remotes, visible, tips)
	for commit, k := range tips {
		kinds[commit] = string(k)
	}

	seen := map[string]bool{}
	for _, p := range rules.Patterns(branchkind.Trunk) {
		pick := ""
		for _, list := range [][]ref{local, remote} {
			for _, r := range list {
				if branchkind.Match(p, r.name) && visible(r.name) {
					pick = r.commit
					break
				}
			}
			if pick != "" {
				break
			}
		}
		if pick != "" && !seen[pick] {
			seen[pick] = true
			pins = append(pins, pick)
			if len(pins) == maxPins {
				break
			}
		}
	}
	return pins, kinds
}
