package app

import (
	"slices"
	"strconv"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// commitFiles commits several files at once as somebody, with any trailers.
func commitFiles(fx *gittest.Repo, name, email, subject string, files []string, trailers ...string) {
	for _, f := range files {
		fx.WriteFile(f, subject+"\n")
		fx.Git("add", "--", f)
	}
	args := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "commit", "-q", "-m", subject}
	for _, tr := range trailers {
		args = append(args, "-m", tr)
	}
	fx.Git(args...)
}

func area(t *testing.T, got AuthorAreas, name string) AuthorArea {
	t.Helper()
	for _, a := range got.Areas {
		if a.Name == name {
			return a
		}
	}
	var names []string
	for _, a := range got.Areas {
		names = append(names, a.Name)
	}
	t.Fatalf("no area %q among %v", name, names)
	return AuthorArea{}
}

func areaNames(got AuthorAreas) []string {
	names := []string{}
	for _, a := range got.Areas {
		names = append(names, a.Name+" "+strconv.Itoa(a.Commits))
	}
	return names
}

var maya = []AuthorIdentity{{Name: "Maya", Email: "maya@example.com"}}

func TestMapAuthorAreasCountACommitOnceInEachFolderItTouches(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	commitFiles(fx, "Maya", "maya@example.com", "one", []string{"frontend/src/a.ts", "frontend/src/b.ts", "frontend/e2e/a.spec.ts"})
	commitFiles(fx, "Maya", "maya@example.com", "two", []string{"frontend/src/lib/c.ts", "internal/git/log.go"})
	commitFiles(fx, "Maya", "maya@example.com", "three", []string{"README.md", "internal/app/app.go"})
	commitFiles(fx, "Maya", "maya@example.com", "four", []string{"frontend/package.json"})
	// Somebody else's work isn't hers.
	commitFiles(fx, "Leo", "leo@example.com", "five", []string{"docs/guide.md", "frontend/src/d.ts"})
	path := openForMap(t, a, fx)

	top, err := a.mapAuthorAreasSince(path, maya, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	// Three of her four commits touch frontend, two touch internal, one a file at the top.
	if got := areaNames(top); !slices.Equal(got, []string{"frontend/ 3", "internal/ 2", "top-level files 1"}) {
		t.Fatalf("top = %v", got)
	}
	if top.Commits != 4 || top.More != 0 || top.Under != "" {
		t.Fatalf("top = %+v, want her 4 commits", top)
	}
	fe := area(t, top, "frontend/")
	if fe.Path != "frontend" || fe.Files || fe.Folders != 2 || fe.Updated == 0 {
		t.Fatalf("frontend = %+v, want a folder with src and e2e under it", fe)
	}
	if files := area(t, top, "top-level files"); !files.Files || files.Path != "" || files.Folders != 0 {
		t.Fatalf("top-level files = %+v", files)
	}

	// A level down, from what was read the first time: her folders, and the files beside them.
	in, err := a.mapAuthorAreasSince(path, maya, 0, 0, "frontend/")
	if err != nil {
		t.Fatal(err)
	}
	if got := areaNames(in); !slices.Equal(got, []string{"src/ 2", "e2e/ 1", "files in frontend/ 1"}) {
		t.Fatalf("frontend = %v", got)
	}
	if src := area(t, in, "src/"); src.Path != "frontend/src" || src.Folders != 1 {
		t.Fatalf("src = %+v, want lib under it", src)
	}
	deep, _ := a.mapAuthorAreasSince(path, maya, 0, 0, "frontend/src")
	if got := areaNames(deep); !slices.Equal(got, []string{"files in src/ 1", "lib/ 1"}) {
		t.Fatalf("frontend/src = %v", got)
	}
	// A folder she never touched has nothing under it.
	if none, err := a.mapAuthorAreasSince(path, maya, 0, 0, "docs"); err != nil || len(none.Areas) != 0 || none.Areas == nil {
		t.Fatalf("docs = %+v, %v", none, err)
	}
}

func TestMapAuthorAreasListTheLatestCommitsInAFolder(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	var shas []string
	for i := range 7 {
		commitFiles(fx, "Maya", "maya@example.com", "change "+strconv.Itoa(i), []string{"src/f" + strconv.Itoa(i) + ".go"})
		shas = append(shas, fx.Head())
		// The fixture's clock moves on with its own commits.
		fx.CommitFile("tick"+strconv.Itoa(i)+".txt", "1", "tick")
	}
	top, err := a.mapAuthorAreasSince(openForMap(t, a, fx), maya, 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	src := area(t, top, "src/")
	if src.Commits != 7 || len(src.Recent) != maxAreaCommits {
		t.Fatalf("src = %d commits, %d listed; want 7 and the latest %d", src.Commits, len(src.Recent), maxAreaCommits)
	}
	for i, c := range src.Recent {
		if want := shas[len(shas)-1-i]; c.SHA != want || c.Subject != "change "+strconv.Itoa(6-i) || c.When == 0 {
			t.Errorf("recent[%d] = %+v, want %s", i, c, want)
		}
	}
}

func TestMapAuthorAreasFollowTheWholePersonAndTheWindow(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	commitFiles(fx, "Maya", "maya@example.com", "before the window", []string{"old/a.txt"})
	fx.CommitFile("tick.txt", "1", "tick")
	commitFiles(fx, "mokafor", "maya@work.example.com", "under another name", []string{"api/a.go"})
	since, _ := strconv.ParseInt(fx.Git("log", "-1", "--format=%ct"), 10, 64)
	// Credited on somebody else's commit; and only mentioned in another, which isn't hers.
	commitFiles(fx, "Leo", "leo@example.com", "paired", []string{"web/a.ts"}, "Co-authored-by: Maya <maya@example.com>")
	commitFiles(fx, "Leo", "leo@example.com", "Thanks to <maya@example.com>", []string{"thanks/a.txt"})
	// On a branch the trunk doesn't have.
	fx.Git("checkout", "-q", "-b", "topic")
	commitFiles(fx, "Maya", "maya@example.com", "on a branch", []string{"branch/a.txt"})
	fx.Checkout("main")
	// A merge is nobody's.
	fx.Git("checkout", "-q", "-b", "side", "HEAD~1")
	commitFiles(fx, "Leo", "leo@example.com", "side", []string{"side/a.txt"})
	fx.Checkout("main")
	fx.Git("-c", "user.name=Maya", "-c", "user.email=maya@example.com", "merge", "-q", "--no-ff", "-m", "Merge branch 'side'", "side")
	path := openForMap(t, a, fx)

	both := []AuthorIdentity{{Name: "Maya", Email: "maya@example.com"}, {Name: "mokafor", Email: "maya@work.example.com"}}
	got, err := a.mapAuthorAreasSince(path, both, 14, since, "")
	if err != nil {
		t.Fatal(err)
	}
	// All made in the fixture's same minute, so they come by name.
	if names := areaNames(got); !slices.Equal(names, []string{"api/ 1", "branch/ 1", "web/ 1"}) {
		t.Fatalf("in the window = %v, want what she wrote under both names, on a branch, and was credited on", names)
	}
	if got.Commits != 3 {
		t.Errorf("commits = %d, want 3", got.Commits)
	}
	// All of history brings back what's older.
	all, _ := a.mapAuthorAreasSince(path, both, 0, 0, "")
	if _, old := area(t, all, "old/"), 0; old != 0 || all.Commits != 4 {
		t.Fatalf("all of history = %+v", all)
	}
	// Under one name alone, less.
	one, _ := a.mapAuthorAreasSince(path, both[1:], 0, 0, "")
	if names := areaNames(one); !slices.Equal(names, []string{"api/ 1"}) {
		t.Fatalf("under one name = %v", names)
	}
	// Nobody at all has no folders.
	if none, err := a.mapAuthorAreasSince(path, nil, 0, 0, ""); err != nil || len(none.Areas) != 0 {
		t.Fatalf("nobody = %+v, %v", none, err)
	}
}

// Where someone has worked counts a commit that credits them under a name the mailmap maps
// to them, as the map and their card do (#843).
func TestMapAuthorAreasCountCreditsUnderAMappedName(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	fx.WriteFile(".mailmap", "Jane Doe <jane@example.com> <jdoe@old.example.com>\n")
	commitFiles(fx, "Jane Doe", "jane@example.com", "hers", []string{"api/a.go"})
	commitFiles(fx, "Sam", "sam@example.com", "theirs together", []string{"docs/guide.md"}, "Co-authored-by: jdoe <jdoe@old.example.com>")
	commitFiles(fx, "Sam", "sam@example.com", "sam alone", []string{"web/app.ts"})
	got, err := a.MapAuthorAreas(openForMap(t, a, fx), []AuthorIdentity{{Name: "Jane Doe", Email: "jane@example.com"}}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if docs := area(t, got, "docs/"); docs.Commits != 1 {
		t.Errorf("docs/ = %+v, want the commit that credits her under her old address", docs)
	}
	if len(got.Areas) != 2 {
		t.Errorf("areas = %+v, want api/ and docs/ and nothing of Sam's alone", got.Areas)
	}
}

func TestMapAuthorAreasShowTheMostWorkedInAndCountTheRest(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Linear(1)
	var files []string
	for i := range maxAuthorChildren + 3 {
		files = append(files, "dir"+strconv.Itoa(100+i)+"/a.txt")
	}
	commitFiles(fx, "Maya", "maya@example.com", "everywhere", files)
	commitFiles(fx, "Maya", "maya@example.com", "again", []string{"dir102/b.txt"})
	got, err := a.MapAuthorAreas(openForMap(t, a, fx), maya, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Areas) != maxAuthorChildren || got.More != 3 {
		t.Fatalf("%d areas and %d more, want %d and 3", len(got.Areas), got.More, maxAuthorChildren)
	}
	if got.Areas[0].Name != "dir102/" || got.Areas[0].Commits != 2 {
		t.Fatalf("first = %+v, want the folder with the most commits", got.Areas[0])
	}
}
