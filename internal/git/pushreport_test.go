package git

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestPushReport(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Commit("one")
	ok, behind, after := gittest.NewBare(t), gittest.NewBare(t), gittest.NewBare(t)
	missing := filepath.Join(t.TempDir(), "missing.git")
	// behind already has work this repository doesn't: its push is rejected, and git goes on.
	theirs := gittest.Clone(t, behind)
	theirs.Commit("theirs")
	theirs.Git("push", "-q", "origin", "HEAD:main")

	fx.Git("remote", "add", "origin", ok.Dir)
	for _, u := range []string{ok.Dir, behind.Dir, missing, after.Dir} {
		fx.Git("remote", "set-url", "--add", "--push", "origin", u)
	}
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	dests, err := repo.PushWithReport(ctx, PushOptions{Remote: "origin", Refspec: "HEAD:refs/heads/main"}, NetOptions{})
	if err == nil {
		t.Fatal("a push that couldn't reach one URL reported no error")
	}
	want := []string{PushPushed, PushRejected, PushFailed, PushNotTried}
	if len(dests) != len(want) {
		t.Fatalf("dests = %+v", dests)
	}
	for i, w := range want {
		if dests[i].State != w {
			t.Fatalf("dest %d = %+v, want %s (all: %+v)", i, dests[i], w, dests)
		}
	}
	if dests[1].Detail == "" || dests[2].Detail == "" {
		t.Fatalf("reasons missing: %+v", dests)
	}

	// Going on with the one that wasn't tried.
	if d := repo.PushToURL(ctx, after.Dir, "HEAD:refs/heads/main", NetOptions{}); d.State != PushPushed {
		t.Fatalf("push to the rest = %+v", d)
	}
	if d := repo.PushToURL(ctx, after.Dir, "HEAD:refs/heads/main", NetOptions{}); d.State != PushUpToDate {
		t.Fatalf("again = %+v", d)
	}
}

func TestPushDestsWithOneURL(t *testing.T) {
	t.Parallel()
	out := "To https://example.com/r.git\n=\trefs/heads/main:refs/heads/main\t[up to date]\nDone\n"
	if d := PushDests([]string{"https://example.com/r"}, out, nil); d[0].State != PushUpToDate {
		t.Fatalf("got %+v", d)
	}
	out = "To https://example.com/r.git\n*\trefs/heads/x:refs/heads/x\t[new branch]\nDone\n"
	if d := PushDests([]string{"https://example.com/r.git"}, out, nil); d[0].State != PushPushed {
		t.Fatalf("got %+v", d)
	}
}

func TestRejectionReason(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"[rejected] (fetch first)":          "fetch first",
		"[rejected] (non-fast-forward)":     "non-fast-forward",
		"[remote rejected] (hook declined)": "hook declined",
		"[rejected]":                        "rejected",
	} {
		if got := rejectionReason(in); got != want {
			t.Errorf("rejectionReason(%q) = %q, want %q", in, got, want)
		}
	}
}

// A refusal in --porcelain output reads, on stderr, as git's plain push writes it, so what
// explains refusals keeps working.
func TestClassicRefusals(t *testing.T) {
	t.Parallel()
	out := "To ../r.git\n" +
		"!\trefs/heads/main:refs/heads/main\t[remote rejected] (pre-receive hook declined)\n" +
		"!\trefs/heads/fix:refs/heads/fix\t[rejected] (fetch first)\n" +
		"*\trefs/heads/new:refs/heads/new\t[new branch]\n" +
		"Done\n"
	got := classicRefusals(out)
	for _, want := range []string{" ! [remote rejected] main -> main (pre-receive hook declined)\n", " ! [rejected] fix -> fix (fetch first)\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "new") {
		t.Errorf("a pushed ref came out as refused:\n%s", got)
	}
}
