package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func commitAs(fx *gittest.Repo, name, email, msg string, trailers ...string) {
	args := []string{"-c", "user.name=" + name, "-c", "user.email=" + email, "commit", "-q", "--allow-empty", "-m", msg}
	if len(trailers) > 0 {
		args = append(args, "-m", strings.Join(trailers, "\n"))
	}
	fx.Git(args...)
}

func TestHistoryIdentities(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1) // Test User
	commitAs(fx, "Ada", "ada@example.com", "Pair up",
		"Reviewed-by: Rex <rex@example.com>", "Co-authored-by: Bo <bo@example.com>", "Co-authored-by: Bo <bo@example.com>", "Co-developed-by: Cy <cy@example.com>")
	commitAs(fx, "Bo", "bo@example.com", "Credit myself", "Co-authored-by: Bo <bo@example.com>")
	fx.Git("checkout", "-q", "-b", "side", "HEAD~1")
	commitAs(fx, "ada", "ADA@example.com", "On the side")
	fx.Checkout("main")
	fx.Git("-c", "user.name=Ada", "-c", "user.email=ada@example.com", "merge", "-q", "--no-ff", "-m", "Merge branch 'side'", "side")
	r := OpenRepo(newTestRunner(t), fx.Dir)

	ids, commits, err := r.HistoryIdentities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if commits != 5 {
		t.Errorf("read %d commits, want 5 with the merge", commits)
	}
	got := map[Identity][2]int{}
	for _, h := range ids {
		got[h.Identity] = [2]int{h.Authored, h.CoAuthored}
	}
	want := map[Identity][2]int{
		{Name: "Test User", Email: "test@example.com"}: {1, 0},
		{Name: "Ada", Email: "ada@example.com"}:        {2, 0}, // her commit and the merge
		{Name: "ada", Email: "ADA@example.com"}:        {1, 0}, // as written: another identity
		{Name: "Bo", Email: "bo@example.com"}:          {1, 1}, // credited once on Ada's, and not on his own
		{Name: "Cy", Email: "cy@example.com"}:          {0, 1},
	}
	if len(got) != len(want) {
		t.Fatalf("identities = %+v\nwant %+v", got, want)
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s <%s> = %v, want %v (written, co-authored)", id.Name, id.Email, got[id], w)
		}
	}

	// A repository with no commits names nobody.
	empty := OpenRepo(newTestRunner(t), gittest.New(t).Dir)
	if ids, commits, err := empty.HistoryIdentities(context.Background()); err != nil || len(ids) != 0 || commits != 0 {
		t.Fatalf("an empty repository gave %v, %d, %v", ids, commits, err)
	}
}

func TestMailmapSources(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	// With no mailmap anywhere, the work tree's is still listed: it's where one would go.
	got, err := r.MailmapSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Source != MailmapWorkTree || got[0].Exists || !got[0].Editable || got[0].Lines != 0 {
		t.Fatalf("with no mailmap: %+v", got)
	}
	if text, err := r.ReadMailmapSource(ctx, MailmapWorkTree); err != nil || text != "" {
		t.Fatalf("a missing .mailmap read as %q, %v", text, err)
	}

	fx.CommitFile(".mailmap", "# people\nJane Doe <jdoe@old.example>\nnot a line\n", "Add a mailmap")
	fx.WriteFile("mine.txt", "Me <me@example.com> <old-me@example.com>\r\nAlso Me <me@example.com> <older@example.com>\r\n")
	fx.Git("config", "mailmap.file", "mine.txt")
	fx.Git("config", "mailmap.blob", "HEAD:.mailmap")
	got, err = r.MailmapSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("sources = %+v", got)
	}
	want := []MailmapSource{
		{Source: MailmapWorkTree, Path: filepath.Join(fx.Dir, ".mailmap"), Exists: true, Editable: true, Lines: 1},
		{Source: MailmapBlob, Path: "HEAD:.mailmap", Exists: true, Lines: 1},
		{Source: MailmapFile, Path: filepath.Join(fx.Dir, "mine.txt"), Exists: true, Lines: 2},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("source %d = %+v\nwant       %+v", i, got[i], want[i])
		}
	}
	if text, _ := r.ReadMailmapSource(ctx, MailmapFile); text != "Me <me@example.com> <old-me@example.com>\nAlso Me <me@example.com> <older@example.com>\n" {
		t.Errorf("mailmap.file read as %q", text)
	}

	// A setting that points at nothing is listed, and reads as empty.
	fx.Git("config", "mailmap.blob", "HEAD:no-such-file")
	got, err = r.MailmapSources(ctx)
	if err != nil || len(got) != 3 || got[1].Exists || got[1].Lines != 0 {
		t.Fatalf("a blob that isn't there: %+v, %v", got, err)
	}
}

func TestWriteMailmapNeverCreatesAFileUnasked(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	p := filepath.Join(fx.Dir, ".mailmap")
	exists := func() bool {
		_, err := os.Stat(p)
		return err == nil
	}
	line := "Jane Doe <jdoe@old.example>\n"

	// No file, and nobody agreed to one: nothing is written.
	if err := r.WriteMailmap(ctx, "", line, false); !errors.Is(err, ErrNoMailmap) {
		t.Fatalf("without permission, err = %v, want ErrNoMailmap", err)
	}
	if exists() {
		t.Fatal("a .mailmap was created without permission")
	}
	// Saving nothing where there's nothing leaves nothing, with or without permission.
	for _, create := range []bool{false, true} {
		if err := r.WriteMailmap(ctx, "", "", create); err != nil || exists() {
			t.Fatalf("saving no text (create %v): err %v, file %v", create, err, exists())
		}
	}
	// With permission the file is made.
	if err := r.WriteMailmap(ctx, "", line, true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(p); string(data) != line {
		t.Fatalf("the new file holds %q", data)
	}
	// Once it exists, a save needs no more permission.
	more := line + "Sam Ortiz <sam@example.com>\n"
	if err := r.WriteMailmap(ctx, line, more, false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(p); string(data) != more {
		t.Fatalf("after a second save the file holds %q", data)
	}
}

func TestWriteMailmapKeepsLineEndingsAndRefusesAStaleBase(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	p := filepath.Join(fx.Dir, ".mailmap")
	if err := os.WriteFile(p, []byte("# people\r\nJane Doe <jdoe@old.example>\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base, err := r.ReadMailmapSource(ctx, MailmapWorkTree)
	if err != nil || base != "# people\nJane Doe <jdoe@old.example>\n" {
		t.Fatalf("read %q, %v", base, err)
	}

	if err := r.WriteMailmap(ctx, base, base+"Sam Ortiz <sam@example.com>\n", false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(p); string(data) != "# people\r\nJane Doe <jdoe@old.example>\r\nSam Ortiz <sam@example.com>\r\n" {
		t.Fatalf("the file's line endings changed: %q", data)
	}

	// Someone else's edit since the text was read: nothing is written over it.
	if err := os.WriteFile(p, []byte("# changed elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteMailmap(ctx, base, "mine\n", false); !errors.Is(err, ErrMailmapChanged) {
		t.Fatalf("with a stale base, err = %v, want ErrMailmapChanged", err)
	}
	if data, _ := os.ReadFile(p); string(data) != "# changed elsewhere\n" {
		t.Fatalf("the other edit was overwritten: %q", data)
	}

	// Emptying it leaves an empty file: deleting a file the repository tracks isn't a save's job.
	if err := r.WriteMailmap(ctx, "# changed elsewhere\n", "", false); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(p); err != nil || len(data) != 0 {
		t.Fatalf("after emptying: %q, %v", data, err)
	}
}
