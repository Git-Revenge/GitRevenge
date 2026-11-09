package git

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestReferences(t *testing.T) {
	t.Parallel()
	for msg, want := range map[string][]string{
		"Cache readings offline (TIDE-88)":                        {"TIDE-88"},
		"Paginate readings (#41)\n\nBody mentions OTHER-1.":       {"#41"},
		"Fix rounding\n\nDetails.\n\nRefs: TIDE-9, TIDE-10":       {"TIDE-9", "TIDE-10"},
		"TIDE-3 and TIDE-3 again\n\nCloses: TIDE-3\nSee: HARB-14": {"TIDE-3", "HARB-14"},
		"Plain subject\n\nSigned-off-by: Someone <s@example.com>": {},
		"lowercase-12 is not a reference":                         {},
	} {
		if got := References(msg); !slices.Equal(got, want) {
			t.Errorf("References(%q) = %v, want %v", msg, got, want)
		}
	}
}

// shippedFixture builds main with a squash-style commit, a merged branch, a revert and a
// fixup, one minute apart from the fixture epoch, and returns the repo and the commits.
func shippedFixture(t *testing.T) (*gittest.Repo, map[string]string) {
	t.Helper()
	fx := gittest.New(t)
	c := map[string]string{}
	c["base"] = fx.CommitFile("a.txt", "one\n", "Start")
	c["squash"] = fx.CommitFile("a.txt", "one\ntwo\n", "Add two (TIDE-1)")
	fx.Branch("feature")
	fx.Checkout("feature")
	c["f1"] = fx.CommitFile("b.txt", "b\n", "Feature part one")
	c["f2"] = fx.CommitFile("b.txt", "b\nb2\n", "Feature part two")
	fx.Checkout("main")
	c["other"] = fx.CommitFile("c.txt", "c\n", "Unrelated change\n\nRefs: HARB-2")
	fx.Git("merge", "--no-ff", "-m", "Merge feature (TIDE-2)", "feature")
	c["merge"] = fx.Head()
	c["revert"] = fx.Commit("Revert \"Unrelated change\"")
	c["fixup"] = fx.CommitFile("a.txt", "one\ntwo!\n", "fixup! Add two (TIDE-1)")
	return fx, c
}

func TestShipped(t *testing.T) {
	t.Parallel()
	fx, c := shippedFixture(t)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	target, err := repo.InsightsTarget(ctx)
	if err != nil || target != "main" {
		t.Fatalf("target = %q, %v", target, err)
	}
	tip, err := repo.Tip(ctx, target)
	if err != nil || tip.SHA != c["fixup"] {
		t.Fatalf("tip = %+v, %v", tip, err)
	}

	all, err := repo.Shipped(ctx, ShippedQuery{Target: target, Since: 0, Until: tip.Time})
	if err != nil {
		t.Fatal(err)
	}
	var shas []string
	for _, it := range all {
		shas = append(shas, it.SHA)
	}
	// First-parent only: the feature branch's own commits aren't items.
	if want := []string{c["fixup"], c["revert"], c["merge"], c["other"], c["squash"], c["base"]}; !slices.Equal(shas, want) {
		t.Fatalf("items = %v, want %v", shas, want)
	}
	by := func(sha string) ShippedItem {
		return all[slices.IndexFunc(all, func(it ShippedItem) bool { return it.SHA == sha })]
	}
	merge := by(c["merge"])
	if !merge.Merge || merge.Files != 1 || merge.Added != 2 || !slices.Equal(merge.Refs, []string{"TIDE-2"}) {
		t.Errorf("merge = %+v", merge)
	}
	// The branch's first commit is two commits before the merge, one minute apart.
	if merge.Cycle <= 0 {
		t.Errorf("merge cycle = %d, want the time from feature's first commit", merge.Cycle)
	}
	if sq := by(c["squash"]); sq.Merge || sq.Cycle != 0 || sq.Added != 1 || !slices.Equal(sq.Refs, []string{"TIDE-1"}) {
		t.Errorf("squash = %+v", sq)
	}
	if o := by(c["other"]); !slices.Equal(o.Refs, []string{"HARB-2"}) || o.Author != "Test User" {
		t.Errorf("other = %+v", o)
	}
	if !by(c["revert"]).Revert || !by(c["fixup"]).Fixup || by(c["merge"]).Revert {
		t.Error("revert and fixup flags")
	}

	// A window keeps only the commits committed inside it, inclusive. The merge didn't
	// advance the fixture clock, so the revert shares its second and is inside too.
	other, merged := by(c["other"]).Committed, by(c["merge"]).Committed
	win, err := repo.Shipped(ctx, ShippedQuery{Target: target, Since: other, Until: merged})
	if err != nil {
		t.Fatal(err)
	}
	var winSHAs []string
	for _, it := range win {
		winSHAs = append(winSHAs, it.SHA)
	}
	if want := []string{c["revert"], c["merge"], c["other"]}; !slices.Equal(winSHAs, want) {
		t.Errorf("window = %v, want %v", winSHAs, want)
	}

	// A range lists the commits between two revisions.
	fx.Git("tag", "v1", c["squash"])
	fx.Git("tag", "v2", c["merge"])
	rng, err := repo.Shipped(ctx, ShippedQuery{Target: target, Range: "v1..v2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rng) != 2 || rng[0].SHA != c["merge"] || rng[1].SHA != c["other"] {
		t.Errorf("range = %+v", rng)
	}
	tags, err := repo.Tags(ctx)
	if err != nil || len(tags) != 2 || tags[0].Name != "v2" {
		t.Errorf("tags = %+v, %v", tags, err)
	}
}

// TestShippedIgnoresPersonalConfig is ADR-0008's promise: a user's own mailmap file and
// diff settings don't change the results; the committed .mailmap does.
func TestShippedIgnoresPersonalConfig(t *testing.T) {
	fx, _ := shippedFixture(t)
	ctx := context.Background()
	plain, err := OpenRepo(newTestRunner(t), fx.Dir).Shipped(ctx, ShippedQuery{Target: "main", Until: 1 << 40})
	if err != nil {
		t.Fatal(err)
	}

	personal := filepath.Join(t.TempDir(), "mailmap")
	if err := os.WriteFile(personal, []byte("Someone Else <test@example.com>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	cfg := "[mailmap]\n\tfile = " + filepath.ToSlash(personal) + "\n[diff]\n\trenames = false\n\tnoprefix = true\n[log]\n\tshowSignature = true\n"
	if err := os.WriteFile(global, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	custom, err := OpenRepo(newTestRunner(t), fx.Dir).Shipped(ctx, ShippedQuery{Target: "main", Until: 1 << 40})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain, custom) {
		t.Fatalf("personal config changed the results:\n%+v\n%+v", plain, custom)
	}

	// A committed .mailmap at the target does apply.
	fx.CommitFile(".mailmap", "Mapped Name <test@example.com>\n", "Add mailmap")
	mapped, err := OpenRepo(newTestRunner(t), fx.Dir).Shipped(ctx, ShippedQuery{Target: "main", Until: 1 << 40})
	if err != nil {
		t.Fatal(err)
	}
	if mapped[0].Author != "Mapped Name" {
		t.Errorf("author with a committed .mailmap = %q", mapped[0].Author)
	}
}

func TestInsightsTargetPrefersOriginHead(t *testing.T) {
	t.Parallel()
	origin := gittest.New(t)
	origin.Linear(1)
	fx := gittest.Clone(t, origin)
	fx.Git("branch", "-m", "main", "local-work")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	if got, err := repo.InsightsTarget(context.Background()); err != nil || got != "origin/main" {
		t.Fatalf("target = %q, %v; want origin/main", got, err)
	}

	empty := OpenRepo(newTestRunner(t), gittest.New(t).Dir)
	if _, err := empty.InsightsTarget(context.Background()); err == nil {
		t.Error("a repo with no commits has no target")
	}
}
