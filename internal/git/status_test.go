package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func status(t *testing.T, repo *Repo) map[string]FileStatus {
	t.Helper()
	st, err := repo.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]FileStatus{}
	for _, f := range st.Files {
		m[f.Path] = f
	}
	return m
}

func TestStatusKinds(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.WriteFile("mod.txt", "a\n")
	fx.WriteFile("del.txt", "d\n")
	fx.WriteFile("old.txt", strings.Repeat("rename\n", 20))
	fx.WriteFile("both.txt", "1\n")
	fx.Commit("base")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	fx.WriteFile("mod.txt", "b\n")     // unstaged modify
	fx.RemoveFile("del.txt")           // unstaged delete
	fx.Git("mv", "old.txt", "new.txt") // staged rename
	fx.WriteFile("added.txt", "new\n") // staged add
	fx.Git("add", "added.txt")
	fx.WriteFile("both.txt", "2\n") // staged, then modified again
	fx.Git("add", "both.txt")
	fx.WriteFile("both.txt", "3\n")
	fx.WriteFile("dir with space/untracked file.txt", "?") // untracked, nested

	s := status(t, repo)
	check := func(path, x, y string, staged, unstaged bool) {
		t.Helper()
		f, ok := s[path]
		if !ok {
			t.Fatalf("%s missing from status: %+v", path, s)
		}
		if f.Index != x || f.WorkTree != y || f.Staged() != staged || f.Unstaged() != unstaged {
			t.Fatalf("%s = %+v (staged %v unstaged %v)", path, f, f.Staged(), f.Unstaged())
		}
	}
	check("mod.txt", ".", "M", false, true)
	check("del.txt", ".", "D", false, true)
	check("added.txt", "A", ".", true, false)
	check("both.txt", "M", "M", true, true)
	check("new.txt", "R", ".", true, false)
	if s["new.txt"].OrigPath != "old.txt" {
		t.Fatalf("rename source = %q", s["new.txt"].OrigPath)
	}
	if u := s["dir with space/untracked file.txt"]; !u.Untracked || u.Staged() || !u.Unstaged() {
		t.Fatalf("untracked = %+v", u)
	}
}

func TestStatusBranchInfo(t *testing.T) {
	t.Parallel()
	local, _ := gittest.WithRemote(t, 1)
	local.CommitFile("x.txt", "x", "ahead")
	st, err := OpenRepo(newTestRunner(t), local.Dir).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Head != "main" || st.Upstream != "origin/main" || st.Ahead != 1 || st.Behind != 0 || st.Oid != local.Head() {
		t.Fatalf("status = %+v", st)
	}
}

func TestStatusConflictAndEmptyRepo(t *testing.T) {
	t.Parallel()
	fx := gittest.WithConflict(t)
	if f := status(t, OpenRepo(newTestRunner(t), fx.Dir))["conflict.txt"]; !f.Conflicted || f.Staged() || !f.Unstaged() {
		t.Fatalf("conflicted = %+v", f)
	}

	empty := gittest.New(t)
	empty.WriteFile("a.txt", "a")
	st, err := OpenRepo(newTestRunner(t), empty.Dir).Status(context.Background())
	if err != nil || st.Oid != "" || st.Head != "main" || len(st.Files) != 1 {
		t.Fatalf("empty repo status = %+v, %v", st, err)
	}
}

func TestStageUnstageRoundTrip(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	fx.WriteFile("file.txt", "changed\n")
	// "br[ac]ket.txt" is a valid name on every OS but also a glob that would match
	// "braket.txt"; literal pathspecs must stage only the former.
	fx.WriteFile("br[ac]ket.txt", "literal\n")
	fx.WriteFile("braket.txt", "decoy\n")
	fx.WriteFile("other.txt", "x\n")

	if err := repo.Stage(ctx, []string{"file.txt", "br[ac]ket.txt"}); err != nil {
		t.Fatal(err)
	}
	s := status(t, repo)
	if !s["file.txt"].Staged() || !s["br[ac]ket.txt"].Staged() || s["braket.txt"].Staged() || s["other.txt"].Staged() {
		t.Fatalf("after stage: %+v", s)
	}
	if err := repo.Unstage(ctx, []string{"file.txt"}); err != nil {
		t.Fatal(err)
	}
	if s := status(t, repo); s["file.txt"].Staged() || !s["file.txt"].Unstaged() {
		t.Fatalf("after unstage: %+v", s["file.txt"])
	}
	if err := repo.StageAll(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.UnstageAll(ctx); err != nil {
		t.Fatal(err)
	}
	for p, f := range status(t, repo) {
		if f.Staged() {
			t.Fatalf("%s still staged after UnstageAll", p)
		}
	}
}

func TestUnstageBeforeFirstCommit(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	fx.WriteFile("a.txt", "a")
	fx.WriteFile("b.txt", "b")
	if err := repo.StageAll(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.Unstage(ctx, []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	s := status(t, repo)
	if !s["a.txt"].Untracked || !s["b.txt"].Staged() {
		t.Fatalf("after unstage on unborn HEAD: %+v", s)
	}
	if err := repo.UnstageAll(ctx); err != nil {
		t.Fatal(err)
	}
	if !status(t, repo)["b.txt"].Untracked {
		t.Fatal("UnstageAll on unborn HEAD should leave files untracked")
	}
}

func TestDiscard(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("keep.txt", "original\n", "base")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	fx.WriteFile("keep.txt", "staged\n")
	fx.Git("add", "keep.txt")
	fx.WriteFile("keep.txt", "staged plus unstaged\n")
	fx.WriteFile("junk/new.txt", "junk")

	s := status(t, repo)
	if err := repo.Discard(ctx, []FileStatus{s["keep.txt"], s["junk/new.txt"]}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(fx.Path("keep.txt")); string(b) != "staged\n" {
		t.Fatalf("discard should restore the staged version, got %q", b)
	}
	if _, err := os.Stat(fx.Path("junk/new.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file not deleted")
	}

	conflicted := FileStatus{Path: "c.txt", Conflicted: true}
	if err := repo.Discard(ctx, []FileStatus{conflicted}); !errors.Is(err, ErrConflicted) {
		t.Fatalf("err = %v", err)
	}
	escape := FileStatus{Path: "../outside.txt", Untracked: true, Index: "?", WorkTree: "?"}
	if err := repo.Discard(ctx, []FileStatus{escape}); err == nil {
		t.Fatal("path outside the repo must be refused")
	}
}

func TestCommitAndAmend(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	fx.WriteFile("a.txt", "a")
	fx.Git("add", "a.txt")
	msg := "Add a\n\n#123 keeps hash lines\nbody"
	sha, err := repo.Commit(ctx, msg, false)
	if err != nil {
		t.Fatal(err)
	}
	if sha != fx.Head() {
		t.Fatal("Commit should return the new HEAD")
	}
	if got, _ := repo.LastCommitMessage(ctx); got != msg {
		t.Fatalf("message = %q", got)
	}

	amended, err := repo.Commit(ctx, "Add a (reworded)", true)
	if err != nil {
		t.Fatal(err)
	}
	if amended == sha || fx.Git("rev-list", "--count", "HEAD") != "2" {
		t.Fatal("amend should replace the commit, not add one")
	}
	if _, err := repo.Commit(ctx, "   ", false); err == nil {
		t.Fatal("empty message accepted")
	}
}

func TestCommitHookFailureIsReported(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh for hooks")
	}
	fx := gittest.New(t)
	fx.Linear(1)
	hook := fx.Path(".git/hooks/pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'lint failed: fix me' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fx.WriteFile("a.txt", "a")
	fx.Git("add", "a.txt")
	_, err := OpenRepo(newTestRunner(t), fx.Dir).Commit(context.Background(), "msg", false)
	var gerr *Error
	if !errors.As(err, &gerr) || !strings.Contains(gerr.Stderr, "lint failed") {
		t.Fatalf("hook output should surface: %v", err)
	}
}

func TestCommitSignedWithSSH(t *testing.T) {
	t.Parallel()
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("ssh-keygen not installed")
	}
	fx := gittest.New(t)
	fx.Linear(1)
	key := filepath.Join(t.TempDir(), "key")
	if out, err := exec.Command(keygen, "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen failed: %v %s", err, out)
	}
	fx.Git("config", "gpg.format", "ssh")
	fx.Git("config", "user.signingkey", filepath.ToSlash(key))
	fx.Git("config", "commit.gpgsign", "true")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	sig := repo.CommitSigning(context.Background())
	if !sig.Enabled || sig.Format != "ssh" {
		t.Fatalf("signing = %+v", sig)
	}
	fx.WriteFile("s.txt", "signed")
	fx.Git("add", "s.txt")
	if _, err := repo.Commit(context.Background(), "signed commit", false); err != nil {
		t.Fatal(err)
	}
	if raw := fx.Git("cat-file", "commit", "HEAD"); !strings.Contains(raw, "gpgsig") {
		t.Fatal("commit should carry a signature when commit.gpgsign is set")
	}
}

func TestCommitIdentity(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if got := repo.CommitIdentity(context.Background()); got != (Identity{Name: "Test User", Email: "test@example.com"}) {
		t.Fatalf("identity = %+v", got)
	}
}

func TestCommitNothingStaged(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	fx.WriteFile("unstaged.txt", "x")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if _, err := repo.Commit(context.Background(), "msg", false); !errors.Is(err, ErrNothingStaged) {
		t.Fatalf("err = %v", err)
	}
	// Amending with nothing staged is how you reword the last commit.
	if _, err := repo.Commit(context.Background(), "reworded", true); err != nil {
		t.Fatal(err)
	}
}
