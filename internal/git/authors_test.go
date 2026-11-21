package git

import (
	"context"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestParseIdentity(t *testing.T) {
	for in, want := range map[string]Identity{
		"Maya Okafor <maya@example.com>":    {Name: "Maya Okafor", Email: "maya@example.com"},
		"  Maya   <maya@example.com>  ":     {Name: "Maya", Email: "maya@example.com"},
		"<maya@example.com>":                {Email: "maya@example.com"},
		"Maya Okafor":                       {Name: "Maya Okafor"},
		"dependabot[bot] <1+d@example.com>": {Name: "dependabot[bot]", Email: "1+d@example.com"},
		"Maya <maya@example.com":            {Name: "Maya", Email: "maya@example.com"},
		"":                                  {},
	} {
		if got := ParseIdentity(in); got != want {
			t.Errorf("ParseIdentity(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestAuthorLogReadsAuthorsAndCoAuthorTrailers(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1", "Start")
	fx.WriteFile("a.txt", "2")
	fx.Git("-c", "user.name=Ada", "-c", "user.email=ada@example.com", "commit", "-qam", "Pair up", "-m", "Reviewed-by: Rex <rex@example.com>\nCo-authored-by: Bo <bo@example.com>\nCo-developed-by: Cy <cy@example.com>")
	fx.Git("checkout", "-q", "-b", "side", "HEAD~1")
	fx.CommitFile("b.txt", "1", "On the side")
	fx.Checkout("main")
	fx.Git("merge", "-q", "--no-ff", "-m", "Merge branch 'side'", "side")
	r := OpenRepo(newTestRunner(t), fx.Dir)

	var got []AuthorCommit
	if err := r.AuthorLog(context.Background(), 0, []string{"main"}, func(c AuthorCommit) { got = append(got, c) }); err != nil {
		t.Fatal(err)
	}
	// Three commits: the merge isn't read.
	if len(got) != 3 {
		t.Fatalf("commits = %+v, want 3 without the merge", got)
	}
	var pair *AuthorCommit
	for i := range got {
		if got[i].Name == "Ada" {
			pair = &got[i]
		}
	}
	if pair == nil || pair.Email != "ada@example.com" || pair.CoAuthors != "Bo <bo@example.com>\x1dCy <cy@example.com>" || pair.Committed == 0 || len(pair.SHA) != 40 {
		t.Fatalf("the paired commit = %+v, want its two co-authors and not its reviewer", pair)
	}

	// A range reads only what the branch has of its own.
	got = nil
	if err := r.AuthorLog(context.Background(), 0, []string{"main~1..side"}, func(c AuthorCommit) { got = append(got, c) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Test User" {
		t.Fatalf("the range = %+v, want side's one commit", got)
	}

	// A name that git would read as an option is refused.
	if err := r.AuthorLog(context.Background(), 0, []string{"--all"}, func(AuthorCommit) {}); err == nil {
		t.Fatal("an option was accepted as a revision")
	}
}
