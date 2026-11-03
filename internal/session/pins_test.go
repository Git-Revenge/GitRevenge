package session

import (
	"context"
	"testing"

	"github.com/risadams/gitrevenge/internal/branchkind"
	"github.com/risadams/gitrevenge/internal/gittest"
)

// tipKindsOf opens the fixture and returns the kind of each kinded tip.
func tipKindsOf(t *testing.T, fx *gittest.Repo, rules branchkind.Rules, remotes ...string) map[string]string {
	t.Helper()
	m, _ := newManager(t)
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	_, kinds := KindTips(context.Background(), r.Git, rules, remotes, false)
	return kinds
}

// tipOffTrunk makes a commit no branch points at yet, away from the fixture's main branch
// (which would otherwise win as Trunk), with these branches on it.
func tipOffTrunk(fx *gittest.Repo, branches ...string) string {
	fx.Linear(1)
	fx.Git("checkout", "-q", "--detach")
	head := fx.CommitFile("work.txt", "work", "work")
	for _, b := range branches {
		fx.Git("branch", b)
	}
	return head
}

func TestSharedTipTakesTheKindEarliestInTheOrder(t *testing.T) {
	cases := []struct {
		name     string
		branches []string
		want     branchkind.Kind
	}{
		{"release beats feature, though feature sorts first", []string{"feature/a", "release/1"}, branchkind.Release},
		{"release beats a later ordinary name", []string{"release/1", "zzz"}, branchkind.Release},
		{"environment beats bot, though bot sorts first", []string{"dependabot/a", "staging"}, branchkind.Environment},
		{"bot beats feature, though feature sorts last", []string{"dependabot/a", "feature/z"}, branchkind.Bot},
	}
	rules := branchkind.New(map[branchkind.Kind][]string{branchkind.Environment: {"staging"}}, nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := gittest.New(t)
			head := tipOffTrunk(fx, tc.branches...)
			if got := tipKindsOf(t, fx, rules)[head]; got != string(tc.want) {
				t.Errorf("kind = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSharedTipIgnoresWhetherABranchIsLocal(t *testing.T) {
	up := gittest.New(t)
	head := tipOffTrunk(up, "release/1")
	fx := gittest.Clone(t, up)
	fx.Git("branch", "feature/a", head)
	if got := tipKindsOf(t, fx, branchkind.New(nil, nil), "origin")[head]; got != string(branchkind.Release) {
		t.Errorf("kind = %q, want the remote's Release over the local Feature", got)
	}
}

func TestDocsTipOnItsOwnStaysDocs(t *testing.T) {
	fx := gittest.New(t)
	head := tipOffTrunk(fx, "gh-pages")
	if got := tipKindsOf(t, fx, branchkind.New(nil, nil))[head]; got != string(branchkind.Docs) {
		t.Errorf("kind = %q, want docs", got)
	}
}

func TestDocsTipSharedWithOrdinaryBranchIsNotMuted(t *testing.T) {
	fx := gittest.New(t)
	head := tipOffTrunk(fx, "gh-pages", "topic")
	if got, ok := tipKindsOf(t, fx, branchkind.New(nil, nil))[head]; ok {
		t.Errorf("kind = %q, want none", got)
	}
}

func TestDocsTipSharedWithFeatureBranchTakesFeature(t *testing.T) {
	fx := gittest.New(t)
	head := tipOffTrunk(fx, "gh-pages", "feature/a")
	if got := tipKindsOf(t, fx, branchkind.New(nil, nil))[head]; got != string(branchkind.Feature) {
		t.Errorf("kind = %q, want feature", got)
	}
}

func TestDocsTipIgnoresAHiddenBranch(t *testing.T) {
	fx := gittest.New(t)
	head := tipOffTrunk(fx, "gh-pages", "secret")
	if got := tipKindsOf(t, fx, branchkind.New(nil, []string{"secret"}))[head]; got != string(branchkind.Docs) {
		t.Errorf("kind = %q, want docs: a hidden branch isn't on show", got)
	}
}

func TestDocsTipIgnoresRemoteHead(t *testing.T) {
	up := gittest.New(t)
	head := tipOffTrunk(up, "gh-pages")
	up.Checkout("gh-pages")
	// The clone has origin/HEAD pointing here too.
	fx := gittest.Clone(t, up)
	if got := tipKindsOf(t, fx, branchkind.New(nil, nil), "origin")[head]; got != string(branchkind.Docs) {
		t.Errorf("kind = %q, want docs: origin/HEAD isn't real work", got)
	}
}
