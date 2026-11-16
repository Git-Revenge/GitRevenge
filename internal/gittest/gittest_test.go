package gittest

import (
	"os"
	"strings"
	"testing"
)

func TestLinearIsDeterministic(t *testing.T) {
	a := New(t).Linear(3)
	b := New(t).Linear(3)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("commit %d: SHAs differ between identical repos: %s vs %s", i, a[i], b[i])
		}
	}
}

func TestIgnoresUserConfig(t *testing.T) {
	r := New(t)
	if got := r.Git("config", "user.email"); got != "test@example.com" {
		t.Fatalf("user.email = %q", got)
	}
	want := "protocol.file.allow=always"
	if Git3() {
		want += "\ninit.defaultrefformat=reftable\ninit.defaultobjectformat=sha256\ninit.defaultbranch=main\nsafe.barerepository=explicit"
	}
	if out, _ := r.TryGit("config", "--global", "--list"); out != want {
		t.Fatalf("user's global config leaked into test repo: %q", out)
	}
}

func TestGit3Defaults(t *testing.T) {
	r, bare := New(t), NewBare(t)
	formats := "sha1\nfiles"
	if Git3() {
		formats = "sha256\nreftable"
	}
	for _, repo := range []*Repo{r, bare} {
		if got := repo.Git("rev-parse", "--show-object-format", "--show-ref-format"); got != formats {
			t.Errorf("%s: formats = %q, want %q", repo.Dir, got, formats)
		}
	}
	// A bare remote takes pushes and answers git even with safe.bareRepository=explicit.
	r.CommitFile("a.txt", "a\n", "first")
	r.Git("push", bare.Dir, "main")
	if got, want := bare.Git("rev-parse", "main"), r.Head(); got != want {
		t.Errorf("bare main = %s, want %s", got, want)
	}
}

func TestWithMergeHistory(t *testing.T) {
	r := WithMergeHistory(t)
	parents := strings.Fields(r.Git("rev-list", "--parents", "-n1", "HEAD"))
	if len(parents) != 3 {
		t.Fatalf("HEAD should be a merge commit, got %v", parents)
	}
	if got := r.Git("rev-list", "--count", "--all"); got != "6" {
		t.Fatalf("commit count = %s, want 6", got)
	}
	if got := r.Git("describe", "--tags", "--abbrev=0"); got != "v0.1.0" {
		t.Fatalf("tag = %q", got)
	}
}

func TestWithConflict(t *testing.T) {
	r := WithConflict(t)
	if got := r.Git("diff", "--name-only", "--diff-filter=U"); got != "conflict.txt" {
		t.Fatalf("conflicted files = %q", got)
	}
	if _, err := os.Stat(r.Path(".git/MERGE_HEAD")); err != nil {
		t.Fatalf("MERGE_HEAD missing: %v", err)
	}
}

func TestWithRemote(t *testing.T) {
	local, remote := WithRemote(t, 2)
	if got := local.Git("rev-parse", "origin/main"); got != remote.Git("rev-parse", "main") {
		t.Fatalf("origin/main %s does not match remote main", got)
	}
}

func TestStash(t *testing.T) {
	r := New(t)
	r.Linear(1)
	r.WriteFile("new.txt", "untracked\n")
	r.Stash("wip")
	if got := r.Git("stash", "list"); !strings.Contains(got, "wip") {
		t.Fatalf("stash list = %q", got)
	}
}

func TestAddSubmodule(t *testing.T) {
	sub := New(t)
	sub.Linear(1)
	r := New(t)
	r.Linear(1)
	r.AddSubmodule(sub, "libs/sub")
	if got := r.Git("submodule", "status"); !strings.Contains(got, "libs/sub") {
		t.Fatalf("submodule status = %q", got)
	}
}

func TestSetupLFS(t *testing.T) {
	r := New(t)
	r.SetupLFS("*.bin")
	r.WriteFile("big.bin", "binary-ish\n")
	r.Commit("add bin")
	if got := r.Git("lfs", "ls-files", "--name-only"); got != "big.bin" {
		t.Fatalf("lfs files = %q", got)
	}
}
