package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// twelve is twelve lines with the changes given.
func twelve(changes map[int]string) string {
	var b strings.Builder
	for i := 1; i <= 12; i++ {
		if s, ok := changes[i]; ok {
			b.WriteString(s + "\n")
		} else {
			b.WriteString(strings.Repeat("y", i) + "\n")
		}
	}
	return b.String()
}

// diffFixture is a repository and two diffs, chosen as if picked in the file dialog: diff
// changes a.txt's line 2 and adds n.txt; clash changes line 6, which main has changed another
// way since. plain is diff's a.txt change as diff -u writes it, naming no versions.
func diffFixture(t *testing.T, a *App) (fx *gittest.Repo, path, diff, clash, plain string) {
	t.Helper()
	fx = gittest.New(t)
	fx.CommitFile("a.txt", twelve(nil), "base")
	fx.Git("checkout", "-q", "-b", "side")
	fx.WriteFile("a.txt", twelve(map[int]string{2: "two"}))
	fx.WriteFile("n.txt", "new\n")
	fx.Git("add", "-A")
	fx.Commit("side")
	dir := t.TempDir()
	write := func(name, text string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		a.chosen.add(p)
		return p
	}
	diff = write("side.diff", fx.Git("diff", "main", "side")+"\n")
	fx.Git("checkout", "-q", "-b", "other", "main")
	fx.CommitFile("a.txt", twelve(map[int]string{6: "SIX"}), "other six")
	clash = write("clash.diff", fx.Git("diff", "main", "other")+"\n")
	plain = write("plain.diff", "--- a/a.txt\n+++ b/a.txt\n@@ -5,3 +5,3 @@\n "+strings.Repeat("y", 5)+"\n-"+strings.Repeat("y", 6)+"\n+SIX\n "+strings.Repeat("y", 7)+"\n")
	fx.Git("checkout", "-q", "main")
	fx.CommitFile("a.txt", twelve(map[int]string{6: "six"}), "main six")
	return fx, openForMap(t, a, fx), diff, clash, plain
}

func TestReadPatchesReadsAPlainDiff(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	_, path, diff, _, plain := diffFixture(t, a)
	f, err := a.ReadPatches(path, diff)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(f.Token)
	if f.Kind != "diff" || f.Token == "" || f.Diff == nil || !f.Diff.Named || len(f.Diff.Unclean) != 0 || f.Branch != "main" {
		t.Fatalf("file = %+v, diff %+v", f, f.Diff)
	}
	var paths []string
	for _, x := range f.Diff.Files {
		paths = append(paths, x.Path)
		if x.Path == "n.txt" && !x.Created {
			t.Error("n.txt is new")
		}
	}
	slices.Sort(paths)
	if !slices.Equal(paths, []string{"a.txt", "n.txt"}) {
		t.Errorf("files = %v", paths)
	}
	p, err := a.ReadPatches(path, plain)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(p.Token)
	if p.Diff == nil || p.Diff.Named {
		t.Errorf("a diff -u names no versions: %+v", p.Diff)
	}
}

func TestTryDiffSaysHowEachFileGoes(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path, diff, clash, plain := diffFixture(t, a)
	try := func(file string, reverse bool) map[string]string {
		t.Helper()
		f, err := a.ReadPatches(path, file)
		if err != nil {
			t.Fatal(err)
		}
		defer a.ClosePatches(f.Token)
		rows, err := a.TryDiff(path, f.Token, reverse)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, r := range rows {
			out[r.Path] = r.Status
		}
		return out
	}
	if got := try(diff, false); got["a.txt"] != "applies" || got["n.txt"] != "applies" {
		t.Errorf("diff: %v", got)
	}
	if got := try(diff, true); got["a.txt"] != "applied" || got["n.txt"] != "applied" {
		t.Errorf("in reverse, nothing of it is here to take out: %v", got)
	}
	if got := try(clash, false); got["a.txt"] != "conflicts" {
		t.Errorf("clash: %v", got)
	}
	if got := try(plain, false); got["a.txt"] != "fails" {
		t.Errorf("a diff -u that doesn't fit can't be merged: %v", got)
	}
	if fx.Git("status", "--porcelain") != "" {
		t.Error("trying must leave the files alone")
	}
}

func TestApplyDiffAndUndoIt(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path, diff, _, _ := diffFixture(t, a)
	f, err := a.ReadPatches(path, diff)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(f.Token)
	done, err := a.ApplyDiff(path, f.Token, []string{"a.txt", "n.txt"}, "files", false)
	if err != nil {
		t.Fatal(err)
	}
	if done.Files != 2 || done.ID == "" || len(done.Conflicts) != 0 {
		t.Fatalf("applied = %+v", done)
	}
	if got := fx.Git("status", "--porcelain", "--untracked-files=all"); got != "M a.txt\n?? n.txt" {
		t.Errorf("status = %q", got)
	}
	if err := a.UndoDiff(path, done.ID); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("status", "--porcelain", "--untracked-files=all"); got != "" {
		t.Errorf("after undo, status = %q", got)
	}
	if err := a.UndoDiff(path, done.ID); err == nil {
		t.Error("an undo is used up once it's done")
	}
}

func TestApplyDiffWaitsForYourChanges(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path, diff, _, _ := diffFixture(t, a)
	f, err := a.ReadPatches(path, diff)
	if err != nil {
		t.Fatal(err)
	}
	defer a.ClosePatches(f.Token)
	fx.WriteFile("a.txt", "mine\n")
	_, err = a.ApplyDiff(path, f.Token, []string{"a.txt", "n.txt"}, "files", false)
	if err == nil || !strings.Contains(err.Error(), "a.txt has changes of yours") {
		t.Fatalf("err = %v", err)
	}
	// n.txt alone is clean: it can go in.
	if _, err := a.ApplyDiff(path, f.Token, []string{"n.txt"}, "files", false); err != nil {
		t.Fatal(err)
	}
}

func TestUndoDiffRefusesOnceItsFilesChange(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path, diff, _, _ := diffFixture(t, a)
	f, _ := a.ReadPatches(path, diff)
	defer a.ClosePatches(f.Token)
	done, err := a.ApplyDiff(path, f.Token, []string{"a.txt", "n.txt"}, "index", false)
	if err != nil {
		t.Fatal(err)
	}
	fx.WriteFile("n.txt", "edited since\n")
	if err := a.UndoDiff(path, done.ID); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(fx.Path("n.txt")); string(got) != "edited since\n" {
		t.Error("a refused undo leaves the files be")
	}
}

func TestApplyDiffWithAConflictPutsBackOnUndo(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path, _, clash, _ := diffFixture(t, a)
	f, _ := a.ReadPatches(path, clash)
	defer a.ClosePatches(f.Token)
	done, err := a.ApplyDiff(path, f.Token, []string{"a.txt"}, "files", false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(done.Conflicts, []string{"a.txt"}) || done.ID == "" {
		t.Fatalf("applied = %+v", done)
	}
	if err := a.UndoDiff(path, done.ID); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("status", "--porcelain"); got != "" {
		t.Errorf("status = %q", got)
	}
}

func TestStagedOnlyRefusesAConflict(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path, _, clash, _ := diffFixture(t, a)
	f, _ := a.ReadPatches(path, clash)
	defer a.ClosePatches(f.Token)
	_, err := a.ApplyDiff(path, f.Token, []string{"a.txt"}, "cached", false)
	if err == nil || !strings.Contains(err.Error(), "staging only can't hold") {
		t.Fatalf("err = %v", err)
	}
	if got := fx.Git("status", "--porcelain"); got != "" {
		t.Errorf("status = %q, want it put back", got)
	}
}

func TestApplyDiffInReverse(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx, path, diff, _, _ := diffFixture(t, a)
	f, _ := a.ReadPatches(path, diff)
	defer a.ClosePatches(f.Token)
	if _, err := a.ApplyDiff(path, f.Token, []string{"a.txt", "n.txt"}, "index", false); err != nil {
		t.Fatal(err)
	}
	fx.Commit("took the diff")
	done, err := a.ApplyDiff(path, f.Token, []string{"a.txt", "n.txt"}, "index", true)
	if err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("diff", "--cached", "--name-only", "HEAD~1"); got != "" || done.Files != 2 {
		t.Errorf("against before the diff: %q, %+v", got, done)
	}
}
