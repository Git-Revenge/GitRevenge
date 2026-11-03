package git

import (
	"context"
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
// that holds its work. Anything that goes wrong reads as "not known to be merged", since
// the answer decides whether deleting the branch asks first.
func (r *Repo) BranchMergeState(ctx context.Context, branch string, targets []string) BranchMerge {
	var first BranchMerge
	for i, target := range targets {
		state := r.mergeInto(ctx, "refs/heads/"+branch, target)
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
	if missing+landed > 1 && r.squashedWhole(ctx, ref, target) {
		state.How = SquashedIn
		state.Unmerged = 0
	}
	return state
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
