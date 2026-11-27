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

// numbered is twelve lines, "1" to "12", with the changes given.
func numbered(changes map[int]string) string {
	var b strings.Builder
	for i := 1; i <= 12; i++ {
		if s, ok := changes[i]; ok {
			b.WriteString(s + "\n")
		} else {
			b.WriteString(strings.Repeat("x", i) + "\n")
		}
	}
	return b.String()
}

// diffRepo is a repository whose side branch changes a.txt's line 2, adds n.txt, deletes
// c.txt and renames b.txt. Its diff is at diff; main, checked out, has since changed line 6.
// clash is a diff of line 6 made another way, which only merges with a conflict.
func diffRepo(t *testing.T) (fx *gittest.Repo, diff, clash string) {
	t.Helper()
	fx = gittest.New(t)
	fx.WriteFile("a.txt", numbered(nil))
	fx.WriteFile("b.txt", "bee\n")
	fx.WriteFile("c.txt", "gone soon\n")
	fx.Git("add", ".")
	fx.Commit("base")
	fx.Git("checkout", "-q", "-b", "side")
	fx.WriteFile("a.txt", numbered(map[int]string{2: "two"}))
	fx.WriteFile("n.txt", "new\n")
	fx.Git("rm", "-q", "c.txt")
	fx.Git("mv", "b.txt", "b2.txt")
	fx.Git("add", "-A")
	fx.Commit("side")
	dir := t.TempDir()
	diff = filepath.Join(dir, "side.diff")
	if err := os.WriteFile(diff, []byte(fx.Git("diff", "main", "side")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fx.Git("checkout", "-q", "-b", "other", "main")
	fx.CommitFile("a.txt", numbered(map[int]string{6: "SIX"}), "other six")
	clash = filepath.Join(dir, "clash.diff")
	if err := os.WriteFile(clash, []byte(fx.Git("diff", "main", "other")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fx.Git("checkout", "-q", "main")
	fx.CommitFile("a.txt", numbered(map[int]string{6: "six"}), "main six")
	return fx, diff, clash
}

func TestReadDiffListsEachFileAndWhatHappensToIt(t *testing.T) {
	t.Parallel()
	fx, diff, _ := diffRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	files, err := repo.ReadDiff(context.Background(), diff)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]DiffFile{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	if a := byPath["a.txt"]; a.Added != 1 || a.Deleted != 1 || a.Created || a.Removed {
		t.Errorf("a.txt = %+v", a)
	}
	if n := byPath["n.txt"]; !n.Created || n.Added != 1 {
		t.Errorf("n.txt = %+v", n)
	}
	if c := byPath["c.txt"]; !c.Removed || c.Deleted != 1 {
		t.Errorf("c.txt = %+v", c)
	}
	if b := byPath["b2.txt"]; b.OldPath != "b.txt" || !slices.Equal(b.Paths(), []string{"b.txt", "b2.txt"}) {
		t.Errorf("b2.txt = %+v", b)
	}
	text, _ := os.ReadFile(diff)
	if !NamesVersions(text) {
		t.Error("git diff's output names its versions")
	}
	if NamesVersions([]byte("--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-x\n+y\n")) {
		t.Error("a diff -u doesn't")
	}
}

func TestRenameNames(t *testing.T) {
	for line, want := range map[string][2]string{
		"rename b.txt => b2.txt (100%)":         {"b.txt", "b2.txt"},
		"rename src/{a => b}/x.js (90%)":        {"src/a/x.js", "src/b/x.js"},
		"rename src/{ => lib}/x.js (100%)":      {"src/x.js", "src/lib/x.js"},
		"copy docs/{old.md => new.md} (75%)":    {"docs/old.md", "docs/new.md"},
		"rename with space.txt => w s 2 (100%)": {"with space.txt", "w s 2"},
	} {
		from, to, ok := renameNames(line)
		if !ok || from != want[0] || to != want[1] {
			t.Errorf("%q: %q => %q, %v", line, from, to, ok)
		}
	}
}

func TestTryDiffFileSaysHowEachFileGoes(t *testing.T) {
	t.Parallel()
	fx, diff, clash := diffRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	trial, err := repo.StartTrial(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Taking it back out: it isn't in, so its reverse doesn't apply but it does.
	if got := trial.TryDiffFile(ctx, diff, []string{"n.txt"}, true, true); got.Status != TryApplied {
		t.Errorf("reverse of a file that isn't in: %+v", got)
	}
	for _, p := range [][]string{{"a.txt"}, {"n.txt"}, {"c.txt"}, {"b.txt", "b2.txt"}} {
		if got := trial.TryDiffFile(ctx, diff, p, false, true); got.Status != TryApplies {
			t.Errorf("%s: %+v, want it to apply", p, got)
		}
	}
	// In, each comes back out.
	for _, p := range [][]string{{"a.txt"}, {"n.txt"}, {"c.txt"}, {"b.txt", "b2.txt"}} {
		if got := trial.TryDiffFile(ctx, diff, p, true, true); got.Status != TryApplies {
			t.Errorf("%s in reverse: %+v, want it to apply", p, got)
		}
	}

	// Line 6 changed both ways: by merging it conflicts, and without the versions it can't.
	trial, _ = repo.StartTrial(ctx, t.TempDir())
	if got := trial.TryDiffFile(ctx, clash, []string{"a.txt"}, false, false); got.Status != TryFails || got.Error == "" {
		t.Errorf("no merging: %+v", got)
	}
	if got := trial.TryDiffFile(ctx, clash, []string{"a.txt"}, false, true); got.Status != TryConflicts || !slices.Equal(got.Files, []string{"a.txt"}) {
		t.Errorf("merging: %+v", got)
	}
	if fx.Git("status", "--porcelain") != "" {
		t.Error("a try must leave the index and the files alone")
	}
}

func TestApplyDiffInEachMode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	all := []string{"a.txt", "n.txt", "c.txt", "b.txt", "b2.txt"}
	for mode, want := range map[DiffMode]string{
		// Status without renames, sorted: a rename is a deletion and a new file.
		DiffToFiles:         "?? b2.txt\n?? n.txt\nD b.txt\nD c.txt\nM a.txt",
		DiffToFilesAndIndex: "A  b2.txt\nA  n.txt\nD  b.txt\nD  c.txt\nM  a.txt",
		// Staged only: the files on disk are as they were, so they read as the reverse.
		DiffToIndex: "?? b.txt\n?? c.txt\nAD b2.txt\nAD n.txt\nD  b.txt\nD  c.txt\nMM a.txt",
	} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			fx, diff, _ := diffRepo(t)
			repo := OpenRepo(newTestRunner(t), fx.Dir)
			if err := repo.ApplyDiff(ctx, diff, all, mode, false, true); err != nil {
				t.Fatal(err)
			}
			if got := sortedStatus(fx); got != want {
				t.Errorf("status =\n%s\nwant\n%s", got, want)
			}
			onDisk, _ := os.ReadFile(fx.Path("a.txt"))
			if changed := strings.Contains(string(onDisk), "two\n"); changed == (mode == DiffToIndex) {
				t.Errorf("a.txt on disk = %q", onDisk)
			}
		})
	}
}

func sortedStatus(fx *gittest.Repo) string {
	lines := strings.Split(strings.TrimRight(fx.Git("-c", "status.renames=false", "status", "--porcelain", "--untracked-files=all"), "\n"), "\n")
	// Each trimmed: fx.Git trims the output, and with it the first line's leading space.
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

func TestApplyDiffOnlyThePathsNamed(t *testing.T) {
	t.Parallel()
	fx, diff, _ := diffRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if err := repo.ApplyDiff(context.Background(), diff, []string{"n.txt"}, DiffToFiles, false, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(fx.Git("status", "--porcelain", "--untracked-files=all")); got != "?? n.txt" {
		t.Errorf("status = %q, want only n.txt", got)
	}
}

func TestApplyDiffByMergingLeavesTheIndexAloneInFilesMode(t *testing.T) {
	t.Parallel()
	fx, diff, _ := diffRepo(t)
	// Line 4 changes too: line 2's change no longer fits as it is, but merges.
	fx.CommitFile("a.txt", numbered(map[int]string{4: "four", 6: "six"}), "main four")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	if err := repo.ApplyDiff(ctx, diff, []string{"a.txt"}, DiffToFiles, false, false); err == nil {
		t.Fatal("without merging it shouldn't fit")
	}
	if err := repo.ApplyDiff(ctx, diff, []string{"a.txt"}, DiffToFiles, false, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(fx.Git("status", "--porcelain")); got != "M a.txt" {
		t.Errorf("status = %q, want a.txt changed and not staged", got)
	}
	if onDisk, _ := os.ReadFile(fx.Path("a.txt")); string(onDisk) != numbered(map[int]string{2: "two", 4: "four", 6: "six"}) {
		t.Errorf("a.txt = %q", onDisk)
	}
}

func TestApplyDiffStopsOnAConflict(t *testing.T) {
	t.Parallel()
	fx, _, clash := diffRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	err := repo.ApplyDiff(context.Background(), clash, []string{"a.txt"}, DiffToFiles, false, true)
	if !errors.Is(err, ErrConflicts) {
		t.Fatalf("err = %v, want a conflict", err)
	}
	if got := strings.TrimSpace(fx.Git("status", "--porcelain")); got != "UU a.txt" {
		t.Errorf("status = %q", got)
	}
}

func TestApplyDiffInReverseTakesItBackOut(t *testing.T) {
	t.Parallel()
	fx, diff, _ := diffRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	all := []string{"a.txt", "n.txt", "c.txt", "b.txt", "b2.txt"}
	if err := repo.ApplyDiff(ctx, diff, all, DiffToFilesAndIndex, false, true); err != nil {
		t.Fatal(err)
	}
	fx.Commit("applied")
	if err := repo.ApplyDiff(ctx, diff, all, DiffToFilesAndIndex, true, true); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("diff", "--cached", "--name-only", "HEAD~1"); got != "" {
		t.Errorf("against before it was applied: %q", got)
	}
}

func TestUncleanNamesFilesWithYourChanges(t *testing.T) {
	t.Parallel()
	fx, _, _ := diffRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	fx.WriteFile("a.txt", "mine\n")
	fx.WriteFile("n.txt", "here already\n")
	fx.WriteFile("elsewhere.txt", "not asked about\n")
	got, err := repo.Unclean(context.Background(), []string{"a.txt", "n.txt", "c.txt"})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"a.txt", "n.txt"}) {
		t.Errorf("unclean = %v", got)
	}
}

func TestPutBackRestoresWhatHEADHas(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	all := []string{"a.txt", "n.txt", "c.txt", "b.txt", "b2.txt"}
	for _, mode := range []DiffMode{DiffToFiles, DiffToFilesAndIndex, DiffToIndex} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			fx, diff, _ := diffRepo(t)
			repo := OpenRepo(newTestRunner(t), fx.Dir)
			before, err := repo.Fingerprint(ctx, all)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.ApplyDiff(ctx, diff, []string{"a.txt", "n.txt", "c.txt", "b2.txt"}, mode, false, true); err != nil {
				t.Fatal(err)
			}
			after, _ := repo.Fingerprint(ctx, all)
			if after == before {
				t.Fatal("applying should change the fingerprint")
			}
			if again, _ := repo.Fingerprint(ctx, all); again != after {
				t.Fatal("nothing changed, so the fingerprint shouldn't")
			}
			if err := repo.PutBack(ctx, all); err != nil {
				t.Fatal(err)
			}
			if got := fx.Git("status", "--porcelain", "--untracked-files=all"); got != "" {
				t.Errorf("status after putting back = %q", got)
			}
			if back, _ := repo.Fingerprint(ctx, all); back != before {
				t.Error("putting back should match before")
			}
		})
	}
}

func TestPutBackClearsAConflict(t *testing.T) {
	t.Parallel()
	fx, _, clash := diffRepo(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	_ = repo.ApplyDiff(ctx, clash, []string{"a.txt"}, DiffToFiles, false, true)
	if err := repo.PutBack(ctx, []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	if got := fx.Git("status", "--porcelain"); got != "" {
		t.Errorf("status = %q", got)
	}
}
