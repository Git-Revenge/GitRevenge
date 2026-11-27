package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// patchRepo makes main with a base commit and a side branch of a commit, a merge of main into
// it, and two more: a series with one merge to leave out.
func patchRepo(t *testing.T) (fx *gittest.Repo, base, tip string, ids []string) {
	t.Helper()
	fx = gittest.New(t)
	base = fx.CommitFile("a.txt", "one\n", "base")
	fx.Git("checkout", "-q", "-b", "side")
	ids = append(ids, fx.CommitFile("b.txt", "b\n", "Add b"))
	fx.Git("checkout", "-q", "main")
	fx.CommitFile("m.txt", "m\n", "main moves on")
	fx.Git("checkout", "-q", "side")
	fx.Git("merge", "-q", "--no-edit", "main")
	ids = append(ids, fx.CommitFile("b.txt", "b2\n", "Change b"))
	ids = append(ids, fx.CommitFile("c.txt", "c\n", "Add c"))
	return fx, base, fx.Head(), ids
}

func TestPatchList(t *testing.T) {
	t.Parallel()
	fx, base, tip, ids := patchRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	subjects := func(cs []PatchCommit) (out []string) {
		for _, c := range cs {
			out = append(out, c.Subject)
		}
		return out
	}

	got, merges, err := repo.PatchList(ctx, PatchRevs{Since: base, Tip: tip})
	if err != nil {
		t.Fatal(err)
	}
	// The range holds main's commit too, through the merge.
	if want := []string{"Add b", "main moves on", "Change b", "Add c"}; !slices.Equal(subjects(got), want) || merges != 1 {
		t.Fatalf("range = %v, %d merges; want %v and 1", subjects(got), merges, want)
	}
	if got[0].First != base || got[0].ID != ids[0] {
		t.Fatalf("the first = %+v", got[0])
	}
	last, merges, err := repo.PatchList(ctx, PatchRevs{Tip: tip, Last: 2})
	if err != nil || !slices.Equal(subjects(last), []string{"Change b", "Add c"}) || merges != 0 {
		t.Fatalf("the last two = %v, %d, %v", subjects(last), merges, err)
	}
	named, _, err := repo.PatchList(ctx, PatchRevs{IDs: []string{ids[2], ids[0]}})
	if err != nil || !slices.Equal(subjects(named), []string{"Add c", "Add b"}) {
		t.Fatalf("named = %v, %v; want the order given", subjects(named), err)
	}
	if _, _, err := repo.PatchList(ctx, PatchRevs{Since: "--all", Tip: tip}); err == nil {
		t.Fatal("an option for a commit should be refused")
	}
}

func TestFormatPatchesWithEveryOption(t *testing.T) {
	t.Parallel()
	fx, _, tip, _ := patchRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	dir := t.TempDir()
	opts := PatchOptions{Prefix: "RFC PATCH", Version: 2, Numbered: true, Cover: true, CoverSubject: "Bees and cees", CoverText: "Two files that go together.", Signoff: true, Base: "HEAD~2"}

	names, err := repo.FormatPatches(context.Background(), PatchRevs{Tip: tip, Last: 2}, 2, opts, dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"v2-0000-cover-letter.patch", "v2-0001-Change-b.patch", "v2-0002-Add-c.patch"}; !slices.Equal(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	cover, _ := os.ReadFile(filepath.Join(dir, names[0]))
	for _, want := range []string{"Subject: [RFC PATCH v2 0/2] Bees and cees", "Two files that go together.", "base-commit: "} {
		if !strings.Contains(string(cover), want) {
			t.Fatalf("the cover letter lacks %q:\n%s", want, cover)
		}
	}
	if strings.Contains(string(cover), "*** SUBJECT HERE ***") || strings.Contains(string(cover), "*** BLURB HERE ***") {
		t.Fatalf("a placeholder was left:\n%s", cover)
	}
	first, _ := os.ReadFile(filepath.Join(dir, names[1]))
	if !strings.Contains(string(first), "Subject: [RFC PATCH v2 1/2] Change b") || !strings.Contains(string(first), "Signed-off-by: ") {
		t.Fatalf("the first patch:\n%s", first)
	}
}

func TestFormatPatchTextAppliesWithAm(t *testing.T) {
	t.Parallel()
	fx, base, tip, _ := patchRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	one, err := repo.FormatPatchText(ctx, PatchRevs{Tip: tip, Last: 1}, 1, PatchOptions{Numbered: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(one, "Subject: [PATCH] Add c") {
		t.Fatalf("one patch, unnumbered as git does:\n%s", one)
	}
	text, err := repo.FormatPatchText(ctx, PatchRevs{Since: base, Tip: tip}, 4, PatchOptions{Numbered: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(text, "\nSubject: [PATCH ") != 4 || !strings.Contains(text, "[PATCH 4/4] Add c") {
		t.Fatalf("four numbered patches, the merge left out:\n%s", text)
	}
	// Applied in a fresh clone at base, they make the same files as the tip.
	mbox := filepath.Join(t.TempDir(), "side.mbox")
	if err := os.WriteFile(mbox, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other")
	fx.Git("clone", "-q", "--no-checkout", fx.Dir, other)
	git := func(args ...string) string {
		t.Helper()
		out, err := repo.ReadString(ctx, append([]string{"-C", other}, args...)...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	git("checkout", "-q", "-b", "applied", base)
	git("-c", "user.name=T", "-c", "user.email=t@example.com", "am", "-q", mbox)
	if got, want := git("rev-parse", "HEAD^{tree}"), fx.Git("rev-parse", tip+"^{tree}"); got != want {
		t.Fatalf("applied tree %s, want the tip's %s", got, want)
	}
}

func TestPatchRevsRefuseTooMany(t *testing.T) {
	t.Parallel()
	ids := make([]string, maxPatchIDs+1)
	for i := range ids {
		ids[i] = strings.Repeat("a", 40)
	}
	if _, err := (PatchRevs{IDs: ids}).revs(); !errors.Is(err, ErrTooManyPatches) {
		t.Fatalf("err = %v, want ErrTooManyPatches", err)
	}
	if needs, err := OpenRepo(newTestRunner(t), gittest.New(t).Dir).PatchNeeds(context.Background(), PatchRevs{Tip: "HEAD"}); err != nil || needs != nil {
		t.Fatalf("a whole clone needs nothing: %v, %v", needs, err)
	}
}
