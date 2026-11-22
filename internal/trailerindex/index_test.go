package trailerindex

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func repoOf(t *testing.T, fx *gittest.Repo) *git.Repo {
	t.Helper()
	bin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	return git.OpenRepo(git.NewRunner(bin), fx.Dir)
}

// commit makes a commit with this message, exactly as given, and returns its id.
func commit(fx *gittest.Repo, file, message string) string {
	fx.WriteFile(file, message)
	fx.Git("add", "-A")
	fx.Git("commit", "-q", "--cleanup=verbatim", "-m", message)
	return fx.Head()
}

// counting is a Source that says how many commits it was asked to read.
type counting struct {
	Source
	read  int
	calls int
	// stopAfter, when set, ends the read after that many commits, as a closed tab does.
	stopAfter int
}

var errStopped = errors.New("stopped")

func (c *counting) CommitMessages(ctx context.Context, m git.MessageRange, each func(id, message string) error) error {
	c.calls++
	return c.Source.CommitMessages(ctx, m, func(id, message string) error {
		if c.stopAfter > 0 && c.read >= c.stopAfter {
			return errStopped
		}
		c.read++
		return each(id, message)
	})
}

func read(t *testing.T, x *Index, src Source) {
	t.Helper()
	if err := x.Read(context.Background(), src, nil); err != nil {
		t.Fatal(err)
	}
}

func keysOf(x *Index) []string {
	var out []string
	for _, k := range x.Keys() {
		out = append(out, k.Key)
	}
	return out
}

// tidewater is a small history with trailers of each shape the parser reads.
func tidewater(t *testing.T) (fx *gittest.Repo, ids map[string]string) {
	t.Helper()
	fx = gittest.New(t)
	ids = map[string]string{}
	ids["plain"] = commit(fx, "a.txt", "Start the tide table")
	ids["signed"] = commit(fx, "b.txt", "Round tide heights\n\nSigned-off-by: Sarah Lind <sarah@tidewater.example>")
	ids["closes"] = commit(fx, "c.txt", "Raise an alert on high wind\n\nIt was only logged before.\n\nCloses #12\nSigned-off-by: Sam Ortiz <sam@tidewater.example>\nReviewed-by: Sarah Lind <sarah@tidewater.example>")
	// A subject that looks like a trailer isn't one.
	ids["subject"] = commit(fx, "d.txt", "fix: read the gauge feed")
	ids["twice"] = commit(fx, "e.txt", "Pair on the alerts\n\nCo-authored-by: Sam Ortiz <sam@tidewater.example>\nCo-authored-by: Sarah Lind <sarah@tidewater.example>\nSigned-off-by: Sarah Lind <sarah@tidewater.example>")
	return fx, ids
}

func TestReadKeepsEveryCommitsTrailers(t *testing.T) {
	t.Parallel()
	fx, ids := tidewater(t)
	x := Open(t.TempDir(), fx.Dir)
	defer x.Close()
	read(t, x, repoOf(t, fx))

	st := x.Status()
	if !st.Done || st.Reading || st.Commits != 3 || st.Read != 5 || st.Total != 5 {
		t.Fatalf("status = %+v, want 3 commits with trailers of 5 read, and done", st)
	}
	want := []KeyStat{
		{Key: "Signed-off-by", Commits: 3, Lines: 3, Values: 2},
		{Key: "Closes", Commits: 1, Lines: 1, Values: 1},
		{Key: "Co-authored-by", Commits: 1, Lines: 2, Values: 2},
		{Key: "Reviewed-by", Commits: 1, Lines: 1, Values: 1},
	}
	if got := x.Keys(); !slices.Equal(got, want) {
		t.Fatalf("keys = %+v\nwant %+v", got, want)
	}
	values := x.Values("Signed-off-by")
	if len(values) != 2 || values[0] != (ValueStat{Value: "Sarah Lind <sarah@tidewater.example>", Commits: 2}) || values[1].Commits != 1 {
		t.Fatalf("values = %+v", values)
	}
	got := x.Commits("Signed-off-by", "Sarah Lind <sarah@tidewater.example>")
	slices.Sort(got)
	wantIDs := []string{ids["signed"], ids["twice"]}
	slices.Sort(wantIDs)
	if !slices.Equal(got, wantIDs) {
		t.Fatalf("commits = %v, want %v", got, wantIDs)
	}
	if all := x.Commits("Signed-off-by", ""); len(all) != 3 {
		t.Fatalf("every commit with the key: %v", all)
	}
	if none := x.Commits("Signed-off-by", "Nobody"); len(none) != 0 {
		t.Fatalf("a value nobody has: %v", none)
	}
	if none := x.Values("signed-off-by"); len(none) != 0 {
		t.Fatalf("keys are as written, and another spelling gave %v", none)
	}

	// A commit's own trailers, in its order, and the same as the commit panel reads.
	trailers, ok := x.Trailers(ids["closes"])
	if !ok || len(trailers) != 3 || trailers[0] != (git.Trailer{Key: "Closes", Value: "#12"}) || trailers[2].Key != "Reviewed-by" {
		t.Fatalf("trailers = %+v, %v", trailers, ok)
	}
	bin, _ := exec.LookPath("git")
	objects := git.NewObjectReader(git.NewRunner(bin), fx.Dir)
	defer func() { _ = objects.Close() }()
	for name, id := range ids {
		c, err := git.ReadCommit(objects, id)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := x.Trailers(id)
		if !slices.Equal(got, c.Trailers) {
			t.Errorf("%s: the index has %+v, the commit panel reads %+v", name, got, c.Trailers)
		}
	}
	if _, ok := x.Trailers(ids["subject"]); ok {
		t.Error("a subject was read as a trailer")
	}
}

// A second look reads nothing, and after a new commit it reads only that.
func TestReadOnlyReadsWhatIsNew(t *testing.T) {
	t.Parallel()
	fx, _ := tidewater(t)
	dir := t.TempDir()
	x := Open(dir, fx.Dir)
	src := &counting{Source: repoOf(t, fx)}
	read(t, x, src)
	if src.read != 5 {
		t.Fatalf("the first read took in %d commits, want 5", src.read)
	}
	read(t, x, src)
	if src.read != 5 || src.calls != 1 {
		t.Fatalf("with nothing new it read %d commits in %d walks", src.read, src.calls)
	}

	fresh := commit(fx, "f.txt", "Tidy the alert tests\n\nReviewed-by: Sam Ortiz <sam@tidewater.example>")
	fx.Branch("topic")
	fx.Checkout("topic")
	side := commit(fx, "g.txt", "Draft the gauge reader\n\nRefs: TIDE-7")
	read(t, x, src)
	if src.read != 7 {
		t.Fatalf("after two new commits it has read %d in all, want 7", src.read)
	}
	if got := x.Commits("Reviewed-by", "Sam Ortiz <sam@tidewater.example>"); !slices.Equal(got, []string{fresh}) {
		t.Fatalf("the new commit on main: %v", got)
	}
	if got := x.Commits("Refs", "TIDE-7"); !slices.Equal(got, []string{side}) {
		t.Fatalf("the new commit on a branch: %v", got)
	}
	x.Close()

	// Opened again, as after the app restarts: everything is there, and nothing is read.
	y := Open(dir, fx.Dir)
	defer y.Close()
	again := &counting{Source: repoOf(t, fx)}
	read(t, y, again)
	if again.read != 0 {
		t.Fatalf("after reopening it read %d commits, want none", again.read)
	}
	if st := y.Status(); !st.Done || st.Commits != 5 {
		t.Fatalf("status after reopening = %+v", st)
	}
	if !slices.Equal(keysOf(y), keysOf(x)) {
		t.Fatalf("keys after reopening = %v, were %v", keysOf(y), keysOf(x))
	}
	if got := y.Commits("Refs", "TIDE-7"); !slices.Equal(got, []string{side}) {
		t.Fatalf("after reopening: %v", got)
	}
}

// A read that's cut short keeps what it had and its place, and the next one carries on
// from there without reading those commits again.
func TestReadCarriesOnWhereItStopped(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	var ids []string
	for i := range 9 {
		ids = append(ids, commit(fx, "f.txt", "Commit "+string(rune('a'+i))+"\n\nStep: "+string(rune('a'+i))))
	}
	dir := t.TempDir()
	x := Open(dir, fx.Dir)
	src := &counting{Source: repoOf(t, fx), stopAfter: 4}
	if err := x.Read(context.Background(), src, nil); !errors.Is(err, errStopped) {
		t.Fatalf("err = %v, want the read to stop", err)
	}
	st := x.Status()
	if st.Done || st.Reading || st.Read != 4 || st.Commits != 4 {
		t.Fatalf("status after stopping = %+v, want 4 read and not done", st)
	}
	// Newest first: the four newest are in.
	for _, id := range ids[5:] {
		if _, ok := x.Trailers(id); !ok {
			t.Fatalf("%s wasn't kept", id)
		}
	}
	x.Close()

	y := Open(dir, fx.Dir)
	defer y.Close()
	rest := &counting{Source: repoOf(t, fx)}
	read(t, y, rest)
	if rest.read != 5 {
		t.Fatalf("carrying on read %d commits, want the other 5", rest.read)
	}
	if st := y.Status(); !st.Done || st.Commits != 9 {
		t.Fatalf("status = %+v, want all 9", st)
	}
	if got := y.Values("Step"); len(got) != 9 {
		t.Fatalf("values = %+v", got)
	}
}

// When a commit a cut-short read started from is gone, its place means nothing: the next
// read starts over, and counts nothing twice.
func TestReadStartsOverWhenItsPlaceIsGone(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	for i := range 4 {
		commit(fx, "f.txt", "Commit "+string(rune('a'+i))+"\n\nStep: "+string(rune('a'+i)))
	}
	fx.Branch("gone")
	fx.Checkout("gone")
	commit(fx, "g.txt", "On a branch that goes\n\nStep: side")
	fx.Checkout("main")
	dir := t.TempDir()
	x := Open(dir, fx.Dir)
	if err := x.Read(context.Background(), &counting{Source: repoOf(t, fx), stopAfter: 2}, nil); !errors.Is(err, errStopped) {
		t.Fatal(err)
	}
	x.Close()
	fx.Git("branch", "-D", "gone")
	fx.Git("reflog", "expire", "--expire=now", "--all")
	fx.Git("gc", "-q", "--prune=now")

	y := Open(dir, fx.Dir)
	defer y.Close()
	read(t, y, repoOf(t, fx))
	if st := y.Status(); !st.Done {
		t.Fatalf("status = %+v", st)
	}
	for _, v := range y.Values("Step") {
		if v.Commits != 1 {
			t.Fatalf("%q is counted %d times", v.Value, v.Commits)
		}
	}
	if got := len(y.Commits("Step", "")); got < 4 {
		t.Fatalf("%d commits with the key, want the 4 on main at least", got)
	}
}

func TestProgressSaysHowFar(t *testing.T) {
	t.Parallel()
	fx, _ := tidewater(t)
	x := New()
	var seen []Status
	if err := x.Read(context.Background(), repoOf(t, fx), func(s Status) { seen = append(seen, s) }); err != nil {
		t.Fatal(err)
	}
	if len(seen) < 2 || !seen[0].Reading || seen[0].Read != 0 {
		t.Fatalf("progress = %+v, want it to start by saying it's reading", seen)
	}
	last := seen[len(seen)-1]
	if last.Reading || last.Read != 5 || last.Total != 5 || !last.Done {
		t.Fatalf("the last word = %+v, want all 5 read and done", last)
	}
}

// An empty repository has nothing to read, and that's the end of it.
func TestReadOfAnEmptyRepository(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	x := Open(t.TempDir(), fx.Dir)
	defer x.Close()
	read(t, x, repoOf(t, fx))
	if st := x.Status(); !st.Done || st.Commits != 0 || len(x.Keys()) != 0 {
		t.Fatalf("status = %+v", st)
	}
}

// Two tabs on one repository get one index, and a file from another parser is read again.
func TestOpenSharesAndStartsOverForAnotherParser(t *testing.T) {
	t.Parallel()
	fx, _ := tidewater(t)
	dir := t.TempDir()
	x := Open(dir, fx.Dir)
	if again := Open(dir, fx.Dir); again != x {
		t.Fatal("the same repository opened twice gave two indexes")
	} else {
		again.Close()
	}
	read(t, x, repoOf(t, fx))
	path := x.path
	x.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no index file: %v", err)
	}
	if filepath.Dir(path) != dir || !strings.HasSuffix(path, suffix) {
		t.Fatalf("the file is %s", path)
	}

	y := Open(dir, fx.Dir)
	y.parser++ // as a release with a changed parser would have it
	src := &counting{Source: repoOf(t, fx)}
	read(t, y, src)
	y.Close()
	if src.read != 5 {
		t.Fatalf("with another parser it read %d commits, want all 5 again", src.read)
	}
}

// What a crash leaves at the end of the file is stepped over, and what's appended after
// it is read.
func TestLoadStepsOverADamagedFrame(t *testing.T) {
	t.Parallel()
	fx, _ := tidewater(t)
	dir := t.TempDir()
	x := Open(dir, fx.Dir)
	read(t, x, repoOf(t, fx))
	path := x.path
	x.Close()

	// Half a frame, as a crash mid-write leaves.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	torn := encodeBatch(passBatch{nonce: 7, done: 3, strs: []string{"Key", "value"}})
	if _, err := f.Write(torn[:len(torn)-4]); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	commit(fx, "z.txt", "After the crash\n\nRefs: TIDE-9")
	y := Open(dir, fx.Dir)
	src := &counting{Source: repoOf(t, fx)}
	read(t, y, src)
	y.Close()
	if src.read != 1 {
		t.Fatalf("after the damage it read %d commits, want only the new one", src.read)
	}

	z := Open(dir, fx.Dir)
	defer z.Close()
	z.load()
	if st := z.Status(); st.Commits != 4 {
		t.Fatalf("loaded %d commits, want the 3 from before the damage and the 1 after", st.Commits)
	}
	if got := z.Commits("Refs", "TIDE-9"); len(got) != 1 {
		t.Fatalf("the commit appended after the damage: %v", got)
	}
}

// Two copies of the app with one repository open each read it and each append what they
// read, to the same file. Loaded, a commit is there once.
func TestTwoWritersCountACommitOnce(t *testing.T) {
	t.Parallel()
	fx, _ := tidewater(t)
	dir := t.TempDir()
	path := fileFor(dir, fx.Dir)
	// Not through Open, which would hand both the same index, as it should in one process.
	for range 2 {
		x := &Index{path: path, parser: git.TrailerParserVersion}
		// Neither has loaded what the other wrote: both believe the file is theirs to fill.
		x.loaded, x.first = true, []uint32{0}
		read(t, x, repoOf(t, fx))
	}

	x := Open(dir, fx.Dir)
	defer x.Close()
	src := &counting{Source: repoOf(t, fx)}
	read(t, x, src)
	if src.read != 0 {
		t.Fatalf("it read %d commits, though both writers had finished", src.read)
	}
	if st := x.Status(); st.Commits != 3 {
		t.Fatalf("%d commits held, want 3: each once", st.Commits)
	}
	for _, k := range x.Keys() {
		if k.Key == "Signed-off-by" && (k.Commits != 3 || k.Lines != 3) {
			t.Fatalf("Signed-off-by = %+v, want 3 commits counted once each", k)
		}
	}
}

func TestClearUnusedRemovesOnlyOldIndexFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := filepath.Join(dir, "old"+suffix)
	recent := filepath.Join(dir, "recent"+suffix)
	other := filepath.Join(dir, "notes.txt")
	for _, p := range []string{old, recent, other} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	long := now.Add(-unusedFor - time.Hour)
	for _, p := range []string{old, other} {
		if err := os.Chtimes(p, long, long); err != nil {
			t.Fatal(err)
		}
	}
	clearUnused(dir, now)
	if _, err := os.Stat(old); err == nil {
		t.Error("an index nobody has opened in months is still there")
	}
	for _, p := range []string{recent, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed", filepath.Base(p))
		}
	}
}
