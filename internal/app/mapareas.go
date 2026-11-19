package app

import (
	"cmp"
	"context"
	"encoding/json"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// The Map's Authors mode opens an author into the folders their commits touched (#812). The
// files of everyone's commits take minutes to read on a long history, so they're read for
// one author at a time, when that author is opened, and kept so that going a folder deeper
// costs no more git.

// maxAreaCommits is how many of an author's most recent commits a folder lists.
const maxAreaCommits = 5

// AreaCommit is one commit an author made in a folder, for the folder's card.
type AreaCommit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	// When is the committer date, in Unix seconds.
	When int64 `json:"when"`
}

// AuthorArea is one folder an author worked in, or the files that sit directly in a folder.
type AuthorArea struct {
	// Path is the folder, with forward slashes and no slash at the end: "frontend/src". For
	// Files it's the folder they sit in, "" for the top of the repository.
	Path string `json:"path"`
	// Name is what to call it beside its parent: "src/", or "top-level files".
	Name string `json:"name"`
	// Files marks the node for the files directly in Path, apart from its subfolders.
	Files bool `json:"files"`
	// Commits is how many of the author's commits in the window touched anything in it. A
	// commit counts once for each folder it touches, so folders don't add up to the author's
	// commits.
	Commits int `json:"commits"`
	// Updated is when the latest of those commits was made, in Unix seconds.
	Updated int64 `json:"updated"`
	// Folders is how many subfolders of it they touched: whether it opens, and into how many.
	Folders int `json:"folders"`
	// Recent are their latest commits in it, newest first.
	Recent []AreaCommit `json:"recent"`
}

// AuthorAreas are the folders one level under a folder that an author worked in.
type AuthorAreas struct {
	// Under is the folder asked about, "" for the top of the repository.
	Under string `json:"under"`
	// Areas are its subfolders and the files directly in it, most commits first.
	Areas []AuthorArea `json:"areas"`
	// More is how many were left out for having the fewest commits.
	More int `json:"more"`
	// Commits is how many commits of the author's in the window changed any file.
	Commits int `json:"commits"`
}

// MapAuthorAreas returns the folders under one folder that the person behind identities
// worked in over the last days days, in the repository at path. under is "" for the top of
// the repository. days of 0 or less means all of history. identities are the names and emails
// MapAuthors folded into the person.
func (a *App) MapAuthorAreas(path string, identities []AuthorIdentity, days int, under string) (AuthorAreas, error) {
	var since int64
	if days > 0 {
		since = time.Now().Unix() - int64(days)*24*60*60
	}
	return a.mapAuthorAreasSince(path, identities, days, since, under)
}

// mapAuthorAreasSince is MapAuthorAreas with the start of the window given. An author's
// folders are read once and kept, as MapAuthors keeps its answer.
func (a *App) mapAuthorAreasSince(repoPath string, identities []AuthorIdentity, days int, since int64, under string) (AuthorAreas, error) {
	under = strings.Trim(strings.ReplaceAll(under, "\\", "/"), "/")
	empty := AuthorAreas{Under: under, Areas: []AuthorArea{}}
	r, err := a.openRepo(repoPath)
	if err != nil {
		return empty, err
	}
	ctx := context.Background()
	refs := r.Refs().All
	trunk, ok := mapTrunkOf(refs)
	if !ok {
		return empty, nil
	}
	ids := make([]git.Identity, len(identities))
	for i, id := range identities {
		ids[i] = git.Identity{Name: id.Name, Email: id.Email}
	}
	state, err := refState(ctx, r.Git)
	if err != nil {
		return empty, err
	}
	shape, err := json.Marshal([]any{ids, a.branchRules(r.Path).Hide(), trunk.Name})
	if err != nil {
		return empty, err
	}
	key := strings.Join([]string{"areas", r.Path, strconv.Itoa(days), strconv.FormatInt(since/3600, 10), state, string(shape)}, "\x00")
	var tree *areaTree
	if v, ok := a.mapAuthors.get(key); ok {
		tree, _ = v.(*areaTree)
	}
	if tree == nil {
		// The trunk and every unmerged branch the Authors mode counts, each commit once.
		branches, _ := authorBranches(refs, trunk, since)
		revs := []string{trunk.Name}
		for _, b := range branches {
			revs = append(revs, b.Name)
		}
		tree = newAreaTree()
		if err := r.Git.AuthorFiles(ctx, trunk.Name, since, revs, ids, tree.add); err != nil {
			return empty, err
		}
		tree.index()
		a.mapAuthors.put(key, tree)
	}
	out := tree.under(under)
	return out, nil
}

// areaTree is every folder one author touched, with what they did in each.
type areaTree struct {
	// nodes are keyed by folder path; the files directly in a folder are keyed by its path
	// and filesMark.
	nodes map[string]*areaNode
	// kids lists the keys one level under each folder, once index has run.
	kids    map[string][]string
	commits int
}

// filesMark ends the key of a folder's own files. No path holds it.
const filesMark = "\x00"

type areaNode struct {
	commits int
	updated int64
	recent  []AreaCommit
}

func newAreaTree() *areaTree { return &areaTree{nodes: map[string]*areaNode{}} }

// add counts one commit in every folder it touches, each once.
func (t *areaTree) add(c git.AuthorFilesCommit) {
	touched := map[string]bool{}
	for _, p := range c.Paths {
		dir := path.Dir(strings.ReplaceAll(p, "\\", "/"))
		if dir == "." || dir == "/" {
			dir = ""
		}
		touched[dir+filesMark] = true
		for d := dir; d != ""; d = parentFolder(d) {
			if touched[d] {
				// Its folders above are in already.
				break
			}
			touched[d] = true
		}
	}
	if len(touched) == 0 {
		return
	}
	t.commits++
	for key := range touched {
		n := t.nodes[key]
		if n == nil {
			n = &areaNode{}
			t.nodes[key] = n
		}
		n.commits++
		n.updated = max(n.updated, c.Committed)
		n.recent = append(n.recent, AreaCommit{SHA: c.SHA, Subject: c.Subject, When: c.Committed})
		// Kept short as it goes: an author can have thousands of commits in one folder.
		if len(n.recent) > 4*maxAreaCommits {
			n.recent = newestAreaCommits(n.recent)
		}
	}
}

// parentFolder is the folder a folder is in, "" for one at the top.
func parentFolder(dir string) string {
	if i := strings.LastIndex(dir, "/"); i >= 0 {
		return dir[:i]
	}
	return ""
}

// newestAreaCommits keeps the latest commits, newest first; commits made in the same second
// go by their id, so the list is the same each time.
func newestAreaCommits(list []AreaCommit) []AreaCommit {
	slices.SortFunc(list, func(x, y AreaCommit) int { return cmp.Or(cmp.Compare(y.When, x.When), cmp.Compare(x.SHA, y.SHA)) })
	if len(list) > maxAreaCommits {
		list = list[:maxAreaCommits]
	}
	return list
}

// index lists each folder's children, once every commit is in, and trims the recent lists.
func (t *areaTree) index() {
	t.kids = map[string][]string{}
	for key, n := range t.nodes {
		n.recent = newestAreaCommits(n.recent)
		parent := parentFolder(key)
		if dir, files := strings.CutSuffix(key, filesMark); files {
			parent = dir
		}
		t.kids[parent] = append(t.kids[parent], key)
	}
}

// under returns what's one level under a folder: its subfolders and its own files, the most
// commits first, and no more than an author opens into.
func (t *areaTree) under(folder string) AuthorAreas {
	out := AuthorAreas{Under: folder, Areas: []AuthorArea{}, Commits: t.commits}
	for _, key := range t.kids[folder] {
		n := t.nodes[key]
		area := AuthorArea{Path: key, Commits: n.commits, Updated: n.updated, Recent: n.recent}
		if dir, files := strings.CutSuffix(key, filesMark); files {
			area.Path, area.Files = dir, true
			area.Name = "top-level files"
			if dir != "" {
				area.Name = "files in " + path.Base(dir) + "/"
			}
		} else {
			area.Name = path.Base(key) + "/"
			for _, kid := range t.kids[key] {
				if !strings.HasSuffix(kid, filesMark) {
					area.Folders++
				}
			}
		}
		out.Areas = append(out.Areas, area)
	}
	slices.SortFunc(out.Areas, func(x, y AuthorArea) int {
		return cmp.Or(cmp.Compare(y.Commits, x.Commits), cmp.Compare(y.Updated, x.Updated), cmp.Compare(x.Name, y.Name))
	})
	if len(out.Areas) > maxAuthorChildren {
		out.More = len(out.Areas) - maxAuthorChildren
		out.Areas = out.Areas[:maxAuthorChildren]
	}
	return out
}
