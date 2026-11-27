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

func TestPatchFileKind(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, text, want string }{
		{"format-patch", "From 3fbfa29 Mon Sep 17 00:00:00 2001\nFrom: A <a@x>\nSubject: [PATCH] x\n\n---\ndiff --git a/x b/x\n", PatchMail},
		{"a mail saved without its mbox line", "Return-Path: <a@x>\nFrom: A <a@x>\nSubject: [PATCH] x\nDate: now\n\nbody\n", PatchMail},
		{"git diff", "diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n", PatchDiff},
		{"diff -u", "--- x.orig\n+++ x\n@@ -1 +1 @@\n-a\n+b\n", PatchDiff},
		{"neither", "Dear diary,\n\nToday I wrote some code.\n", ""},
		{"an empty file", "", ""},
	} {
		if got := PatchFileKind([]byte(c.text)); got != c.want {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSubjectTags(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		subject string
		n, m, v int
	}{
		{"[PATCH] Fix it", 0, 0, 1},
		{"[PATCH v2 3/5] Fix it", 3, 5, 2},
		{"[RFC PATCH net-next v3 0/4] Cover", 0, 4, 3},
		{"Re: [PATCH 2/3] Fix it", 2, 3, 1},
		{"Fix it with no brackets", 0, 0, 1},
	} {
		n, m, v := subjectTags(c.subject)
		if n != c.n || m != c.m || v != c.v {
			t.Errorf("%q = %d/%d v%d, want %d/%d v%d", c.subject, n, m, v, c.n, c.m, c.v)
		}
	}
}

func TestReadMailboxOfAThread(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "base")
	fx.CommitFile("a.txt", "2\n", "Two")
	fx.CommitFile("b.txt", "b\n", "Add b")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	dir := t.TempDir()
	v1 := fx.Git("format-patch", "--stdout", "-1", "HEAD~1")
	v2 := fx.Git("format-patch", "--stdout", "-v2", "--cover-letter", "-2", "HEAD")
	reply := "From nobody Mon Sep 17 00:00:00 2001\nFrom: Reviewer <r@example.com>\nSubject: Re: [PATCH v2 1/2] Two\n\nLooks good.\n"
	thread := filepath.Join(dir, "thread.mbox")
	if err := os.WriteFile(thread, []byte(v1+"\n"+v2+"\n"+reply), 0o644); err != nil {
		t.Fatal(err)
	}
	split := filepath.Join(dir, "split")
	if err := os.Mkdir(split, 0o755); err != nil {
		t.Fatal(err)
	}

	msgs, err := repo.ReadMailbox(ctx, thread, split)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		subject     string
		n, m, v     int
		cover, diff bool
	}
	var got []row
	for _, m := range msgs {
		got = append(got, row{m.Subject, m.Number, m.Total, m.Version, m.Cover, m.HasPatch})
	}
	want := []row{
		{"Two", 0, 0, 1, false, true},
		{"*** SUBJECT HERE ***", 0, 2, 2, true, false},
		{"Two", 1, 2, 2, false, true},
		{"Add b", 2, 2, 2, false, true},
		{"Two", 1, 2, 2, false, false},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("messages =\n%+v\nwant\n%+v", got, want)
	}
	if f := msgs[3].Files; len(f) != 1 || f[0] != (NumStat{Path: "b.txt", Added: 1}) {
		t.Fatalf("Add b's files = %+v", f)
	}
	if msgs[4].Author != "Reviewer" || msgs[0].Email == "" {
		t.Fatalf("who = %+v, %+v", msgs[4], msgs[0])
	}
}

// trialRepo makes a side branch of three patches against main, where main has moved on: the
// first now only merges, the second is in main already, the third conflicts.
func trialRepo(t *testing.T) (fx *gittest.Repo, patches []string, picked string) {
	t.Helper()
	fx = gittest.New(t)
	lines := func(changes map[int]string) string {
		var b strings.Builder
		for i := 1; i <= 12; i++ {
			if s, ok := changes[i]; ok {
				b.WriteString(s + "\n")
			} else {
				b.WriteString(strings.Repeat(string(rune('a'+i-1)), 3) + "\n")
			}
		}
		return b.String()
	}
	fx.CommitFile("a.txt", lines(nil), "base")
	fx.Git("checkout", "-q", "-b", "side")
	fx.CommitFile("a.txt", lines(map[int]string{2: "two"}), "Change line 2")
	fx.CommitFile("a.txt", lines(map[int]string{2: "two", 10: "ten"}), "Change line 10")
	fx.CommitFile("a.txt", lines(map[int]string{2: "two", 10: "ten", 6: "six"}), "Change line 6")
	dir := t.TempDir()
	for _, name := range strings.Split(fx.Git("format-patch", "-o", dir, "-3", "HEAD"), "\n") {
		patches = append(patches, strings.TrimSpace(name))
	}
	picked = fx.Git("rev-parse", "HEAD~1")
	fx.Git("checkout", "-q", "main")
	// Main changes line 5, in the first patch's context but apart from its change; takes the
	// second as it is; and changes line 6, which the third changes too.
	fx.CommitFile("a.txt", lines(map[int]string{5: "five"}), "Main changes line 5")
	fx.Git("cherry-pick", picked)
	fx.CommitFile("a.txt", lines(map[int]string{5: "five", 10: "ten", 6: "SIX"}), "Main changes line 6")
	return fx, patches, picked
}

func TestPatchTrialSaysHowEachWouldGo(t *testing.T) {
	t.Parallel()
	fx, patches, picked := trialRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	trial, err := repo.StartTrial(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var statuses []string
	var last TryResult
	for _, p := range patches {
		last = trial.Try(ctx, p)
		statuses = append(statuses, last.Status)
	}
	if want := []string{TryMerges, TryApplied, TryConflicts}; !slices.Equal(statuses, want) {
		t.Fatalf("statuses = %v, want %v", statuses, want)
	}
	if !slices.Equal(last.Files, []string{"a.txt"}) {
		t.Fatalf("the conflict's files = %v", last.Files)
	}
	if status := fx.Git("status", "--porcelain"); status != "" {
		t.Fatalf("the trial touched the real index or files:\n%s", status)
	}

	// The one that's in already matches the commit it came in as.
	ids, err := repo.PatchCommitIDs(ctx, "HEAD", 50)
	if err != nil {
		t.Fatal(err)
	}
	id, err := repo.PatchID(ctx, patches[1])
	if err != nil || ids[id] == "" || ids[id] == picked {
		t.Fatalf("patch id %q → %q, want main's copy of it, not the side's %s", id, ids[id], picked)
	}
	if subject := fx.Git("log", "-1", "--format=%s", ids[id]); subject != "Change line 10" {
		t.Fatalf("matched %q", subject)
	}

	// A patch for a file this repository has never had fails, with git's reason.
	bad := filepath.Join(t.TempDir(), "bad.patch")
	if err := os.WriteFile(bad, []byte("diff --git a/nowhere.txt b/nowhere.txt\nindex 1234567..89abcde 100644\n--- a/nowhere.txt\n+++ b/nowhere.txt\n@@ -1 +1 @@\n-x\n+y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh, err := repo.StartTrial(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := fresh.Try(ctx, bad); got.Status != TryFails || got.Error == "" {
		t.Fatalf("a patch for nothing = %+v", got)
	}
}

func TestAmAppliesAsCommitsAndStopsOnAConflict(t *testing.T) {
	t.Parallel()
	fx, patches, _ := trialRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	mbox := filepath.Join(t.TempDir(), "first.mbox")
	text, _ := os.ReadFile(patches[0])
	if err := os.WriteFile(mbox, text, 0o644); err != nil {
		t.Fatal(err)
	}
	if repo.HasStaged(ctx) {
		t.Fatal("nothing is staged yet")
	}
	if err := repo.Am(ctx, mbox, true); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("log", "-1", "--format=%s%n%an%n%b"); !strings.Contains(got, "Change line 2") || !strings.Contains(got, "Signed-off-by:") {
		t.Fatalf("the commit:\n%s", got)
	}
	third, _ := os.ReadFile(patches[2])
	if err := os.WriteFile(mbox, third, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.Am(ctx, mbox, false); !errors.Is(err, ErrConflicts) {
		t.Fatalf("err = %v, want ErrConflicts", err)
	}
	fx.WriteFile("a.txt", "staged\n")
	fx.Git("add", "a.txt")
	if !repo.HasStaged(ctx) {
		t.Fatal("a staged change should count")
	}
}
