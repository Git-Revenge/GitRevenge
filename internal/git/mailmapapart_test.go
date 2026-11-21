package git

import (
	"context"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// gitReads saves text as a repository's .mailmap and asks git who everyone is.
func gitReads(t *testing.T, text string, everyone []Identity) []Identity {
	t.Helper()
	fx := gittest.New(t)
	fx.Linear(1)
	if text != "" {
		fx.WriteFile(".mailmap", text)
	}
	got, err := OpenRepo(newTestRunner(t), fx.Dir).Mailmap(context.Background(), everyone)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// apartAsGitSeesIt checks a plan against git: the one taken apart reads as wantApart, and
// everyone else reads exactly as they did before.
func apartAsGitSeesIt(t *testing.T, before string, plan ApartPlan, everyone []Identity, who, wantApart Identity) {
	t.Helper()
	was := gitReads(t, before, everyone)
	now := gitReads(t, plan.Text, everyone)
	for i, id := range everyone {
		want := was[i]
		if id == who {
			want = wantApart
		}
		if slices.Contains(plan.Lost, id) {
			continue // said to come apart too
		}
		if now[i] != want {
			t.Errorf("git reads %s <%s> as %s <%s>, want %s <%s>", id.Name, id.Email, now[i].Name, now[i].Email, want.Name, want.Email)
		}
	}
}

func TestPlanApartRemovesTheLineThatJoinedOnlyThem(t *testing.T) {
	t.Parallel()
	everyone := []Identity{sarah, slind, sam, samToo, maya}
	text := "# Tidewater\n" +
		"Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>\n" +
		"Sam Ortiz <sam@tidewater.example>\n"

	plan := PlanApart(text, onlyWorkTree, historyOf(everyone), slind)
	if want := "# Tidewater\nSam Ortiz <sam@tidewater.example>\n"; plan.Text != want || !slices.Equal(plan.Removed, []int{2}) {
		t.Fatalf("plan = %+v", plan)
	}
	if len(plan.Added) != 0 || len(plan.Kept) != 0 || len(plan.Lost) != 0 {
		t.Fatalf("nobody else was on that line, and the plan says %+v", plan)
	}
	apartAsGitSeesIt(t, text, plan, everyone, slind, slind)

	// Someone nothing joins: nothing to do, and the text is as it was.
	plan = PlanApart(text, onlyWorkTree, historyOf(everyone), maya)
	if plan.Text != text || len(plan.Removed) != 0 {
		t.Fatalf("an identity no line joins: %+v", plan)
	}
}

// One line for an address joins every name under it. Taking one name apart mustn't take
// the others: the line is replaced, where it was, by lines that name each of the others.
func TestPlanApartKeepsTheOthersALineJoined(t *testing.T) {
	t.Parallel()
	one := Identity{Name: "sam", Email: "pair@tidewater.example"}
	two := Identity{Name: "S. Ortiz", Email: "pair@tidewater.example"}
	three := Identity{Name: "Samuel", Email: "pair@tidewater.example"}
	everyone := []Identity{sam, one, two, three, maya}
	text := "# before\n" +
		"Sam Ortiz <sam@tidewater.example> <pair@tidewater.example>\n" +
		"# after\n" +
		"Maya O. <maya@tidewater.example>\n"

	plan := PlanApart(text, onlyWorkTree, historyOf(everyone), one)
	want := "# before\n" +
		"Sam Ortiz <sam@tidewater.example> S. Ortiz <pair@tidewater.example>\n" +
		"Sam Ortiz <sam@tidewater.example> Samuel <pair@tidewater.example>\n" +
		"# after\n" +
		"Maya O. <maya@tidewater.example>\n"
	if plan.Text != want {
		t.Fatalf("text =\n%s\nwant\n%s", plan.Text, want)
	}
	if !slices.Equal(plan.Removed, []int{2}) || len(plan.Added) != 2 || !slices.Equal(plan.Kept, []Identity{two, three}) || len(plan.Lost) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	apartAsGitSeesIt(t, text, plan, everyone, one, one)
}

// A name from one line and an address from another: both go, and what else they joined stays.
func TestPlanApartWhenTwoLinesDecide(t *testing.T) {
	t.Parallel()
	old := Identity{Name: "jdoe", Email: "jdoe@old.example"}
	also := Identity{Name: "J Doe", Email: "jdoe@old.example"}
	everyone := []Identity{old, also, maya}
	text := "Jane Doe <jdoe@old.example>\n<jane@tidewater.example> <jdoe@old.example>\n"
	jane := Identity{Name: "Jane Doe", Email: "jane@tidewater.example"}
	if got := gitReads(t, text, everyone); got[0] != jane || got[1] != jane {
		t.Fatalf("before: git reads them as %+v", got[:2])
	}

	plan := PlanApart(text, onlyWorkTree, historyOf(everyone), old)
	if !slices.Equal(plan.Removed, []int{1, 2}) || !slices.Equal(plan.Kept, []Identity{also}) {
		t.Fatalf("plan = %+v", plan)
	}
	if want := "Jane Doe <jane@tidewater.example> J Doe <jdoe@old.example>\n"; plan.Text != want {
		t.Fatalf("text = %q, want %q", plan.Text, want)
	}
	apartAsGitSeesIt(t, text, plan, everyone, old, old)
}

// A line that already names the commit joins only that one: it just goes.
func TestPlanApartRemovesANamedLine(t *testing.T) {
	t.Parallel()
	one := Identity{Name: "sam", Email: "pair@tidewater.example"}
	other := Identity{Name: "Jo Whitlock", Email: "pair@tidewater.example"}
	everyone := []Identity{sam, one, other}
	text := "Jo W. <jo@tidewater.example> <pair@tidewater.example>\nSam Ortiz <sam@tidewater.example> sam <pair@tidewater.example>\n"

	plan := PlanApart(text, onlyWorkTree, historyOf(everyone), one)
	if plan.Text != "Jo W. <jo@tidewater.example> <pair@tidewater.example>\n" || len(plan.Added) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	// With its own line gone, the plain line for the address reads it, as git would.
	apartAsGitSeesIt(t, text, plan, everyone, one, Identity{Name: "Jo W.", Email: "jo@tidewater.example"})
}

// Someone with no name can't be told apart by one, so no line can keep them: the plan says
// they come apart too, and doesn't pretend otherwise.
func TestPlanApartSaysWhoComesApartToo(t *testing.T) {
	t.Parallel()
	one := Identity{Name: "sam", Email: "pair@tidewater.example"}
	nameless := Identity{Name: "", Email: "pair@tidewater.example"}
	text := "Sam Ortiz <sam@tidewater.example> <pair@tidewater.example>\n"

	plan := PlanApart(text, onlyWorkTree, historyOf([]Identity{sam, one, nameless}), one)
	if plan.Text != "" || !slices.Equal(plan.Lost, []Identity{nameless}) || len(plan.Kept) != 0 {
		t.Fatalf("plan = %+v, want the nameless identity said to come apart", plan)
	}
}

// Lines in mailmap.file aren't this file's to remove.
func TestPlanApartLeavesOtherSourcesAlone(t *testing.T) {
	t.Parallel()
	withMine := func(text string) *Mailmap {
		m := NewMailmap()
		m.Add(MailmapWorkTree, text)
		m.Add(MailmapFile, "Mine <mine@example.com> <slind@users.noreply.example>\n")
		return m
	}
	text := "Maya O. <maya@tidewater.example>\n"
	plan := PlanApart(text, withMine, historyOf([]Identity{slind, maya}), slind)
	if plan.Text != text || len(plan.Removed) != 0 {
		t.Fatalf("plan = %+v, want nothing removed", plan)
	}
}
