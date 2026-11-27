package app

import (
	"context"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Two versions of a branch compared with git range-diff (#1038).

// VersionCount is a version's size: its commits as range-diff reads them, merges left out,
// and the merges it leaves out.
type VersionCount struct {
	Commits int `json:"commits"`
	Merges  int `json:"merges"`
}

// VersionBase is where a version of branch ending at tip starts: where tip left the branch's
// base (its stack parent when it has one, else its remote's default branch), as a full id.
// "" when the branch has no base to go by; the dialog then asks for one.
func (a *App) VersionBase(path, branch, stackParent, tip string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	if err := git.CheckRevs(tip); err != nil {
		return "", userError(err.Error())
	}
	base, err := a.BranchPatchBase(path, branch, stackParent)
	if err != nil || base == "" {
		return "", err
	}
	at, err := r.Git.ReadString(context.Background(), "merge-base", tip, base)
	if err != nil {
		// No commit in common: the dialog asks where it starts.
		return "", nil
	}
	return at, nil
}

// CountVersion counts the commits from base to tip, for the dialog as it's filled in.
func (a *App) CountVersion(path, base, tip string) (VersionCount, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return VersionCount{}, err
	}
	side, err := resolveSide(context.Background(), r.Git, git.RangeSide{Base: base, Tip: tip})
	if err != nil {
		return VersionCount{}, err
	}
	commits, merges, err := r.Git.RangeCount(context.Background(), side)
	if err != nil {
		return VersionCount{}, patchError(err)
	}
	return VersionCount{Commits: commits, Merges: merges}, nil
}

// VersionsCompared is two versions compared: each old commit and the new one it became.
type VersionsCompared struct {
	// Old and New are the versions as compared, by full ids.
	Old git.RangeSide `json:"old"`
	New git.RangeSide `json:"new"`
	// OldCount and NewCount are their sizes.
	OldCount VersionCount    `json:"oldCount"`
	NewCount VersionCount    `json:"newCount"`
	Pairs    []git.RangePair `json:"pairs"`
	// Text is range-diff's whole output, for Copy as text.
	Text string `json:"text"`
	// Command is the git command that made it.
	Command string `json:"command"`
	// Loose says the commits were matched loosely.
	Loose bool `json:"loose"`
	// TooBig says a side has more than maxVersionCommits and nothing was compared: asked
	// again with anyway, it is (#1041).
	TooBig bool `json:"tooBig"`
	// Missing, in a partial clone, lists the file versions range-diff needs that were never
	// downloaded; nothing was compared until they are.
	Missing []string `json:"missing"`
	// OldEdge and NewEdge say a side holds a shallow clone's edge commit, whose change reads as
	// every file added.
	OldEdge bool `json:"oldEdge"`
	NewEdge bool `json:"newEdge"`
}

// maxVersionCommits is the most commits a side has before Compare asks first (#1041): 200
// against 200 takes about half a second on the kernel.
const maxVersionCommits = 500

// CompareVersions pairs old's commits with new's with git range-diff. Each side is a range,
// Base..Tip, of any branches, tags or commits. Loose matches commits reworked more. A side over
// maxVersionCommits comes back TooBig unless anyway; in a partial clone, what range-diff would
// read and isn't here comes back as Missing, to download first.
func (a *App) CompareVersions(path string, old, newer git.RangeSide, loose, anyway bool) (VersionsCompared, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return VersionsCompared{}, err
	}
	ctx := context.Background()
	if old, err = resolveSide(ctx, r.Git, old); err != nil {
		return VersionsCompared{}, err
	}
	if newer, err = resolveSide(ctx, r.Git, newer); err != nil {
		return VersionsCompared{}, err
	}
	out := VersionsCompared{Old: old, New: newer, Loose: loose, Pairs: []git.RangePair{}, Missing: []string{}}
	short := func(id string) string { return id[:min(len(id), 7)] }
	out.Command = "git range-diff " + short(old.Base) + ".." + short(old.Tip) + " " + short(newer.Base) + ".." + short(newer.Tip)
	if loose {
		out.Command = strings.Replace(out.Command, "range-diff ", "range-diff --creation-factor=100 ", 1)
	}
	if out.OldCount.Commits, out.OldCount.Merges, err = r.Git.RangeCount(ctx, old); err != nil {
		return VersionsCompared{}, patchError(err)
	}
	if out.NewCount.Commits, out.NewCount.Merges, err = r.Git.RangeCount(ctx, newer); err != nil {
		return VersionsCompared{}, patchError(err)
	}
	limit := maxVersionCommits
	if a.versionCap > 0 {
		limit = a.versionCap
	}
	if !anyway && max(out.OldCount.Commits, out.NewCount.Commits) > limit {
		out.TooBig = true
		return out, nil
	}
	missing, err := r.Git.RangeNeeds(ctx, old, newer)
	if err != nil {
		return VersionsCompared{}, patchError(err)
	}
	if len(missing) > 0 {
		out.Missing = missing
		return out, nil
	}
	out.OldEdge, _ = r.Git.RangeAtEdge(ctx, old)
	out.NewEdge, _ = r.Git.RangeAtEdge(ctx, newer)
	pairs, text, err := r.Git.RangeDiff(ctx, old, newer, loose)
	if err != nil {
		return VersionsCompared{}, patchError(err)
	}
	if pairs != nil {
		out.Pairs = pairs
	}
	out.Text = text
	return out, nil
}

// resolveSide turns a side's branches, tags or commits into full commit ids, saying which one
// isn't here.
func resolveSide(ctx context.Context, repo *git.Repo, side git.RangeSide) (git.RangeSide, error) {
	for _, rev := range []*string{&side.Base, &side.Tip} {
		name := strings.TrimSpace(*rev)
		if name == "" {
			return side, userError("Say where each version starts and ends.")
		}
		if err := git.CheckRevs(name); err != nil {
			return side, userError(err.Error())
		}
		id, err := repo.ReadString(ctx, "rev-parse", "-q", "--verify", name+"^{commit}")
		if err != nil || id == "" {
			// A whole id the app had (an earlier tip, where a rebase began) that git has since
			// pruned (#1041).
			if git.IsCommitID(name) && len(name) == 40 {
				return side, userError(name[:7] + " isn't in this repository any more, so there's nothing to compare it with.")
			}
			return side, userError("“" + name + "” isn't a branch, tag or commit here.")
		}
		*rev = id
	}
	return side, nil
}
