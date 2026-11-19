package app

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/git"
)

// The Map's Authors mode (#816): a node for each person who wrote commits inside a window of
// time, sized by how many. A commit counts for its author and for everyone a Co-authored-by
// or Co-developed-by trailer credits; merges count for nobody.

// Limits on the Authors mode.
const (
	// maxMapPeople and maxMapBots cap the authors returned: the ones with the most commits in
	// the window. You are returned whether or not you're among them.
	maxMapPeople = 60
	maxMapBots   = 20
	// maxAuthorBranches caps the unmerged branches read for who wrote on them: the ones that
	// moved most recently. Each is its own git log.
	maxAuthorBranches = 200
	// maxAuthorChildren caps the branches one author opens into.
	maxAuthorChildren = 12
)

// AuthorIdentity is one name and email a person committed under.
type AuthorIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	// Commits is how many commits in the window carry this identity.
	Commits int `json:"commits"`
}

// MapAuthor is one person in the Map's Authors mode.
type MapAuthor struct {
	// Key names the author among the others: the email they used most, in lower case, or
	// "name:" and their name when they have none.
	Key string `json:"key"`
	// Name and Email are the identity they used most in the window.
	Name  string `json:"name"`
	Email string `json:"email"`
	// Identities are every name and email folded into this person, most used first: after the
	// committed .mailmap, anyone sharing an email or a name is one person.
	Identities []AuthorIdentity `json:"identities"`
	// Commits is the node's size: the commits in the window they wrote or are credited on,
	// each counted once. Authored and CoAuthored split it.
	Commits    int `json:"commits"`
	Authored   int `json:"authored"`
	CoAuthored int `json:"coAuthored"`
	// Updated is when their latest commit in the window was made, in Unix seconds.
	Updated int64 `json:"updated"`
	// Bot marks an author that matches a bot author pattern.
	Bot bool `json:"bot"`
	// You marks the author who is the repository's user.name and user.email.
	You bool `json:"you"`
	// Trunk is how many of their commits the trunk has: what has landed.
	Trunk int `json:"trunk"`
	// Branches are the unmerged branches that hold commits of theirs, most recently moved
	// first, each sized by their commits the trunk doesn't have. A commit on a branch and on
	// the branch stacked above it counts in both.
	Branches []MapNode `json:"branches"`
	// MoreBranches is how many such branches were left out of Branches.
	MoreBranches int `json:"moreBranches"`
}

// AuthorMap is what the Map's Authors mode draws.
type AuthorMap struct {
	// Trunk is the node in the middle, as BranchMap gives it. Its Name is "" when the
	// repository has no branch to call the trunk.
	Trunk   MapNode     `json:"trunk"`
	Authors []MapAuthor `json:"authors"`
	// People and Bots count every author in the window, returned or not.
	People int `json:"people"`
	Bots   int `json:"bots"`
	// Commits is how many commits the window holds, merges left out.
	Commits int `json:"commits"`
	// Since is the start of the window in Unix seconds, or 0 for all of history.
	Since int64 `json:"since"`
	// UncountedBranches is how many unmerged branches that moved in the window weren't read,
	// for being the least recently moved of more than the limit.
	UncountedBranches int `json:"uncountedBranches"`
}

// MapAuthors returns who wrote the commits of the last days days in the repository at path,
// for the Map's Authors mode. days of 0 or less means all of history. Hidden branches are
// left out, as they are in the graph.
func (a *App) MapAuthors(path string, days int) (AuthorMap, error) {
	var since int64
	if days > 0 {
		since = time.Now().Unix() - int64(days)*24*60*60
	}
	return a.mapAuthorsSince(path, days, since)
}

// mapAuthorsSince is MapAuthors with the start of the window given. Results are kept for the
// rest of the hour, until a ref moves or the rules that shape them change.
func (a *App) mapAuthorsSince(path string, days int, since int64) (AuthorMap, error) {
	empty := AuthorMap{Authors: []MapAuthor{}, Since: since}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	ctx := context.Background()
	refs := r.Refs().All
	trunk, ok := mapTrunkOf(refs)
	if !ok {
		return empty, nil
	}
	rules := a.branchRules(r.Path)
	me := r.Git.CommitIdentity(ctx)

	state, err := refState(ctx, r.Git)
	if err != nil {
		return empty, err
	}
	shape, err := json.Marshal([]any{rules.BotAuthorPatterns(), rules.Hide(), me, trunk.Name})
	if err != nil {
		return empty, err
	}
	// A window measured from now slides: an hour is as stale as an answer gets.
	key := strings.Join([]string{r.Path, strconv.Itoa(days), strconv.FormatInt(since/3600, 10), state, string(shape)}, "\x00")
	if v, ok := a.mapAuthors.get(key); ok {
		if m, ok := v.(AuthorMap); ok {
			return m, nil
		}
	}

	branches, uncounted := authorBranches(refs, trunk, since)

	// One log for the trunk and one for each branch, side by side. Each job fills only its own.
	onTrunk := newAuthorTally()
	onBranch := make([][]git.AuthorCommit, len(branches))
	total := 0
	jobs := []func() error{
		func() error {
			return r.Git.AuthorLog(ctx, trunk.Name, since, []string{trunk.Name}, onTrunk.add)
		},
		func() error {
			s, err := r.Git.ReadString(ctx, "rev-list", "--count", trunk.Name, "--")
			if err != nil {
				return err
			}
			total, err = strconv.Atoi(strings.TrimSpace(s))
			return err
		},
	}
	for i, b := range branches {
		jobs = append(jobs, func() error {
			return r.Git.AuthorLog(ctx, trunk.Name, since, []string{trunk.Name + ".." + b.Name}, func(c git.AuthorCommit) {
				onBranch[i] = append(onBranch[i], c)
			})
		})
	}
	if err := runMapJobs(jobs); err != nil {
		return empty, err
	}

	out := foldAuthors(onTrunk, branches, onBranch, func(ids []git.Identity) []git.Identity {
		// Without the mailmap's answer, a co-author is who the trailer says.
		mapped, _ := r.Git.MailmapIdentities(ctx, trunk.Name, ids)
		return mapped
	}, rules, me)
	out.Trunk = MapNode{Name: trunk.Short, Kind: mapTrunk, Commit: trunk.Commit, Commits: total, Updated: trunk.Updated, Head: trunk.Head, BranchKind: trunk.BranchKind, Worktree: trunk.Worktree}
	out.Since, out.UncountedBranches = since, uncounted
	a.mapAuthors.put(key, out)
	return out, nil
}

// authorBranches returns the unmerged branches the Authors mode reads for a window that
// starts at since, most recently moved first: the ones that moved inside it, since nothing
// older can hold a commit made in it, and no more than the limit. uncounted is how many
// were left out for being over it.
func authorBranches(refs []git.Ref, trunk git.Ref, since int64) (branches []git.Ref, uncounted int) {
	branches = mapBranches(refs, trunk)
	if since > 0 {
		branches = slices.DeleteFunc(branches, func(b git.Ref) bool { return b.Updated < since })
	}
	slices.SortStableFunc(branches, func(x, y git.Ref) int { return cmp.Compare(y.Updated, x.Updated) })
	if len(branches) > maxAuthorBranches {
		uncounted = len(branches) - maxAuthorBranches
		branches = branches[:maxAuthorBranches]
	}
	return branches, uncounted
}

// MapAuthorHistory returns how much the person behind identities has written over the whole
// history of the repository at path, for an author's card in the Map. identities are the
// names and emails MapAuthors folded into them.
func (a *App) MapAuthorHistory(path string, identities []AuthorIdentity) (git.AuthorHistory, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.AuthorHistory{}, err
	}
	refs := r.Refs().All
	trunk, ok := mapTrunkOf(refs)
	if !ok {
		return git.AuthorHistory{}, nil
	}
	revs := []string{trunk.Name}
	for _, b := range mapBranches(refs, trunk) {
		revs = append(revs, b.Name)
	}
	ids := make([]git.Identity, len(identities))
	for i, id := range identities {
		ids[i] = git.Identity{Name: id.Name, Email: id.Email}
	}
	return r.Git.AuthorHistory(context.Background(), trunk.Name, revs, ids)
}

// authorTally counts commits by who took part in them, without keeping the commits: a long
// history has millions of commits and a few thousand different sets of people.
type authorTally struct {
	sets map[authorSet]*authorSetCount
}

// authorSet is the people on one commit: its author, and its co-author trailers as written.
type authorSet struct{ name, email, coAuthors string }

type authorSetCount struct {
	commits int
	last    int64
}

func newAuthorTally() *authorTally { return &authorTally{sets: map[authorSet]*authorSetCount{}} }

func (t *authorTally) add(c git.AuthorCommit) {
	k := authorSet{c.Name, c.Email, c.CoAuthors}
	n := t.sets[k]
	if n == nil {
		n = &authorSetCount{}
		t.sets[k] = n
	}
	n.commits++
	n.last = max(n.last, c.Committed)
}

// authorFold works out who is one person. Identities are numbered as they're met; two that
// share an email or a name are joined.
type authorFold struct {
	index  map[git.Identity]int
	list   []git.Identity
	parent []int
}

func (f *authorFold) id(who git.Identity) int {
	who.Name = strings.Join(strings.Fields(who.Name), " ")
	who.Email = strings.TrimSpace(who.Email)
	if i, ok := f.index[who]; ok {
		return i
	}
	i := len(f.list)
	f.index[who] = i
	f.list = append(f.list, who)
	f.parent = append(f.parent, i)
	return i
}

func (f *authorFold) find(i int) int {
	for f.parent[i] != i {
		f.parent[i] = f.parent[f.parent[i]]
		i = f.parent[i]
	}
	return i
}

func (f *authorFold) join(i, j int) {
	if i, j = f.find(i), f.find(j); i != j {
		// The lower number leads, so the result doesn't depend on the order of joining.
		f.parent[max(i, j)] = min(i, j)
	}
}

// foldAuthors turns the commits counted on the trunk and on each unmerged branch into people.
// mailmap resolves co-authors' identities, which git log leaves as the trailer wrote them.
func foldAuthors(onTrunk *authorTally, branches []git.Ref, onBranch [][]git.AuthorCommit, mailmap func([]git.Identity) []git.Identity, rules branchkind.Rules, me git.Identity) AuthorMap {
	out := AuthorMap{Authors: []MapAuthor{}}
	fold := &authorFold{index: map[git.Identity]int{}}

	// Co-authors first, so the mailmap is asked once for all of them.
	trailers := map[string]bool{}
	for set := range onTrunk.sets {
		trailers[set.coAuthors] = true
	}
	for _, commits := range onBranch {
		for _, c := range commits {
			trailers[c.CoAuthors] = true
		}
	}
	written := map[string]git.Identity{}
	for all := range trailers {
		for value := range strings.SplitSeq(all, "\x1d") {
			if value = strings.TrimSpace(value); value != "" {
				written[value] = git.ParseIdentity(value)
			}
		}
	}
	values := slices.Sorted(func(yield func(string) bool) {
		for v := range written {
			if !yield(v) {
				return
			}
		}
	})
	asked := make([]git.Identity, len(values))
	for i, v := range values {
		asked[i] = written[v]
	}
	resolved := map[string]int{}
	for i, who := range mailmap(asked) {
		resolved[values[i]] = fold.id(who)
	}
	// The people on a commit, the author first, each once.
	people := func(name, email, coAuthors string) []int {
		ids := []int{fold.id(git.Identity{Name: name, Email: email})}
		for value := range strings.SplitSeq(coAuthors, "\x1d") {
			if i, ok := resolved[strings.TrimSpace(value)]; ok && !slices.Contains(ids, i) {
				ids = append(ids, i)
			}
		}
		return ids
	}

	// Every commit, by where it is: -1 for the trunk, else the branch's number. A commit on
	// two branches is one commit that's on both.
	type counted struct {
		ids     []int
		commits int
		last    int64
		on      []int
	}
	var all []*counted
	for set, n := range onTrunk.sets {
		all = append(all, &counted{ids: people(set.name, set.email, set.coAuthors), commits: n.commits, last: n.last, on: []int{-1}})
	}
	bySHA := map[string]*counted{}
	for b, commits := range onBranch {
		for _, c := range commits {
			one := bySHA[c.SHA]
			if one == nil {
				one = &counted{ids: people(c.Name, c.Email, c.CoAuthors), commits: 1, last: c.Committed}
				bySHA[c.SHA] = one
				all = append(all, one)
			}
			one.on = append(one.on, b)
		}
	}

	// Who is one person: anyone sharing an email, or sharing a name.
	byEmail, byName := map[string]int{}, map[string]int{}
	for i, who := range fold.list {
		if e := strings.ToLower(who.Email); e != "" {
			if j, ok := byEmail[e]; ok {
				fold.join(i, j)
			} else {
				byEmail[e] = i
			}
		}
		if n := strings.ToLower(who.Name); n != "" {
			if j, ok := byName[n]; ok {
				fold.join(i, j)
			} else {
				byName[n] = i
			}
		}
	}

	type person struct {
		author   MapAuthor
		used     map[int]int
		onBranch map[int]int
	}
	persons := map[int]*person{}
	for _, c := range all {
		out.Commits += c.commits
		// The people on this commit, each once however many of their identities are on it.
		var roots []int
		for k, id := range c.ids {
			root := fold.find(id)
			p := persons[root]
			if p == nil {
				p = &person{used: map[int]int{}, onBranch: map[int]int{}}
				persons[root] = p
			}
			p.used[id] += c.commits
			if slices.Contains(roots, root) {
				continue
			}
			roots = append(roots, root)
			p.author.Commits += c.commits
			if k == 0 {
				p.author.Authored += c.commits
			} else {
				p.author.CoAuthored += c.commits
			}
			p.author.Updated = max(p.author.Updated, c.last)
			for _, b := range c.on {
				if b < 0 {
					p.author.Trunk += c.commits
				} else {
					p.onBranch[b] += c.commits
				}
			}
		}
	}

	myEmail, myName := strings.ToLower(strings.TrimSpace(me.Email)), strings.ToLower(strings.Join(strings.Fields(me.Name), " "))
	var humans, bots []MapAuthor
	for _, p := range persons {
		au := p.author
		for id, n := range p.used {
			who := fold.list[id]
			au.Identities = append(au.Identities, AuthorIdentity{Name: who.Name, Email: who.Email, Commits: n})
			au.Bot = au.Bot || rules.BotAuthor(who.Name, who.Email)
		}
		slices.SortFunc(au.Identities, func(x, y AuthorIdentity) int {
			return cmp.Or(cmp.Compare(y.Commits, x.Commits), cmp.Compare(x.Name, y.Name), cmp.Compare(x.Email, y.Email))
		})
		au.Name, au.Email = au.Identities[0].Name, au.Identities[0].Email
		au.Key = strings.ToLower(au.Email)
		if au.Key == "" {
			au.Key = "name:" + strings.ToLower(au.Name)
		}
		// The branches, most recently moved first: the order they came in.
		au.Branches = []MapNode{}
		for b, ref := range branches {
			n := p.onBranch[b]
			if n == 0 {
				continue
			}
			if len(au.Branches) == maxAuthorChildren {
				au.MoreBranches++
				continue
			}
			kind := mapBranch
			if ref.Kind == git.RemoteBranch {
				kind = mapRemote
			}
			au.Branches = append(au.Branches, MapNode{Name: ref.Short, Kind: kind, Commit: ref.Commit, Commits: n, Updated: ref.Updated, Parent: au.Key, Head: ref.Head, BranchKind: ref.BranchKind, Worktree: ref.Worktree})
		}
		if au.Bot {
			out.Bots++
			bots = append(bots, au)
		} else {
			out.People++
			humans = append(humans, au)
		}
	}
	// The most commits first; then whoever committed last, then by name, so the same
	// repository always gives the same order.
	order := func(x, y MapAuthor) int {
		return cmp.Or(cmp.Compare(y.Commits, x.Commits), cmp.Compare(y.Updated, x.Updated), cmp.Compare(x.Name, y.Name), cmp.Compare(x.Key, y.Key))
	}
	slices.SortFunc(humans, order)
	slices.SortFunc(bots, order)

	// You are whoever holds your email; failing that, your name.
	mine := func(match func(AuthorIdentity) bool) *MapAuthor {
		for _, list := range [][]MapAuthor{humans, bots} {
			for i := range list {
				if slices.ContainsFunc(list[i].Identities, match) {
					return &list[i]
				}
			}
		}
		return nil
	}
	var you *MapAuthor
	if myEmail != "" {
		you = mine(func(id AuthorIdentity) bool { return strings.ToLower(id.Email) == myEmail })
	}
	if you == nil && myName != "" {
		you = mine(func(id AuthorIdentity) bool { return strings.ToLower(id.Name) == myName })
	}
	if you != nil {
		you.You = true
	}

	keep := func(list []MapAuthor, limit int) {
		for i, au := range list {
			if i < limit || au.You {
				out.Authors = append(out.Authors, au)
			}
		}
	}
	keep(humans, maxMapPeople)
	keep(bots, maxMapBots)
	slices.SortFunc(out.Authors, order)
	return out
}
