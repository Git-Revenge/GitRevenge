package app

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Limits on the Map view. Each branch costs one `rev-list --count`, each tag shown one or
// two, so a repository with thousands of refs still answers quickly.
const (
	// maxMapBranches caps the branches shown: the ones that moved most recently.
	maxMapBranches = 60
	// maxMapTags is how many tags get a node of their own; older ones fold into one.
	maxMapTags = 6
	// minMapGroup is how many remote-only branches must share a first path segment
	// (feature/, bugfix/) to be drawn as one node.
	minMapGroup = 3
	// maxMapGroup caps the branches an opened group shows: the ones that moved most recently.
	maxMapGroup = 40
)

// The kinds of node in the Map view.
const (
	mapTrunk  = "trunk"
	mapBranch = "branch"
	mapRemote = "remote"
	mapTag    = "tag"
	mapTags   = "tags"
	mapGroup  = "group"
)

// MapNode is one node of the Map view: the trunk, a branch, a tag, or the older tags folded
// into one.
type MapNode struct {
	// Name is the short name shown: main, origin/gh-pages, v1.2.0.
	Name string `json:"name"`
	// Kind is "trunk", "branch" (local), "remote" (only on a remote), "tag", or "tags" for the
	// fold of older tags.
	Kind string `json:"kind"`
	// Commit is the commit the node points at: for the fold, its newest tag's.
	Commit string `json:"commit"`
	// Commits is the node's size: every commit for the trunk, the commits the trunk doesn't
	// have for a branch, the commits since the tag before it for a tag, and everything up to
	// its newest tag for the fold.
	Commits int `json:"commits"`
	// Updated is when the node last moved, in Unix seconds.
	Updated int64 `json:"updated"`
	// Parent is the name of the branch node this one hangs off, or "" for the trunk: the
	// branch that holds a tag, or the branch a stacked branch is stacked on.
	Parent string `json:"parent"`
	// Stacked marks a branch that hangs off the branch it's stacked on.
	Stacked bool `json:"stacked,omitempty"`
	// Worktree is the folder of the other worktree the branch is checked out in, or "".
	Worktree string `json:"worktree,omitempty"`
	// Head marks the checked-out branch.
	Head bool `json:"head"`
	// BranchKind is the branch's kind ("release", "docs", …), or "".
	BranchKind string `json:"branchKind,omitempty"`
	// Tags is how many tags the fold holds.
	Tags int `json:"tags,omitempty"`
	// Branches is how many branches a group stands for. A group isn't counted commit by
	// commit until it's opened: Commits is 0.
	Branches int `json:"branches,omitempty"`
}

// BranchMap is what the Map view draws.
type BranchMap struct {
	// Trunk is the node in the middle. Its Name is "" when the repository has no branch to
	// call the trunk (no commits yet, or a detached HEAD with no branches).
	Trunk MapNode   `json:"trunk"`
	Nodes []MapNode `json:"nodes"`
	// Branches and Tags count what the repository has, shown or not.
	Branches int `json:"branches"`
	Tags     int `json:"tags"`
	// MoreBranches is how many branches were left out for being the least recently moved.
	MoreBranches int `json:"moreBranches"`
}

// BranchMap returns a node for each branch and tag of the repository at path, sized by
// commits, for the Map view. Hidden branches are left out, as they are in the graph.
func (a *App) BranchMap(path string) (BranchMap, error) {
	out := BranchMap{Nodes: []MapNode{}}
	r, err := a.openRepo(path)
	if err != nil {
		return out, err
	}
	ctx := context.Background()
	refs := r.Refs().All

	trunk, ok := mapTrunkOf(refs)
	if !ok {
		return out, nil
	}
	count := func(rng string) (int, error) {
		s, err := r.Git.ReadString(ctx, "rev-list", "--count", rng, "--")
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(strings.TrimSpace(s))
	}
	// Every count is its own git call, and none depends on another: they run side by side.
	var jobs []func() error
	out.Trunk = MapNode{Name: trunk.Short, Kind: mapTrunk, Commit: trunk.Commit, Updated: trunk.Updated, Head: trunk.Head, BranchKind: trunk.BranchKind, Worktree: trunk.Worktree}
	jobs = append(jobs, func() (err error) {
		out.Trunk.Commits, err = count(trunk.Name)
		return err
	})

	branches := mapBranches(refs, trunk)
	out.Branches = len(branches) + 1
	// Remote-only branches that share a first path segment are one node between them. They're
	// somebody else's work, there can be hundreds, and counting each one's commits is what
	// takes the time: a group is only counted when it's opened (BranchMapGroup).
	branches, groupNodes := mapGroups(branches)
	// The ones that moved last are the ones to show, when there are too many.
	slices.SortStableFunc(branches, func(x, y git.Ref) int { return cmp.Compare(y.Updated, x.Updated) })
	if len(branches) > maxMapBranches {
		out.MoreBranches = len(branches) - maxMapBranches
		branches = branches[:maxMapBranches]
	}
	shown := map[string]bool{}
	branchNodes := make([]MapNode, len(branches))
	for i, b := range branches {
		kind := mapBranch
		if b.Kind == git.RemoteBranch {
			kind = mapRemote
		}
		shown[b.Name] = true
		branchNodes[i] = MapNode{Name: b.Short, Kind: kind, Commit: b.Commit, Updated: b.Updated, Head: b.Head, BranchKind: b.BranchKind, Worktree: b.Worktree}
		jobs = append(jobs, func() (err error) {
			branchNodes[i].Commits, err = count(trunk.Name + ".." + b.Name)
			return err
		})
	}

	// A stacked branch hangs off the branch under it, when that one has a node too.
	local := map[string]bool{}
	for _, b := range branches {
		if b.Kind == git.LocalBranch {
			local[b.Short] = true
		}
	}
	for i, b := range branches {
		if b.Kind == git.LocalBranch && b.StackParent != "" && local[b.StackParent] {
			branchNodes[i].Parent, branchNodes[i].Stacked = b.StackParent, true
		}
	}

	tags := mapTagsOf(refs)
	out.Tags = len(tags)
	tagNodes := make([]MapNode, min(len(tags), maxMapTags+1))
	for i, t := range tags {
		if i == maxMapTags {
			// Everything older folds into one node, as big as the history up to its newest.
			tagNodes[i] = MapNode{Name: strconv.Itoa(len(tags)-i) + " older tags", Kind: mapTags, Commit: t.Commit, Updated: t.Updated, Tags: len(tags) - i}
			jobs = append(jobs, func() (err error) {
				tagNodes[i].Commits, err = count(t.Name)
				return err
			})
			break
		}
		rng := t.Name
		if i+1 < len(tags) {
			rng = tags[i+1].Name + ".." + t.Name
		}
		tagNodes[i] = MapNode{Name: t.Short, Kind: mapTag, Commit: t.Commit, Updated: t.Updated}
		jobs = append(jobs, func() (err error) {
			tagNodes[i].Commits, err = count(rng)
			return err
		}, func() (err error) {
			tagNodes[i].Parent, err = mapTagParent(ctx, r.Git, trunk, t, refs, shown)
			return err
		})
	}
	if err := runMapJobs(jobs); err != nil {
		return BranchMap{Nodes: []MapNode{}}, err
	}
	out.Nodes = append(append(append(out.Nodes, branchNodes...), groupNodes...), tagNodes...)
	return out, nil
}

// mapGroupOf names the group a remote-only branch belongs to: its remote and the first
// segment of its path, "origin/feature/" for origin/feature/login. A branch with no path
// (origin/topic) belongs to none.
func mapGroupOf(ref git.Ref) string {
	if ref.Kind != git.RemoteBranch {
		return ""
	}
	remote, rest, ok := strings.Cut(ref.Short, "/")
	if !ok {
		return ""
	}
	segment, _, ok := strings.Cut(rest, "/")
	if !ok {
		return ""
	}
	return remote + "/" + segment + "/"
}

// mapGroups takes the remote-only branches that share a group with enough others out of
// branches, and returns the rest with a node for each group: its newest branch's commit and
// date, and how many branches it stands for.
func mapGroups(branches []git.Ref) (rest []git.Ref, groups []MapNode) {
	sizes := map[string]int{}
	for _, b := range branches {
		if g := mapGroupOf(b); g != "" {
			sizes[g]++
		}
	}
	at := map[string]int{}
	for _, b := range branches {
		g := mapGroupOf(b)
		if g == "" || sizes[g] < minMapGroup {
			rest = append(rest, b)
			continue
		}
		i, seen := at[g]
		if !seen {
			i = len(groups)
			at[g] = i
			groups = append(groups, MapNode{Name: g, Kind: mapGroup, Branches: sizes[g]})
		}
		if b.Updated >= groups[i].Updated {
			groups[i].Updated, groups[i].Commit = b.Updated, b.Commit
		}
	}
	return rest, groups
}

// MapGroup is a group of the Map view, opened.
type MapGroup struct {
	// Nodes are the group's branches, newest first, each hanging off the group.
	Nodes []MapNode `json:"nodes"`
	// More is how many of its branches were left out for being the least recently moved.
	More int `json:"more"`
}

// BranchMapGroup returns the branches of one group of the Map view (see BranchMap), sized by
// the commits the trunk doesn't have. group is the group node's name.
func (a *App) BranchMapGroup(path, group string) (MapGroup, error) {
	out := MapGroup{Nodes: []MapNode{}}
	r, err := a.openRepo(path)
	if err != nil {
		return out, err
	}
	ctx := context.Background()
	refs := r.Refs().All
	trunk, ok := mapTrunkOf(refs)
	if !ok {
		return out, nil
	}
	var members []git.Ref
	for _, b := range mapBranches(refs, trunk) {
		if mapGroupOf(b) == group {
			members = append(members, b)
		}
	}
	slices.SortStableFunc(members, func(x, y git.Ref) int { return cmp.Compare(y.Updated, x.Updated) })
	if len(members) > maxMapGroup {
		out.More = len(members) - maxMapGroup
		members = members[:maxMapGroup]
	}
	nodes := make([]MapNode, len(members))
	var jobs []func() error
	for i, b := range members {
		nodes[i] = MapNode{Name: b.Short, Kind: mapRemote, Commit: b.Commit, Updated: b.Updated, BranchKind: b.BranchKind, Parent: group}
		jobs = append(jobs, func() error {
			s, err := r.Git.ReadString(ctx, "rev-list", "--count", trunk.Name+".."+b.Name, "--")
			if err != nil {
				return err
			}
			nodes[i].Commits, err = strconv.Atoi(strings.TrimSpace(s))
			return err
		})
	}
	if err := runMapJobs(jobs); err != nil {
		return out, err
	}
	out.Nodes = nodes
	return out, nil
}

// mapWorkers is how many git calls the Map view runs at once.
const mapWorkers = 8

// runMapJobs runs the jobs a few at a time and returns the first error, if any. Each job
// writes only its own result.
func runMapJobs(jobs []func() error) error {
	var (
		wg    sync.WaitGroup
		once  sync.Once
		first error
		slots = make(chan struct{}, mapWorkers)
	)
	for _, job := range jobs {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			if err := job(); err != nil {
				once.Do(func() { first = err })
			}
		}()
	}
	wg.Wait()
	return first
}

// mapTrunkOf picks the branch in the middle: the local Trunk branch, else the checked-out
// branch, else the first local branch.
func mapTrunkOf(refs []git.Ref) (git.Ref, bool) {
	var head, first *git.Ref
	for i := range refs {
		ref := &refs[i]
		if ref.Kind != git.LocalBranch {
			continue
		}
		if ref.BranchKind == "trunk" {
			return *ref, true
		}
		if ref.Head && head == nil {
			head = ref
		}
		if first == nil {
			first = ref
		}
	}
	if head != nil {
		return *head, true
	}
	if first != nil {
		return *first, true
	}
	return git.Ref{}, false
}

// mapBranches returns the branches that get a node: every local branch but the trunk, and
// each remote branch no local branch stands for. Hidden branches are left out.
func mapBranches(refs []git.Ref, trunk git.Ref) []git.Ref {
	locals := map[string]bool{}
	tracked := map[string]bool{}
	for _, ref := range refs {
		if ref.Kind == git.LocalBranch {
			locals[ref.Short] = true
			if ref.Upstream != "" {
				tracked[ref.Upstream] = true
			}
		}
	}
	var out []git.Ref
	for _, ref := range refs {
		switch ref.Kind {
		case git.LocalBranch:
			if ref.Name != trunk.Name && !ref.Hidden {
				out = append(out, ref)
			}
		case git.RemoteBranch:
			_, name, _ := strings.Cut(ref.Short, "/")
			// origin/HEAD is a pointer, not a branch; a tracked branch, or one with a local
			// namesake, is already there as the local one.
			if name == "HEAD" || tracked[ref.Short] || locals[name] || ref.Hidden {
				continue
			}
			out = append(out, ref)
		}
	}
	return out
}

// mapTagsOf returns the tags, newest first. A pre-release tag (v1.2.0-rc.1) is left out when
// its release (v1.2.0) is tagged too: it rides inside its release.
func mapTagsOf(refs []git.Ref) []git.Ref {
	names := map[string]bool{}
	for _, ref := range refs {
		if ref.Kind == git.Tag {
			names[ref.Short] = true
		}
	}
	var out []git.Ref
	for _, ref := range refs {
		if ref.Kind != git.Tag {
			continue
		}
		if base, _, pre := strings.Cut(ref.Short, "-"); pre && names[base] {
			continue
		}
		out = append(out, ref)
	}
	slices.SortStableFunc(out, func(x, y git.Ref) int {
		if c := cmp.Compare(y.Updated, x.Updated); c != 0 {
			return c
		}
		return cmp.Compare(y.Short, x.Short)
	})
	return out
}

// mapTagParent says which node a tag hangs off: "" (the trunk) when the trunk holds its
// commit, else the first shown branch that does, local before remote.
func mapTagParent(ctx context.Context, g *git.Repo, trunk, tag git.Ref, refs []git.Ref, shown map[string]bool) (string, error) {
	s, err := g.ReadString(ctx, "rev-list", "--count", trunk.Name+".."+tag.Commit, "--")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(s) == "0" {
		return "", nil
	}
	held, err := g.ReadString(ctx, "for-each-ref", "--contains="+tag.Commit, "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return "", err
	}
	holders := map[string]bool{}
	for line := range strings.Lines(held) {
		holders[strings.TrimSpace(line)] = true
	}
	for _, kind := range []git.RefKind{git.LocalBranch, git.RemoteBranch} {
		for _, ref := range refs {
			if ref.Kind == kind && shown[ref.Name] && holders[ref.Name] {
				return ref.Short, nil
			}
		}
	}
	return "", nil
}
