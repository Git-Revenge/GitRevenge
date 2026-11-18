package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// GHSA-qvcv-jp5p-2vw5, end to end.
//
// A remote's owner names its default branch "--exec=<command>". A clone checks that branch
// out, so it's a local branch like any other, and the app passes local branches to git by
// their short names. Rebasing another branch onto it ran "git rebase --exec=<command>",
// which runs the command.
func TestABranchNamedLikeAnOptionRunsNothing(t *testing.T) {
	const branch = "--exec=touch${IFS}ran.txt"
	src := gittest.New(t)
	src.CommitFile("a.txt", "a\n", "one")
	main := src.Git("symbolic-ref", "--short", "HEAD")
	src.Git("update-ref", "refs/heads/"+branch, "HEAD")
	src.Git("symbolic-ref", "HEAD", "refs/heads/"+branch)
	fx := gittest.Clone(t, src)
	if got := fx.Git("symbolic-ref", "--short", "HEAD"); got != branch {
		t.Fatalf("the clone is on %q, not the remote's default branch, so this checks nothing", got)
	}
	// The user's own branch, with work of their own on it.
	fx.Git("switch", "-q", "-c", main, "--track", "origin/"+main)
	fx.CommitFile("mine.txt", "mine\n", "my work")

	a := newTestApp(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if heads := fx.Git("for-each-ref", "--format=%(refname:short)", "refs/heads"); !strings.Contains(heads, branch) {
		t.Fatalf("there's no local branch of that name, so this checks nothing: %q", heads)
	}

	// What the branch's menu offers, with the name the menu passes.
	if err := a.Integrate(tab.Path, "rebase", []string{branch}); err == nil {
		t.Error("rebase onto the branch went ahead")
	}
	_ = a.Integrate(tab.Path, "rebase", []string{branch, "autosquash"})
	_ = a.Integrate(tab.Path, "merge", []string{branch, ""})
	_, _ = a.PreviewRebase(tab.Path, branch)
	_, _ = a.PreviewMerge(tab.Path, branch)
	_ = a.BranchAction(tab.Path, "checkout", []string{branch})
	_ = a.BranchAction(tab.Path, "delete", []string{branch, "true"})
	_ = a.PullBranch(tab.Path, branch)

	if _, err := os.Stat(filepath.Join(fx.Dir, "ran.txt")); err == nil {
		t.Error("the command in the branch's name ran")
	}
}

// The same with a tag, and git's --output, which writes a file. In the app a tag goes to git
// by its full name ("refs/tags/…"), which is no option, wherever it's merged, rebased onto or
// checked out; this holds the places that take a name as it's typed or listed.
func TestATagNamedLikeAnOptionWritesNoFile(t *testing.T) {
	const tag = "--output=planted.txt"
	src := gittest.New(t)
	src.CommitFile("a.txt", "a\n", "one")
	src.Git("update-ref", "refs/tags/v1", "HEAD")
	src.CommitFile("b.txt", "b\n", "two")
	src.Git("update-ref", "refs/tags/"+tag, "HEAD")
	fx := gittest.Clone(t, src)
	a := newTestApp(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := a.InsightsTags(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(tags, func(i git.TagInfo) bool { return i.Name == tag }) {
		t.Fatalf("the tag isn't offered, so this checks nothing: %+v", tags)
	}

	if _, err := a.InsightsShipped(tab.Path, InsightsInputs{Window: "7d", Target: tag}); err == nil {
		t.Error("InsightsShipped measured against the name")
	}
	if _, err := a.InsightsHotspots(tab.Path, InsightsInputs{Window: "7d", Target: tag}); err == nil {
		t.Error("InsightsHotspots measured against the name")
	}
	_, _ = a.InsightsShipped(tab.Path, InsightsInputs{Window: "releases", From: tag, To: "v1"})
	_, _ = a.PreviewMerge(tab.Path, tag)
	_, _ = a.PreviewRebase(tab.Path, tag)
	_, _ = a.PlanBisect(tab.Path, tag, "v1")
	_ = a.StartBisect(tab.Path, "HEAD", tag)
	_ = a.BranchAction(tab.Path, "untag", []string{tag})
	_ = a.BranchAction(tab.Path, "checkout", []string{tag})
	_ = a.BranchAction(tab.Path, "create", []string{"topic", tag, ""})
	_ = a.Integrate(tab.Path, "merge", []string{tag, ""})
	_ = a.Integrate(tab.Path, "reset", []string{tag, "mixed"})

	if files, _ := filepath.Glob(filepath.Join(fx.Dir, "planted.txt*")); len(files) > 0 {
		t.Errorf("git wrote %v: the tag's name was read as --output", files)
	}
}
