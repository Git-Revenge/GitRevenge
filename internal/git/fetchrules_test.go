package git

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestFetchRules(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Commit("one")
	fx.Git("remote", "add", "origin", "https://example.com/r.git")
	// A refspec the app didn't write, kept as it is through every save.
	fx.Git("config", "--add", "remote.origin.fetch", "+refs/notes/*:refs/notes/origin/*")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	read := func() FetchRules {
		t.Helper()
		rs, err := repo.RemoteDetails(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return ReadFetchRules(rs[0], repo.RemoteSettings(ctx).FetchPrune)
	}
	cfg := func(k string) string {
		out, _ := repo.ReadString(ctx, "config", "--local", "--get-all", k)
		return out
	}

	fr := read()
	if !fr.AllBranches || !fr.Prune || fr.TagOpt != "" || fr.SkipFetchAll || len(fr.Patterns) != 0 || !slices.Equal(fr.Other, []string{"+refs/notes/*:refs/notes/origin/*"}) {
		t.Fatalf("defaults = %+v", fr)
	}

	fr = FetchRules{SkipFetchAll: true, Prune: false, TagOpt: "--no-tags", Patterns: []string{"main", "release/*"}, Other: fr.Other}
	if err := repo.SetFetchRules(ctx, "origin", fr); err != nil {
		t.Fatal(err)
	}
	got := read()
	if got.AllBranches || got.Prune || got.TagOpt != "--no-tags" || !got.SkipFetchAll || !slices.Equal(got.Patterns, []string{"main", "release/*"}) || !slices.Equal(got.Other, fr.Other) {
		t.Fatalf("after setting = %+v", got)
	}
	if specs := cfg("remote.origin.fetch"); !strings.Contains(specs, "+refs/heads/release/*:refs/remotes/origin/release/*") || !strings.Contains(specs, "refs/notes/") {
		t.Fatalf("refspecs:\n%s", specs)
	}

	// Back to the defaults: nothing extra is left in config.
	if err := repo.SetFetchRules(ctx, "origin", FetchRules{AllBranches: true, Prune: true, Other: fr.Other}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"remote.origin.prune", "remote.origin.tagOpt", "remote.origin.skipFetchAll"} {
		if v := cfg(k); v != "" {
			t.Errorf("%s = %q, want unset", k, v)
		}
	}
	if got := read(); !got.AllBranches || !got.Prune {
		t.Fatalf("back to defaults = %+v", got)
	}

	// With fetch.prune false, dropping deleted branches needs saying on the remote.
	fx.Git("config", "fetch.prune", "false")
	if got := read(); got.Prune {
		t.Fatal("fetch.prune false should read as not pruning")
	}
	if err := repo.SetFetchRules(ctx, "origin", FetchRules{AllBranches: true, Prune: true, Other: fr.Other}); err != nil {
		t.Fatal(err)
	}
	if v := cfg("remote.origin.prune"); v != "true" {
		t.Fatalf("remote.origin.prune = %q", v)
	}

	for _, bad := range [][]string{{"a b"}, {"*/*"}, {"x:y"}, {""}, {}} {
		err := repo.SetFetchRules(ctx, "origin", FetchRules{Prune: true, Patterns: bad})
		if !errors.Is(err, ErrBadPattern) {
			t.Errorf("patterns %q: err = %v", bad, err)
		}
	}
}
