package git

import (
	"context"
	"slices"
	"strings"
)

// How a branch's work reached its target.
const (
	// MergedIn means the branch's tip is in the target's history.
	MergedIn = "merged"
	// SquashedIn means the same changes are on the target as different commits, from a
	// squash or a rebase, so git itself wouldn't call the branch merged.
	SquashedIn = "squashed"
)

// BranchMerge says whether a branch's work is already on another branch.
type BranchMerge struct {
	// Target is the branch it was compared with: the one holding its work, if any does,
	// else the first one tried. Empty if there was nothing to compare with.
	Target string `json:"target"`
	// How is MergedIn or SquashedIn, or "" when the work isn't known to be on Target.
	How string `json:"how"`
	// Unmerged counts the branch's commits whose changes aren't on Target. It's 0 when How
	// is set, or when the comparison couldn't be made.
	Unmerged int `json:"unmerged"`
}

// BranchMergeState compares a local branch with each target in turn and returns the first
// that holds its work. A branch inside another branch whose work is on a target counts too,
// which is how the lower branches of a squash-merged stack read as merged: they carry
// commits of their own that were squashed separately, and later changes on the target can
// make merging them conflict. Anything that goes wrong reads as "not known to be merged",
// since the answer decides whether deleting the branch asks first.
func (r *Repo) BranchMergeState(ctx context.Context, branch string, targets []string) BranchMerge {
	first := r.mergeIntoAny(ctx, "refs/heads/"+branch, targets)
	if first.How != "" || len(targets) == 0 {
		return first
	}
	for _, above := range r.branchesContaining(ctx, branch) {
		if slices.Contains(targets, above) {
			continue
		}
		if state := r.mergeIntoAny(ctx, "refs/heads/"+above, targets); state.How != "" {
			return BranchMerge{Target: state.Target, How: SquashedIn}
		}
	}
	return first
}

// maxBranchesAbove is how many branches holding a branch are looked at. A branch holding this
// many others is a long-lived one, which isn't where a merged stack's top would be.
const maxBranchesAbove = 30

// branchesContaining lists the other local branches that hold branch, most recent first.
func (r *Repo) branchesContaining(ctx context.Context, branch string) []string {
	out, err := r.ReadString(ctx, "for-each-ref", "--sort=-committerdate", "--contains=refs/heads/"+branch, "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil
	}
	var names []string
	for line := range strings.Lines(out) {
		if name := strings.TrimSpace(line); name != "" && name != branch && len(names) < maxBranchesAbove {
			names = append(names, name)
		}
	}
	return names
}

// mergeIntoAny returns the first target holding ref's work, else the state against the first.
func (r *Repo) mergeIntoAny(ctx context.Context, ref string, targets []string) BranchMerge {
	var first BranchMerge
	for i, target := range targets {
		state := r.mergeInto(ctx, ref, target)
		if state.How != "" {
			return state
		}
		if i == 0 {
			first = state
		}
	}
	return first
}

func (r *Repo) mergeInto(ctx context.Context, ref, target string) BranchMerge {
	state := BranchMerge{Target: target}
	if _, err := r.ReadString(ctx, "merge-base", "--is-ancestor", ref, target); err == nil {
		state.How = MergedIn
		return state
	}
	// Each of the branch's commits, marked - if the same change is on the target.
	out, err := r.ReadString(ctx, "cherry", target, ref)
	if err != nil {
		return state
	}
	missing, landed := countCherry(out)
	if missing == 0 && landed > 0 {
		state.How = SquashedIn
		return state
	}
	state.Unmerged = missing
	// One commit has no combined change that it doesn't already have itself.
	if (missing+landed > 1 && r.squashedWhole(ctx, ref, target)) || r.mergeChangesNothing(ctx, ref, target) {
		state.How = SquashedIn
		state.Unmerged = 0
	}
	return state
}

// mergeChangesNothing reports whether merging ref into target would leave target exactly as
// it is, so ref has nothing to add: the top of a stack squash-merged a branch at a time. A
// merge that conflicts doesn't count, and neither does work that target has since removed,
// since merging it would bring that back. Nor does a branch that changes nothing overall:
// merging it changes nothing either, but none of its work ever landed.
func (r *Repo) mergeChangesNothing(ctx context.Context, ref, target string) bool {
	base, err := r.ReadString(ctx, "merge-base", target, ref)
	if err != nil {
		return false
	}
	if _, err := r.ReadString(ctx, "diff", "--quiet", strings.TrimSpace(base), ref); err == nil {
		return false
	}
	merged, err := r.ReadString(ctx, "merge-tree", "--write-tree", target, ref)
	if err != nil {
		return false
	}
	tree, err := r.ReadString(ctx, "rev-parse", target+"^{tree}")
	if err != nil {
		return false
	}
	first, _, _ := strings.Cut(merged, "\n")
	return strings.TrimSpace(first) == strings.TrimSpace(tree)
}

// squashedWhole reports whether the branch's changes as a whole are one commit on the
// target, as a squash of several commits leaves them. It compares a commit standing in for
// that squash, which nothing refers to, so git drops it in its own time.
func (r *Repo) squashedWhole(ctx context.Context, ref, target string) bool {
	base, err := r.ReadString(ctx, "merge-base", target, ref)
	if err != nil {
		return false
	}
	base = strings.TrimSpace(base)
	whole, err := r.ReadString(ctx, "-c", "user.name=Git Revenge", "-c", "user.email=noreply@gitrevenge.invalid", "commit-tree", ref+"^{tree}", "-p", base, "-m", "squash check")
	if err != nil {
		return false
	}
	out, err := r.ReadString(ctx, "cherry", target, strings.TrimSpace(whole))
	if err != nil {
		return false
	}
	missing, landed := countCherry(out)
	return missing == 0 && landed == 1
}

// countCherry counts the lines of git cherry's output: + for a commit whose change isn't
// on the other branch, - for one whose change is.
func countCherry(out string) (missing, landed int) {
	for line := range strings.Lines(out) {
		switch {
		case strings.HasPrefix(line, "+"):
			missing++
		case strings.HasPrefix(line, "-"):
			landed++
		}
	}
	return missing, landed
}

// MergedBranches returns those of names whose work is on a target. It starts at the tops of
// stacks, because a branch inside one that's merged needs no merging of its own to check, and
// each check can take a second on a big repository. It returns what it has if ctx runs out.
func (r *Repo) MergedBranches(ctx context.Context, names, targets []string) map[string]BranchMerge {
	merged := map[string]BranchMerge{}
	if len(targets) == 0 {
		return merged
	}
	above := make(map[string][]string, len(names))
	for _, name := range names {
		for _, b := range r.branchesContaining(ctx, name) {
			if slices.Contains(names, b) {
				above[name] = append(above[name], b)
			}
		}
	}
	// Fewest branches above first, so whatever a branch could inherit has already been read.
	order := slices.Clone(names)
	slices.SortStableFunc(order, func(a, b string) int { return len(above[a]) - len(above[b]) })
	for _, name := range order {
		if ctx.Err() != nil {
			break
		}
		inherited := false
		for _, b := range above[name] {
			if m, ok := merged[b]; ok {
				merged[name] = BranchMerge{Target: m.Target, How: SquashedIn}
				inherited = true
				break
			}
		}
		if inherited {
			continue
		}
		if state := r.mergeIntoAny(ctx, "refs/heads/"+name, targets); state.How != "" {
			merged[name] = state
		}
	}
	return merged
}
