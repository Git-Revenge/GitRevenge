package session

import (
	"context"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/git"
)

// Stacks returns which local branch each local branch is stacked on: inferred from history
// (git.StackParents, against the first Trunk branch), then overridden by the repository's
// settings, where "" means not stacked. Trunk branches are never parents.
func Stacks(ctx context.Context, g *git.Repo, refs []git.Ref, rules branchkind.Rules, overrides map[string]string) map[string]string {
	local := map[string]bool{}
	trunk := ""
	for _, p := range rules.Patterns(branchkind.Trunk) {
		for _, r := range refs {
			if r.Kind == git.LocalBranch && trunk == "" && branchkind.Match(p, r.Short) {
				trunk = r.Short
			}
		}
	}
	for _, r := range refs {
		if r.Kind == git.LocalBranch {
			local[r.Short] = true
		}
	}
	parents, err := g.StackParents(ctx, trunk, func(n string) bool { return rules.Kind(n) == branchkind.Trunk })
	if err != nil || parents == nil {
		parents = map[string]string{}
	}
	for child, parent := range overrides {
		switch {
		case !local[child]:
		case parent == "":
			delete(parents, child)
		case local[parent] && parent != child:
			parents[child] = parent
		}
	}
	// An override could close a loop (a on b, b on a): drop the edge that closes it.
	for child := range parents {
		seen := map[string]bool{child: true}
		for p := parents[child]; p != ""; p = parents[p] {
			if seen[p] {
				delete(parents, child)
				break
			}
			seen[p] = true
		}
	}
	return parents
}

// annotateStacks sets each local branch's stack parent, depth and the stack's height.
func annotateStacks(refs []git.Ref, parents map[string]string) {
	children := map[string]bool{}
	for _, p := range parents {
		children[p] = true
	}
	depth := func(b string) int {
		d := 1
		for p := parents[b]; p != ""; p = parents[p] {
			d++
		}
		return d
	}
	root := func(b string) string {
		for parents[b] != "" {
			b = parents[b]
		}
		return b
	}
	height := map[string]int{}
	for b := range parents {
		if d := depth(b); d > height[root(b)] {
			height[root(b)] = d
		}
	}
	gone := map[string]bool{}
	for _, r := range refs {
		if r.Kind == git.LocalBranch && r.Gone {
			gone[r.Short] = true
		}
	}
	for i, r := range refs {
		if r.Kind != git.LocalBranch || (parents[r.Short] == "" && !children[r.Short]) {
			continue
		}
		refs[i].StackParentMerged = gone[parents[r.Short]]
		refs[i].StackParent = parents[r.Short]
		refs[i].StackDepth = depth(r.Short)
		refs[i].StackSize = height[root(r.Short)]
	}
}
