package app

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// commitAs commits a new file as somebody else, with any trailers after the subject.
func commitAs(fx *gittest.Repo, name, email, file, subject string, trailers ...string) {
	fx.WriteFile(file, subject+"\n")
	fx.Git("add", "--", file)
	args := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "commit", "-q", "-m", subject}
	if len(trailers) > 0 {
		args = append(args, "-m", strings.Join(trailers, "\n"))
	}
	fx.Git(args...)
}

func mapAuthor(t *testing.T, m AuthorMap, name string) MapAuthor {
	t.Helper()
	for _, au := range m.Authors {
		if au.Name == name {
			return au
		}
	}
	var names []string
	for _, au := range m.Authors {
		names = append(names, au.Name)
	}
	t.Fatalf("no author %q among %v", name, names)
	return MapAuthor{}
}

func allAuthors(t *testing.T, a *App, fx *gittest.Repo) AuthorMap {
	t.Helper()
	m, err := a.mapAuthorsSince(openForMap(t, a, fx), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMapAuthorsCountsAuthorsAndCoAuthorsButNotMerges(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(3) // Test User
	commitAs(fx, "Ada", "ada@example.com", "a1.txt", "Ada alone")
	commitAs(fx, "Ada", "ada@example.com", "a2.txt", "Ada with Bo", "Co-authored-by: Bo <bo@example.com>")
	// The kernel's trailer counts as GitHub's does, and a trailer naming the author adds nothing.
	commitAs(fx, "Ada", "ada@example.com", "a3.txt", "Ada with Cy", "Signed-off-by: Ada <ada@example.com>", "Co-developed-by: Cy <cy@example.com>", "Co-authored-by: Ada <ada@example.com>")
	// A merge commit is nobody's work.
	fx.Git("checkout", "-q", "-b", "side", "HEAD~1")
	commitAs(fx, "Bo", "bo@example.com", "b1.txt", "Bo alone")
	fx.Checkout("main")
	fx.Git("merge", "-q", "--no-ff", "-m", "Merge branch 'side'", "side")

	m := allAuthors(t, a, fx)
	if m.Trunk.Name != "main" || m.Trunk.Commits != 8 {
		t.Fatalf("trunk = %+v, want main with all 8 commits", m.Trunk)
	}
	if m.Commits != 7 || m.People != 4 || m.Bots != 0 {
		t.Fatalf("commits = %d, people = %d, bots = %d; want 7 commits (no merge) by 4 people", m.Commits, m.People, m.Bots)
	}
	for name, want := range map[string][3]int{"Test User": {3, 3, 0}, "Ada": {3, 3, 0}, "Bo": {2, 1, 1}, "Cy": {1, 0, 1}} {
		au := mapAuthor(t, m, name)
		if got := [3]int{au.Commits, au.Authored, au.CoAuthored}; got != want {
			t.Errorf("%s: commits, authored, co-authored = %v, want %v", name, got, want)
		}
		if au.Trunk != au.Commits || len(au.Branches) != 0 {
			t.Errorf("%s: %d on the trunk and branches %v, want everything landed", name, au.Trunk, au.Branches)
		}
		if au.Updated == 0 || au.Bot {
			t.Errorf("%s = %+v", name, au)
		}
	}
	// The most commits first; a tie goes to whoever committed last.
	if got := []string{m.Authors[0].Name, m.Authors[1].Name, m.Authors[2].Name, m.Authors[3].Name}; !slices.Equal(got, []string{"Ada", "Test User", "Bo", "Cy"}) {
		t.Errorf("order = %v", got)
	}
	if you := mapAuthor(t, m, "Test User"); !you.You || you.Key != "test@example.com" {
		t.Errorf("you = %+v, want the repository's user marked", you)
	}
	if mapAuthor(t, m, "Ada").You {
		t.Error("Ada is marked as you")
	}
}

func TestMapAuthorsFoldsOnePersonsIdentities(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	// One person three ways: a second name on the same email, and a second email on the
	// same name. Nothing joins the first and the third directly.
	commitAs(fx, "Maya Okafor", "maya@users.example.com", "m1.txt", "one")
	commitAs(fx, "Maya Okafor", "maya@users.example.com", "m2.txt", "two")
	commitAs(fx, "mokafor", "maya@users.example.com", "m3.txt", "three")
	commitAs(fx, "Maya Okafor", "maya@work.example.com", "m4.txt", "four")
	// Case doesn't part people, and a co-author's trailer folds in too.
	commitAs(fx, "Leo", "leo@example.com", "l1.txt", "five", "Co-authored-by: MOKAFOR <Maya@Users.Example.com>")
	// Somebody else entirely.
	commitAs(fx, "Priya", "priya@example.com", "p1.txt", "six")

	m := allAuthors(t, a, fx)
	if m.People != 4 {
		t.Fatalf("people = %d, want Test User, Maya, Leo and Priya", m.People)
	}
	maya := mapAuthor(t, m, "Maya Okafor")
	if maya.Commits != 5 || maya.Authored != 4 || maya.CoAuthored != 1 {
		t.Fatalf("maya = %+v, want 4 written and 1 credited", maya)
	}
	if maya.Email != "maya@users.example.com" || maya.Key != "maya@users.example.com" {
		t.Errorf("maya is named for %q <%s> (key %q), want the identity used most", maya.Name, maya.Email, maya.Key)
	}
	var ids []string
	for _, id := range maya.Identities {
		ids = append(ids, id.Name+" <"+id.Email+"> "+strconv.Itoa(id.Commits))
	}
	want := []string{"Maya Okafor <maya@users.example.com> 2", "MOKAFOR <Maya@Users.Example.com> 1", "Maya Okafor <maya@work.example.com> 1", "mokafor <maya@users.example.com> 1"}
	if !slices.Equal(ids, want) {
		t.Errorf("identities = %q\nwant         %q", ids, want)
	}
}

// The Map reads the mailmap the graph does, so one that's saved and not yet committed
// counts, and what was counted before it was saved is dropped.
func TestMapAuthorsFollowASavedMailmap(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	commitAs(fx, "jdoe", "jdoe@old.example.com", "j1.txt", "one")
	commitAs(fx, "Sam", "sam@example.com", "s1.txt", "two", "Co-authored-by: jdoe <jdoe@old.example.com>")
	path := openForMap(t, a, fx)

	before, err := a.mapAuthorsSince(path, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if jdoe := mapAuthor(t, before, "jdoe"); jdoe.Commits != 2 {
		t.Fatalf("before any mailmap, jdoe = %+v", jdoe)
	}

	// No ref moves: only the file in the work tree is new.
	fx.WriteFile(".mailmap", "Jane Doe <jane@example.com> <jdoe@old.example.com>\n")
	after, err := a.mapAuthorsSince(path, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	jane := mapAuthor(t, after, "Jane Doe")
	if jane.Commits != 2 || jane.Authored != 1 || jane.CoAuthored != 1 || jane.Email != "jane@example.com" {
		t.Fatalf("jane = %+v, want the author and the co-author both mapped to her by the saved mailmap", jane)
	}
	for _, who := range after.Authors {
		if who.Name == "jdoe" {
			t.Fatalf("jdoe is still on the map after the mailmap: %+v", who)
		}
	}
	// Her card finds the commit she wrote under the old name, and the one that credits her
	// under it in a trailer, which git leaves as it was written (#843).
	h, err := a.MapAuthorHistory(path, []AuthorIdentity{{Name: "Jane Doe", Email: "jane@example.com"}})
	if err != nil || h.Authored != 1 || h.CoAuthored != 1 {
		t.Fatalf("her card = %+v, %v, want the commit she wrote and the one that credits her", h, err)
	}

	// Taking the file away puts jdoe back.
	fx.RemoveFile(".mailmap")
	again, err := a.mapAuthorsSince(path, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if jdoe := mapAuthor(t, again, "jdoe"); jdoe.Commits != 2 {
		t.Fatalf("with the mailmap gone, jdoe = %+v", jdoe)
	}
}

func TestMapAuthorsReadACommittedMailmap(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	commitAs(fx, "jdoe", "jdoe@old.example.com", "j1.txt", "one")
	commitAs(fx, "Sam", "sam@example.com", "s1.txt", "two", "Co-authored-by: jdoe <jdoe@old.example.com>")
	fx.CommitFile(".mailmap", "Jane Doe <jane@example.com> <jdoe@old.example.com>\n", "Add a mailmap")

	m := allAuthors(t, a, fx)
	jane := mapAuthor(t, m, "Jane Doe")
	if jane.Commits != 2 || jane.Authored != 1 || jane.CoAuthored != 1 || jane.Email != "jane@example.com" {
		t.Fatalf("jane = %+v, want the author and the co-author both mapped to her", jane)
	}
	if len(jane.Identities) != 1 {
		t.Errorf("identities = %+v, want only the mapped one", jane.Identities)
	}
}

func TestMapAuthorsMarkBots(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	commitAs(fx, "renovate[bot]", "renovate@example.com", "r1.txt", "Update vite")
	commitAs(fx, "Release Robot", "robot@bots.example.com", "r2.txt", "Release 1.0")
	path := openForMap(t, a, fx)

	m, err := a.mapAuthorsSince(path, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.People != 2 || m.Bots != 1 || !mapAuthor(t, m, "renovate[bot]").Bot || mapAuthor(t, m, "Release Robot").Bot {
		t.Fatalf("people = %d, bots = %d, authors = %+v", m.People, m.Bots, m.Authors)
	}

	// A pattern of the repository's own makes another, and the answer kept from before
	// doesn't hide it.
	p, _ := a.RepoPreferences(path)
	p.BotAuthors = []string{"*@bots.example.com"}
	if err := a.SetRepoPreferences(path, p); err != nil {
		t.Fatal(err)
	}
	m, err = a.mapAuthorsSince(path, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.People != 1 || m.Bots != 2 || !mapAuthor(t, m, "Release Robot").Bot {
		t.Fatalf("with the pattern: people = %d, bots = %d", m.People, m.Bots)
	}
}

func TestMapAuthorsSplitTheTrunkFromUnmergedBranches(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(2)
	commitAs(fx, "Ada", "ada@example.com", "a1.txt", "landed")
	// topic has two of Ada's and one of Bo's; stacked sits on it with one more of Ada's.
	fx.Git("checkout", "-q", "-b", "topic")
	commitAs(fx, "Ada", "ada@example.com", "t1.txt", "topic one")
	commitAs(fx, "Ada", "ada@example.com", "t2.txt", "topic two")
	commitAs(fx, "Bo", "bo@example.com", "t3.txt", "topic three")
	fx.Git("checkout", "-q", "-b", "stacked")
	commitAs(fx, "Ada", "ada@example.com", "s1.txt", "stacked one")
	fx.Checkout("main")
	// A branch with nothing of its own gives nobody a node.
	fx.Git("branch", "merged", "HEAD~1")

	m := allAuthors(t, a, fx)
	ada := mapAuthor(t, m, "Ada")
	// Four commits: the one on both branches is one commit.
	if ada.Commits != 4 || ada.Trunk != 1 {
		t.Fatalf("ada = %d commits, %d on the trunk; want 4 and 1", ada.Commits, ada.Trunk)
	}
	on := map[string]int{}
	for _, b := range ada.Branches {
		on[b.Name] = b.Commits
		if b.Kind != "branch" || b.Parent != ada.Key || b.Commit == "" || b.Updated == 0 {
			t.Errorf("branch node = %+v", b)
		}
	}
	if len(on) != 2 || on["topic"] != 2 || on["stacked"] != 3 {
		t.Fatalf("ada's branches = %v, want topic 2 and stacked 3 (it holds topic's too)", on)
	}
	bo := mapAuthor(t, m, "Bo")
	if bo.Commits != 1 || bo.Trunk != 0 || len(bo.Branches) != 2 {
		t.Fatalf("bo = %+v, want one commit, on both branches and not the trunk", bo)
	}
	if m.Commits != 7 {
		t.Errorf("commits = %d, want 7, each once", m.Commits)
	}
}

func TestMapAuthorsCountOnlyTheWindow(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(3)
	fx.Git("branch", "old")
	commitAs(fx, "Ada", "ada@example.com", "a1.txt", "before")
	// The fixture's clock moves on after each commit of its own, so this one shares Ada's
	// minute and the window opens at the commit after it.
	fx.CommitFile("u0.txt", "0", "before too")
	fx.CommitFile("u1.txt", "1", "inside one")
	since, err := strconv.ParseInt(fx.Git("log", "-1", "--format=%ct"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	fx.Git("checkout", "-q", "-b", "topic")
	fx.CommitFile("u2.txt", "2", "inside two")
	fx.Checkout("main")
	path := openForMap(t, a, fx)

	m, err := a.mapAuthorsSince(path, 14, since)
	if err != nil {
		t.Fatal(err)
	}
	if m.Since != since || m.Commits != 2 || m.People != 1 || len(m.Authors) != 1 {
		t.Fatalf("window = %+v, want only the two commits inside it", m)
	}
	you := m.Authors[0]
	if you.Commits != 2 || you.Trunk != 1 || len(you.Branches) != 1 || you.Branches[0].Name != "topic" {
		t.Fatalf("you = %+v", you)
	}
	// The trunk in the middle is as big as ever.
	if m.Trunk.Commits != 6 {
		t.Errorf("trunk commits = %d, want all 6", m.Trunk.Commits)
	}

	// A new commit moves a ref, and the answer kept from before isn't served.
	fx.CommitFile("u3.txt", "3", "inside three")
	m, err = a.mapAuthorsSince(path, 14, since)
	if err != nil {
		t.Fatal(err)
	}
	if m.Commits != 3 {
		t.Fatalf("after another commit: %d commits, want 3", m.Commits)
	}
}

func TestMapAuthorsOfAnEmptyRepository(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.MapAuthors(tab.Path, 14)
	if err != nil {
		t.Fatal(err)
	}
	if m.Trunk.Name != "" || m.Authors == nil || len(m.Authors) != 0 {
		t.Fatalf("map = %+v, want no trunk and an empty list", m)
	}
}

// A trailer keeps the name it was written with, so an author's card reads it through the
// mailmap as the map does: by the line that applies to that name and address, and no other.
func TestMapAuthorHistoryCountsCreditsUnderAMappedName(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	// Two people shared an address once. Only "jdoe" at it is Jane.
	fx.WriteFile(".mailmap", "Jane Doe <jane@example.com> jdoe <team@example.com>\nJane Doe <jane@example.com> <jdoe@old.example.com>\n")
	fx.Git("add", "--", ".mailmap")
	fx.Git("commit", "-q", "-m", "Add a mailmap")
	commitAs(fx, "Sam", "sam@example.com", "s1.txt", "one", "Co-authored-by: jdoe <team@example.com>")
	commitAs(fx, "Sam", "sam@example.com", "s2.txt", "two", "Co-authored-by: JDoe <TEAM@example.com>")
	commitAs(fx, "Sam", "sam@example.com", "s3.txt", "three", "Co-developed-by: Jay <jdoe@old.example.com>")
	commitAs(fx, "Sam", "sam@example.com", "s4.txt", "four", "Co-authored-by: Jane Doe <jane@example.com>")
	// Not hers: the other person at the shared address, and a message that only mentions her
	// old address.
	commitAs(fx, "Sam", "sam@example.com", "s5.txt", "five", "Co-authored-by: Robin <team@example.com>")
	commitAs(fx, "Sam", "sam@example.com", "s6.txt", "Thanks to <jdoe@old.example.com> for the idea")
	// Hers to have written, not to be credited on, though it names her too.
	commitAs(fx, "jdoe", "jdoe@old.example.com", "j1.txt", "seven", "Co-authored-by: jdoe <team@example.com>")
	path := openForMap(t, a, fx)

	jane := []AuthorIdentity{{Name: "Jane Doe", Email: "jane@example.com"}}
	h, err := a.MapAuthorHistory(path, jane)
	if err != nil {
		t.Fatal(err)
	}
	if h.Authored != 1 || h.CoAuthored != 4 {
		t.Fatalf("history = %+v, want 1 written and 4 credited: three under names the mailmap maps to her, one under her own", h)
	}
	// The map counts her the same way.
	m, err := a.mapAuthorsSince(path, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if on := mapAuthor(t, m, "Jane Doe"); on.Authored != h.Authored || on.CoAuthored != h.CoAuthored {
		t.Errorf("the map says %d written and %d credited, her card %d and %d", on.Authored, on.CoAuthored, h.Authored, h.CoAuthored)
	}

	// Robin, at the shared address, is credited once, and not with what's Jane's.
	robin, err := a.MapAuthorHistory(path, []AuthorIdentity{{Name: "Robin", Email: "team@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if robin.Authored != 0 || robin.CoAuthored != 1 {
		t.Errorf("Robin's history = %+v, want only the commit that credits Robin", robin)
	}
}

func TestMapAuthorHistory(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(2)
	commitAs(fx, "Maya Okafor", "maya@users.example.com", "m1.txt", "one")
	first, _ := strconv.ParseInt(fx.Git("log", "-1", "--format=%ct"), 10, 64)
	fx.CommitFile("gap.txt", "1", "somebody else")
	commitAs(fx, "mokafor", "maya@work.example.com", "m2.txt", "two")
	commitAs(fx, "Leo", "leo@example.com", "l1.txt", "three", "Co-authored-by: Maya Okafor <maya@users.example.com>")
	// A commit that only mentions her isn't hers.
	commitAs(fx, "Leo", "leo@example.com", "l2.txt", "Thanks to <maya@users.example.com> for the idea")
	fx.Git("checkout", "-q", "-b", "topic")
	commitAs(fx, "Maya Okafor", "maya@users.example.com", "m3.txt", "on a branch")
	last, _ := strconv.ParseInt(fx.Git("log", "-1", "--format=%ct"), 10, 64)
	fx.Checkout("main")

	h, err := a.MapAuthorHistory(openForMap(t, a, fx), []AuthorIdentity{{Name: "Maya Okafor", Email: "maya@users.example.com"}, {Name: "mokafor", Email: "MAYA@work.example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if h.Authored != 3 || h.CoAuthored != 1 {
		t.Fatalf("history = %+v, want 3 written (one on a branch) and 1 credited", h)
	}
	if h.First != first || h.Last != last {
		t.Errorf("first, last = %d, %d; want %d, %d", h.First, h.Last, first, last)
	}
	if h, err := a.MapAuthorHistory(openForMap(t, a, fx), nil); err != nil || h.Authored != 0 {
		t.Errorf("nobody's history = %+v, %v", h, err)
	}
}
