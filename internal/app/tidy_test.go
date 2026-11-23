package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestTidy(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 2)
	other := gittest.Clone(t, remote)
	other.Git("push", "-q", "origin", "main:merged", "main:gone-local", "main:stale")
	other.Git("switch", "-q", "-c", "feature")
	other.Commit("feature work")
	other.Git("push", "-q", "origin", "feature")
	local.Git("fetch", "-q", "origin")
	local.Git("branch", "-q", "--track", "gone-local", "origin/gone-local")

	// gone-local goes on the remote and a pruning fetch drops it: the local branch is gone.
	other.Git("push", "-q", "origin", "--delete", "gone-local")
	local.Git("fetch", "-q", "--prune", "origin")
	// Then pruning is off, and stale goes on the remote: its remote-tracking branch stays.
	local.Git("config", "remote.origin.prune", "false")
	other.Git("push", "-q", "origin", "--delete", "stale")
	local.Git("fetch", "-q", "origin")

	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	loadedPage(t, a, tab.Path)
	marks, err := a.TidyMarks(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"gone-local": TidyGone, "origin/stale": TidyGone, "origin/merged": TidyMerged}
	if len(marks) != len(want) {
		t.Fatalf("marks = %v, want %v", marks, want)
	}
	for k, v := range want {
		if marks[k] != v {
			t.Fatalf("marks = %v, want %v", marks, want)
		}
	}

	rep, err := a.RemoteTidy(tab.Path, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Base != "main" || rep.QuietDays != 0 {
		t.Fatalf("report = %+v", rep)
	}
	byName := map[string]TidyItem{}
	for _, it := range rep.Items {
		byName[it.Name] = it
	}
	if it := byName["gone-local"]; !it.Local || !it.Ticked || it.Unpushed != 0 {
		t.Fatalf("gone-local = %+v", it)
	}
	if it := byName["origin/stale"]; !it.Stale || !it.Ticked {
		t.Fatalf("origin/stale = %+v", it)
	}
	// Merged deletes on the remote, for everyone: never ticked.
	if it := byName["origin/merged"]; it.Ticked {
		t.Fatalf("origin/merged = %+v", it)
	}

	// A window makes the unmerged branch quiet (the fixture's commits are from January).
	if err := a.SetQuietDays(tab.Path, 30); err != nil {
		t.Fatal(err)
	}
	rep, _ = a.RemoteTidy(tab.Path, "origin")
	i := slices.IndexFunc(rep.Items, func(it TidyItem) bool { return it.Name == "origin/feature" })
	if i < 0 || rep.Items[i].Kind != TidyQuiet || rep.Items[i].Ticked || rep.Items[i].OnlyHere != 1 || rep.Items[i].LastAuthor == "" {
		t.Fatalf("quiet = %+v", rep.Items)
	}
	if err := a.SetQuietDays(tab.Path, 400); err != nil {
		t.Fatal(err)
	}
	if marks, _ := a.TidyMarks(tab.Path); marks["origin/feature"] != "" {
		t.Fatalf("quiet with a 400-day window: %v", marks)
	}

	// A local branch with work on no remote isn't ticked.
	local.Git("switch", "-q", "gone-local")
	local.Commit("only here")
	local.Git("switch", "-q", "main")
	rep, _ = a.RemoteTidy(tab.Path, "origin")
	if i := slices.IndexFunc(rep.Items, func(it TidyItem) bool { return it.Name == "gone-local" }); i < 0 || rep.Items[i].Ticked || rep.Items[i].Unpushed != 1 {
		t.Fatalf("gone-local with work = %+v", rep.Items)
	}

	// Tidying here deletes the local branch and the stale remote-tracking one, nothing else.
	res, err := a.TidyHere(tab.Path, []string{"gone-local"}, []string{"origin/stale"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Error != "" {
			t.Fatalf("results = %+v", res)
		}
	}
	refs := local.Git("for-each-ref", "--format=%(refname)")
	if strings.Contains(refs, "refs/heads/gone-local") || strings.Contains(refs, "refs/remotes/origin/stale") || !strings.Contains(refs, "refs/remotes/origin/merged") {
		t.Fatalf("refs after tidying:\n%s", refs)
	}
	// The checked-out branch is refused, with git's reason.
	res, _ = a.TidyHere(tab.Path, []string{"main"}, nil)
	if len(res) != 1 || res[0].Error == "" {
		t.Fatalf("deleting the checked-out branch: %+v", res)
	}
}

func TestFetchHeadBranches(t *testing.T) {
	t.Parallel()
	content := "a1\t\tbranch 'main' of https://example.com/r\n" +
		"b2\tnot-for-merge\tbranch 'fix/x' of https://example.com/r\n" +
		"c3\tnot-for-merge\t'refs/notes/commits' of https://example.com/r\n" +
		"d4\tnot-for-merge\ttag 'v1' of https://example.com/r\n" +
		"e5\tnot-for-merge\tbranch 'main' of /srv/other.git\n"
	got := git.FetchHeadBranches(content)
	if !slices.Equal(got["https://example.com/r"], []string{"main", "fix/x"}) || !slices.Equal(got["/srv/other.git"], []string{"main"}) || len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
