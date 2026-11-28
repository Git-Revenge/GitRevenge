package git

import (
	"context"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// #1078: the blame view's track. A file's versions, oldest first, each with what it changed;
// a binary version marked; nothing from other files; and the rename where its name began.
func TestVersions(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	body := strings.Repeat("send an alert when the gust is over the limit\n", 8)
	fx.CommitFile("other.txt", "other\n", "unrelated")
	v1 := fx.CommitFile("lib/sms.js", body, "add sms")
	fx.Git("mv", "lib/sms.js", "lib/texts.js")
	v2 := fx.Commit("call it texts")
	v3 := fx.CommitFile("lib/texts.js", body+"and log it\nand count it\n", "log it")
	fx.CommitFile("other.txt", "other, changed\n", "unrelated again")
	v4 := fx.CommitFile("lib/texts.js", "\x00\x01binary now\n", "make it binary")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	got, err := repo.Versions(ctx, "lib/texts.js", "")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, v := range got {
		ids = append(ids, v.ID)
	}
	// No --follow: the track begins where the file got its name.
	if strings.Join(ids, " ") != strings.Join([]string{v2, v3, v4}, " ") {
		t.Fatalf("versions = %v, want %v %v %v", ids, v2[:7], v3[:7], v4[:7])
	}
	if got[0].Added != 8 || got[0].Removed != 0 || got[0].Subject != "call it texts" || got[0].Author == "" || got[0].Time == 0 {
		t.Errorf("the oldest: %+v", got[0])
	}
	if got[1].Added != 2 || got[1].Removed != 0 {
		t.Errorf("log it: %+v", got[1])
	}
	if !got[2].Binary || got[2].Added != 0 {
		t.Errorf("the binary version: %+v", got[2])
	}
	// The oldest renamed it; the track says from what.
	if old, err := repo.RenamedAt(ctx, got[0].ID, "lib/texts.js"); err != nil || old != "lib/sms.js" {
		t.Errorf("renamed from %q, %v", old, err)
	}
	if old, _ := repo.RenamedAt(ctx, v1, "lib/sms.js"); old != "" {
		t.Errorf("the first version renamed nothing: %q", old)
	}
	// As far as a commit: its history only.
	if older, _ := repo.Versions(ctx, "lib/texts.js", v3); len(older) != 2 || older[1].ID != v3 {
		t.Errorf("versions at log it: %+v", older)
	}
	if none, err := repo.Versions(ctx, "nowhere.js", ""); err != nil || len(none) != 0 {
		t.Errorf("a file never there: %v, %v", none, err)
	}
}

func TestUncommitted(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "first")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	if dirty, err := repo.Uncommitted(ctx, "a.txt"); err != nil || dirty {
		t.Errorf("clean: %v, %v", dirty, err)
	}
	fx.WriteFile("a.txt", "a, changed\n")
	if dirty, _ := repo.Uncommitted(ctx, "a.txt"); !dirty {
		t.Error("a change in the work tree wasn't seen")
	}
	// Staged counts too: it isn't committed.
	fx.Git("add", "a.txt")
	if dirty, _ := repo.Uncommitted(ctx, "a.txt"); !dirty {
		t.Error("a staged change wasn't seen")
	}
}

// #1079: blaming only some lines of a version, for the track to work out the rest from a
// neighbour; and a merge marked, whose new lines may come from its other parent.
func TestBlameRangesAndMerges(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	first := fx.CommitFile("a.txt", "one\ntwo\nthree\nfour\n", "first")
	second := fx.CommitFile("a.txt", "one\nTWO\nthree\nFOUR\n", "second")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	got, err := repo.BlameRanges(ctx, "a.txt", second, []BlameRange{{From: 1, To: 2}, {From: 4, To: 4}})
	if err != nil {
		t.Fatal(err)
	}
	var says []string
	for _, l := range got {
		says = append(says, l.Commit[:7]+" "+l.Line)
	}
	want := []string{first[:7] + " one", second[:7] + " TWO", second[:7] + " FOUR"}
	if strings.Join(says, ", ") != strings.Join(want, ", ") {
		t.Errorf("ranges = %v, want %v", says, want)
	}
	if all, _ := repo.BlameRanges(ctx, "a.txt", second, nil); len(all) != 4 {
		t.Errorf("no ranges: %d lines, want every one", len(all))
	}
	if _, err := repo.BlameRanges(ctx, "a.txt", second, []BlameRange{{From: 3, To: 2}}); err == nil {
		t.Error("took a range that ends before it starts")
	}

	// Both sides change the file, so the merge is a version of its own.
	fx.Git("checkout", "-q", "-b", "side")
	fx.CommitFile("a.txt", "one\nTWO\nthree\nFOUR\nfrom the side\n", "side")
	fx.Git("checkout", "-q", "main")
	fx.CommitFile("a.txt", "from main\none\nTWO\nthree\nFOUR\n", "main")
	fx.Git("merge", "-q", "--no-ff", "-m", "merge side", "side")
	versions, err := repo.Versions(ctx, "a.txt", "")
	if err != nil {
		t.Fatal(err)
	}
	merges := map[string]bool{}
	for _, v := range versions {
		merges[v.Subject] = v.Merge
	}
	if !merges["merge side"] || merges["side"] || merges["main"] || merges["second"] {
		t.Errorf("merges = %v, want only merge side", merges)
	}
}
