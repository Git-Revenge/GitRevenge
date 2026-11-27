package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// rerereRepo has main and side changing f.txt and h.txt in the same line, so merging side
// conflicts in both. rerere is on, and f.txt's resolution is recorded: merging again, rerere
// settles f.txt and h.txt stays a conflict.
func rerereRepo(t *testing.T) (*Repo, *gittest.Repo) {
	t.Helper()
	fx := gittest.New(t)
	fx.Git("config", "rerere.enabled", "true")
	fx.WriteFile("f.txt", "a\nb\nc\n")
	fx.WriteFile("h.txt", "x\n")
	fx.Commit("base")
	fx.Branch("side")
	fx.WriteFile("f.txt", "a\nSIDE\nc\n")
	fx.WriteFile("h.txt", "side\n")
	fx.Commit("side")
	fx.Checkout("main")
	fx.WriteFile("f.txt", "a\nMAIN\nc\n")
	fx.WriteFile("h.txt", "main\n")
	fx.Commit("main")
	if err := fx.Merge("side"); err == nil {
		t.Fatal("the merge didn't conflict")
	}
	fx.WriteFile("f.txt", "a\nBOTH\nc\n")
	fx.Git("rerere") // records f.txt's resolution, as a commit would
	fx.Git("merge", "--abort")
	// Only f.txt's is kept: h.txt's preimage goes, so it's a conflict rerere hasn't seen.
	entries, _ := os.ReadDir(filepath.Join(fx.Dir, ".git", "rr-cache"))
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(fx.Dir, ".git", "rr-cache", e.Name(), "postimage")); err != nil {
			_ = os.RemoveAll(filepath.Join(fx.Dir, ".git", "rr-cache", e.Name()))
		}
	}
	return OpenRepo(newTestRunner(t), fx.Dir), fx
}

func TestRerereResolvedTellsReusedResolutionsFromCleanMerges(t *testing.T) {
	t.Parallel()
	for _, autoUpdate := range []string{"false", "true"} {
		t.Run("autoUpdate="+autoUpdate, func(t *testing.T) {
			t.Parallel()
			repo, fx := rerereRepo(t)
			fx.Git("config", "rerere.autoUpdate", autoUpdate)
			ctx := context.Background()
			_ = fx.Merge("side")
			got, err := repo.RerereResolved(ctx)
			if err != nil || !slices.Equal(got, []string{"f.txt"}) {
				t.Fatalf("resolved = %v, %v", got, err)
			}
			if b, _ := os.ReadFile(filepath.Join(fx.Dir, "f.txt")); string(b) != "a\nBOTH\nc\n" {
				t.Fatalf("f.txt = %q", b)
			}
			// A file resolved by hand in this merge is still rerere's to record: not counted.
			fx.WriteFile("h.txt", "main\n")
			fx.Git("add", "h.txt")
			if got, _ = repo.RerereResolved(ctx); !slices.Equal(got, []string{"f.txt"}) {
				t.Fatalf("after resolving h.txt: %v", got)
			}
		})
	}
}

// resolve-undo outlives the commit: the next operation mustn't count the last one's files.
func TestRerereResolvedForgetsAFinishedMerge(t *testing.T) {
	t.Parallel()
	repo, fx := rerereRepo(t)
	fx.Git("config", "rerere.autoUpdate", "true")
	_ = fx.Merge("side")
	fx.WriteFile("h.txt", "main\n")
	fx.Git("add", "h.txt")
	fx.Git("commit", "-q", "--no-edit")
	if got, err := repo.RerereResolved(context.Background()); err != nil || len(got) != 0 {
		t.Fatalf("after the commit: %v, %v", got, err)
	}
}

func TestRerereResolvedWithRerereOff(t *testing.T) {
	t.Parallel()
	repo, fx := rerereRepo(t)
	fx.Git("config", "rerere.enabled", "false")
	ctx := context.Background()
	if repo.RerereOn(ctx) {
		t.Fatal("rerere.enabled false reads as on")
	}
	_ = fx.Merge("side")
	if got, err := repo.RerereResolved(ctx); err != nil || got != nil {
		t.Fatalf("off: %v, %v", got, err)
	}
	// Unset, rerere is on when there's an rr-cache, as git decides it.
	fx.Git("config", "--unset", "rerere.enabled")
	if !repo.RerereOn(ctx) {
		t.Error("an rr-cache with rerere.enabled unset reads as off")
	}
	_ = os.RemoveAll(filepath.Join(fx.Dir, ".git", "rr-cache"))
	if repo.RerereOn(ctx) {
		t.Error("no rr-cache and nothing set reads as on")
	}
	if err := repo.TurnOnRerere(ctx); err != nil || !repo.RerereOn(ctx) {
		t.Errorf("turning it on: %v", err)
	}
}

func TestForgetResolutionPutsTheConflictBack(t *testing.T) {
	t.Parallel()
	for _, autoUpdate := range []string{"false", "true"} {
		t.Run("autoUpdate="+autoUpdate, func(t *testing.T) {
			t.Parallel()
			repo, fx := rerereRepo(t)
			fx.Git("config", "rerere.autoUpdate", autoUpdate)
			ctx := context.Background()
			_ = fx.Merge("side")
			if err := repo.ForgetResolution(ctx, "f.txt"); err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(filepath.Join(fx.Dir, "f.txt"))
			if !strings.Contains(string(b), "<<<<<<<") || !strings.Contains(string(b), "MAIN") {
				t.Errorf("f.txt = %q, want the conflict back", b)
			}
			if got, _ := repo.RerereResolved(ctx); len(got) != 0 {
				t.Errorf("still counted as resolved: %v", got)
			}
			// Resolved again and recorded, the next merge reuses the new resolution.
			fx.WriteFile("f.txt", "a\nNEW\nc\n")
			fx.Git("rerere")
			fx.Git("merge", "--abort")
			_ = fx.Merge("side")
			if b, _ := os.ReadFile(filepath.Join(fx.Dir, "f.txt")); string(b) != "a\nNEW\nc\n" {
				t.Errorf("next time f.txt = %q", b)
			}
		})
	}
}

func TestConflictFingerprintIsTheConflictsText(t *testing.T) {
	t.Parallel()
	repo, fx := rerereRepo(t)
	fx.Git("config", "rerere.enabled", "false")
	ctx := context.Background()
	_ = fx.Merge("side")
	f, err := repo.ConflictFingerprint(ctx, "f.txt")
	if err != nil || f == "" {
		t.Fatalf("f.txt: %q, %v", f, err)
	}
	h, _ := repo.ConflictFingerprint(ctx, "h.txt")
	if h == "" || h == f {
		t.Fatalf("h.txt %q against f.txt %q", h, f)
	}
	fx.Git("merge", "--abort")

	// The same conflict, the other way round, as a rebase meets it: the same name.
	fx.Checkout("side")
	_ = fx.Merge("main")
	if again, _ := repo.ConflictFingerprint(ctx, "f.txt"); again != f {
		t.Errorf("swapped sides: %q, want %q", again, f)
	}
	if _, err := repo.ConflictFingerprint(ctx, "not-conflicted.txt"); err != nil {
		t.Errorf("a file not in conflict: %v", err)
	}
}

func TestHunksHash(t *testing.T) {
	t.Parallel()
	a := hunksHash("x\n<<<<<<< ours\none\n=======\ntwo\n>>>>>>> theirs\ny\n")
	b := hunksHash("other\n<<<<<<< HEAD\ntwo\n||||||| base\nzero\n=======\none\n>>>>>>> side\nlines\n")
	if a == "" || a != b {
		t.Errorf("the same hunk, sides swapped and a base shown: %q, %q", a, b)
	}
	if hunksHash("no conflict\n") != "" {
		t.Error("a file without a hunk has a name")
	}
}
