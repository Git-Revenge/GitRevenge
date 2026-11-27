package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// seriesRepo makes main and a side branch of three patches saved to a mailbox, then moves
// main on: the first patch only merges now, the second is in main already, the third
// conflicts.
func seriesRepo(t *testing.T) (fx *gittest.Repo, mbox string) {
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
	mbox = filepath.Join(t.TempDir(), "side.mbox")
	if err := os.WriteFile(mbox, []byte(fx.Git("format-patch", "--stdout", "-3", "HEAD")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	picked := fx.Git("rev-parse", "HEAD~1")
	fx.Git("checkout", "-q", "main")
	fx.CommitFile("a.txt", lines(map[int]string{5: "five"}), "Main changes line 5")
	fx.Git("cherry-pick", picked)
	fx.CommitFile("a.txt", lines(map[int]string{5: "five", 10: "ten", 6: "SIX"}), "Main changes line 6")
	return fx, mbox
}

func TestReadPatchesOnlyAChosenFile(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, mbox := seriesRepo(t)
	path := openForMap(t, a, fx)

	if _, err := a.ReadPatches(path, mbox); err == nil {
		t.Fatal("a file nobody chose should be refused")
	}
	a.chosen.add(mbox)
	got, err := a.ReadPatches(path, mbox)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(got.Token)
	if got.Kind != git.PatchMail || got.Token == "" || len(got.Messages) != 3 || got.Branch != "main" || got.Staged {
		t.Fatalf("read = %+v", got)
	}
	if got.Messages[1].Subject != "Change line 10" || got.Messages[1].Number != 2 || got.Messages[1].Total != 3 {
		t.Fatalf("the second = %+v", got.Messages[1])
	}

	diff := filepath.Join(t.TempDir(), "change.diff")
	if err := os.WriteFile(diff, []byte("diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-aaa\n+AAA\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.chosen.add(diff)
	// A plain diff opens the diff's preview (#1024).
	d, err := a.ReadPatches(path, diff)
	if err != nil || d.Kind != git.PatchDiff || d.Token == "" || d.Diff == nil || len(d.Diff.Files) != 1 || d.Diff.Files[0].Path != "a.txt" {
		t.Fatalf("a plain diff = %+v, %v", d, err)
	}
	a.ClosePatches(d.Token)
}

func TestTryPatchesTellsEachRowAsItComes(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	var mu sync.Mutex
	var sent []PatchTry
	a.emitHook = func(name string, data any) {
		if name == "patches:tried" {
			mu.Lock()
			sent = append(sent, data.(PatchTry))
			mu.Unlock()
		}
	}
	fx, mbox := seriesRepo(t)
	path := openForMap(t, a, fx)
	a.chosen.add(mbox)
	file, err := a.ReadPatches(path, mbox)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(file.Token)

	rows, err := a.TryPatches(path, file.Token, []int{0, 1, 2}, 7)
	if err != nil {
		t.Fatal(err)
	}
	var statuses []string
	for _, r := range rows {
		statuses = append(statuses, r.Status)
	}
	if want := []string{git.TryMerges, git.TryApplied, git.TryConflicts}; !slices.Equal(statuses, want) {
		t.Fatalf("statuses = %v, want %v", statuses, want)
	}
	if rows[1].Already == "" || fx.Git("log", "-1", "--format=%s", rows[1].Already) != "Change line 10" {
		t.Fatalf("the one in already = %+v", rows[1])
	}
	// A copy, so the lock isn't held while the next try sends through the hook that takes it.
	mu.Lock()
	got := slices.Clone(sent)
	mu.Unlock()
	if len(got) != 3 || got[0].Run != 7 || got[2].Index != 2 {
		t.Fatalf("sent = %+v", got)
	}

	// Leaving out the first, the conflict comes first and the one after it waits.
	again, err := a.TryPatches(path, file.Token, []int{2, 0}, 8)
	if err != nil || len(again) != 2 || again[0].Status != git.TryConflicts || !again[1].Waits {
		t.Fatalf("again = %+v, %v", again, err)
	}
}

func TestApplyPatchesOntoANewBranch(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, mbox := seriesRepo(t)
	path := openForMap(t, a, fx)
	a.chosen.add(mbox)
	file, err := a.ReadPatches(path, mbox)
	if err != nil {
		t.Fatal(err)
	}

	if err := a.ApplyPatches(path, file.Token, []int{0}, "wind/knots", true); err != nil {
		t.Fatal(err)
	}
	if branch := fx.Git("branch", "--show-current"); branch != "wind/knots" {
		t.Fatalf("on %q, want the new branch", branch)
	}
	got := fx.Git("log", "-1", "--format=%s%n%b")
	if !strings.HasPrefix(got, "Change line 2") || !strings.Contains(got, "Signed-off-by:") {
		t.Fatalf("the commit:\n%s", got)
	}

	fx.WriteFile("a.txt", "staged\n")
	fx.Git("add", "a.txt")
	if err := a.ApplyPatches(path, file.Token, []int{1}, "", false); err == nil || !strings.Contains(err.Error(), "staged") {
		t.Fatalf("with something staged = %v", err)
	}

	dir := a.patches.get(file.Token).dir
	a.ClosePatches(file.Token)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("closing should take the split messages away: %v", err)
	}
	if _, err := a.TryPatches(path, file.Token, []int{0}, 1); err == nil {
		t.Fatal("a closed file of patches should say so")
	}
}

func TestAbortingPatchesTakesTheBranchMadeForThem(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, mbox := seriesRepo(t)
	path := openForMap(t, a, fx)
	a.chosen.add(mbox)
	file, err := a.ReadPatches(path, mbox)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(file.Token)

	// The third conflicts: git am stops on the branch made for it.
	if err := a.ApplyPatches(path, file.Token, []int{2}, "wind/knots", false); err == nil {
		t.Fatal("the third patch should stop on its conflict")
	}
	st, err := a.RepoState(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Operation != "am" || st.Ours != "wind/knots" || st.MadeBranch != "wind/knots" || st.Subject != "Change line 6" || st.Stuck || !slices.Contains(st.Actions, "continue") {
		t.Fatalf("state = %+v", st)
	}
	if err := a.RunOperationAction(path, "abort"); err != nil {
		t.Fatal(err)
	}
	if branch := fx.Git("branch", "--show-current"); branch != "main" {
		t.Fatalf("back on %q, want main", branch)
	}
	if out := fx.Git("branch", "--list", "wind/knots"); out != "" {
		t.Fatalf("the branch made for the patches should be gone: %q", out)
	}
}

func TestAStuckPatchOffersNoContinue(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, _ := seriesRepo(t)
	path := openForMap(t, a, fx)
	bad := filepath.Join(t.TempDir(), "bad.mbox")
	if err := os.WriteFile(bad, []byte("From 0000000000000000000000000000000000000000 Mon Sep 17 00:00:00 2001\nFrom: A <a@example.com>\nSubject: [PATCH] Touch nowhere\nDate: Mon, 1 Jan 2026 00:00:00 +0000\n\n---\ndiff --git a/nowhere.txt b/nowhere.txt\nindex 1234567..89abcde 100644\n--- a/nowhere.txt\n+++ b/nowhere.txt\n@@ -1 +1 @@\n-x\n+y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.chosen.add(bad)
	file, err := a.ReadPatches(path, bad)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(file.Token)
	if err := a.ApplyPatches(path, file.Token, []int{0}, "", false); err == nil {
		t.Fatal("a patch for nothing should stop")
	}
	st, err := a.RepoState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Stuck || slices.Contains(st.Actions, "continue") || !slices.Contains(st.Actions, "skip") || st.MadeBranch != "" {
		t.Fatalf("state = %+v, want stuck with Skip and Abort only", st)
	}
	if patch, err := a.CurrentPatch(path); err != nil || !strings.Contains(patch, "nowhere.txt") {
		t.Fatalf("current patch = %q, %v", patch, err)
	}
	if err := a.RunOperationAction(path, "abort"); err != nil {
		t.Fatal(err)
	}
}

func TestAmStateCountsThePatchesApplied(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, mbox := seriesRepo(t)
	path := openForMap(t, a, fx)
	a.chosen.add(mbox)
	file, err := a.ReadPatches(path, mbox)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(file.Token)
	start := fx.Head()

	// The first applies and the third conflicts.
	if err := a.ApplyPatches(path, file.Token, []int{0, 2}, "", false); err == nil {
		t.Fatal("the third patch should stop on its conflict")
	}
	st, err := a.RepoState(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Operation != "am" || st.Step != 2 || st.Total != 2 || st.Applied != 1 || st.OrigHead != start || st.Ours != "main" {
		t.Fatalf("state = %+v, want patch 2 of 2 with 1 applied", st)
	}
	if err := a.RunOperationAction(path, "skip"); err != nil {
		t.Fatal(err)
	}
	if st, _ = a.RepoState(path); st.Operation != "" {
		t.Fatalf("skipping the last should finish: %+v", st)
	}
	if got := fx.Git("log", "-1", "--format=%s"); got != "Change line 2" {
		t.Fatalf("HEAD = %q, want the first patch", got)
	}
}
