package git

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func historyOf(list []Identity) []HistoryIdentity {
	out := make([]HistoryIdentity, len(list))
	for i, id := range list {
		out[i] = HistoryIdentity{Identity: id, Authored: 1}
	}
	return out
}

// onlyWorkTree reads a .mailmap text with no other source, as a repository with no
// mailmap.file or mailmap.blob does.
func onlyWorkTree(text string) *Mailmap {
	m := NewMailmap()
	m.Add(MailmapWorkTree, text)
	return m
}

var (
	sarah  = Identity{Name: "Sarah Lind", Email: "sarah@tidewater.example"}
	slind  = Identity{Name: "sarah lind", Email: "slind@users.noreply.example"}
	sam    = Identity{Name: "Sam Ortiz", Email: "sam@tidewater.example"}
	samToo = Identity{Name: "sam", Email: "sam@tidewater.example"}
	maya   = Identity{Name: "Maya Okafor", Email: "maya@tidewater.example"}
)

// gitAgrees saves the planned text as a repository's .mailmap and asks git who everyone
// is: each member must read as shown, and everyone else as they did before.
func gitAgrees(t *testing.T, before string, plan JoinPlan, everyone, members []Identity, shown Identity) {
	t.Helper()
	fx := gittest.New(t)
	fx.Linear(1)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	if before != "" {
		fx.WriteFile(".mailmap", before)
	}
	was, err := r.Mailmap(ctx, everyone)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Text == "" {
		fx.RemoveFile(".mailmap")
	} else {
		fx.WriteFile(".mailmap", plan.Text)
	}
	now, err := r.Mailmap(ctx, everyone)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range everyone {
		want := was[i]
		if slices.Contains(members, id) {
			want = shown
		}
		if now[i] != want {
			t.Errorf("git reads %s <%s> as %s <%s>, want %s <%s>", id.Name, id.Email, now[i].Name, now[i].Email, want.Name, want.Email)
		}
	}
}

func TestPlanJoinWritesTheShortestLine(t *testing.T) {
	t.Parallel()
	everyone := []Identity{sarah, slind, sam, samToo, maya}

	// Another address: the old address is the key, and the old name isn't written.
	members := []Identity{sarah, slind}
	plan, err := PlanJoin("", onlyWorkTree, historyOf(everyone), members, sarah)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>"}; !slices.Equal(plan.Lines, want) || plan.Text != want[0]+"\n" || !slices.Equal(plan.At, []int{1}) {
		t.Fatalf("plan = %+v", plan)
	}
	if strings.Contains(plan.Text, "sarah lind") || len(plan.Named) != 0 || len(plan.Blocked) != 0 {
		t.Fatalf("the old name was written, or something was named or blocked: %+v", plan)
	}
	gitAgrees(t, "", plan, everyone, members, sarah)

	// The same address under another name: one line with the name to show.
	members = []Identity{sam, samToo}
	plan, err = PlanJoin("", onlyWorkTree, historyOf(everyone), members, sam)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Sam Ortiz <sam@tidewater.example>"}; !slices.Equal(plan.Lines, want) {
		t.Fatalf("lines = %q, want %q", plan.Lines, want)
	}
	gitAgrees(t, "", plan, everyone, members, sam)

	// Shown under a name and address neither of them had: a line for each.
	shown := Identity{Name: "Sarah L. Lind", Email: "sarah.lind@tidewater.example"}
	members = []Identity{sarah, slind}
	plan, err = PlanJoin("", onlyWorkTree, historyOf(everyone), members, shown)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Lines) != 2 {
		t.Fatalf("lines = %q, want one for each address", plan.Lines)
	}
	gitAgrees(t, "", plan, everyone, members, shown)

	// Already one person: nothing to add, and the text is as it was.
	text := "Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>\n"
	plan, err = PlanJoin(text, onlyWorkTree, historyOf(everyone), []Identity{sarah, slind}, sarah)
	if err != nil || len(plan.Lines) != 0 || plan.Text != text {
		t.Fatalf("already joined: %+v, %v", plan, err)
	}
}

// Two people on one address: joining one of them mustn't take the other along, so the line
// has to name the commit's name. That's the only time an old name is written.
func TestPlanJoinNamesTheCommitOnlyWhenAnAddressIsShared(t *testing.T) {
	t.Parallel()
	pairing := Identity{Name: "sam", Email: "pair@tidewater.example"}
	other := Identity{Name: "Jo Whitlock", Email: "pair@tidewater.example"}
	everyone := []Identity{sam, pairing, other, maya}
	members := []Identity{sam, pairing}

	plan, err := PlanJoin("", onlyWorkTree, historyOf(everyone), members, sam)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Sam Ortiz <sam@tidewater.example> sam <pair@tidewater.example>"}; !slices.Equal(plan.Lines, want) {
		t.Fatalf("lines = %q, want %q", plan.Lines, want)
	}
	if !slices.Equal(plan.Named, []Identity{pairing}) {
		t.Fatalf("named = %+v, want the identity that shares an address", plan.Named)
	}
	gitAgrees(t, "", plan, everyone, members, sam)

	// A plain line for that address is already there for Jo: joining sam still needs the name.
	before := "Jo W. <jo@tidewater.example> <pair@tidewater.example>\n"
	plan, err = PlanJoin(before, onlyWorkTree, historyOf(everyone), members, sam)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Lines) != 1 || len(plan.Named) != 1 || !strings.HasPrefix(plan.Text, before) {
		t.Fatalf("plan = %+v", plan)
	}
	gitAgrees(t, before, plan, everyone, members, sam)
}

func TestPlanJoinPutsTheLineWhereItBelongs(t *testing.T) {
	t.Parallel()
	everyone := []Identity{sarah, slind, sam, samToo, maya}
	line := "Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>"
	join := func(text string) JoinPlan {
		t.Helper()
		plan, err := PlanJoin(text, onlyWorkTree, historyOf(everyone), []Identity{sarah, slind}, sarah)
		if err != nil {
			t.Fatal(err)
		}
		gitAgrees(t, text, plan, everyone, []Identity{sarah, slind}, sarah)
		return plan
	}

	// Beside the person's other lines, wherever those are, and nothing else moves.
	text := "# Tidewater\n" +
		"Sarah Lind <sarah@tidewater.example> <sarah@oldmill.example>\n" +
		"\n" +
		"# later\n" +
		"Priya Raman <priya@tidewater.example> <priya@oldmill.example>\n"
	plan := join(text)
	want := "# Tidewater\n" +
		"Sarah Lind <sarah@tidewater.example> <sarah@oldmill.example>\n" +
		line + "\n" +
		"\n" +
		"# later\n" +
		"Priya Raman <priya@tidewater.example> <priya@oldmill.example>\n"
	if plan.Text != want || !slices.Equal(plan.At, []int{3}) {
		t.Fatalf("beside her other line:\n%s\nat %v", plan.Text, plan.At)
	}

	// A file kept in order, with one line out of place among many: where the order puts it.
	sorted := "# Please keep this list sorted.\n" +
		"Ada <ada@x.example> <ada@old.example>\n" +
		"Bo <bo@x.example> <bo@old.example>\n" +
		"Cy <cy@x.example> <cy@old.example>\n" +
		"Dee <dee@x.example> <dee@old.example>\n" +
		"Eli <eli@x.example> <eli@old.example>\n" +
		"Fay <fay@x.example> <fay@old.example>\n" +
		"Gus <gus@x.example> <gus@old.example>\n" +
		"Hal <hal@x.example> <hal@old.example>\n" +
		"Ivy <ivy@x.example> <ivy@old.example>\n" +
		"Jo <jo@x.example> <jo@old.example>\n" +
		"Kit <kit@x.example> <kit@old.example>\n" +
		"Lou <lou@x.example> <lou@old.example>\n" +
		"Max <max@x.example> <max@old.example>\n" +
		"Ned <ned@x.example> <ned@old.example>\n" +
		"Oz <oz@x.example> <oz@old.example>\n" +
		"Pat <pat@x.example> <pat@old.example>\n" +
		"Quin <quin@x.example> <quin@old.example>\n" +
		"Rae <rae@x.example> <rae@old.example>\n" +
		"Tom <tom@x.example> <tom@old.example>\n" +
		"Sy <sy@x.example> <sy@old.example>\n" +
		"Uma <uma@x.example> <uma@old.example>\n" +
		"Val <val@x.example> <val@old.example>\n"
	plan = join(sorted)
	got := strings.Split(strings.TrimSuffix(plan.Text, "\n"), "\n")
	at := slices.Index(got, line)
	if at < 0 || got[at-1] != "Rae <rae@x.example> <rae@old.example>" || got[at+1] != "Tom <tom@x.example> <tom@old.example>" || plan.At[0] != at+1 {
		t.Fatalf("in a sorted file the line went to %d, after %q", at+1, got[max(at-1, 0)])
	}
	if len(got) != strings.Count(sorted, "\n")+1 || got[0] != "# Please keep this list sorted." {
		t.Fatal("other lines moved or were lost")
	}

	// A file in no order: at the end.
	unsorted := "Zed <zed@x.example> <zed@old.example>\nAda <ada@x.example> <ada@old.example>\n# end\n"
	plan = join(unsorted)
	if plan.Text != unsorted+line+"\n" || plan.At[0] != 4 {
		t.Fatalf("in an unsorted file:\n%s", plan.Text)
	}
}

// mailmap.file is read after the .mailmap, so its line for an address replaces a plain
// line here. A line that names the commit's name as well still counts, since git looks for
// one of those first, so that's what's planned. When the other file names the commit's
// name too, nothing in the .mailmap can win: the plan says so and writes nothing.
func TestPlanJoinSaysWhenAnotherMailmapDecides(t *testing.T) {
	t.Parallel()
	with := func(mine string) func(string) *Mailmap {
		return func(text string) *Mailmap {
			m := NewMailmap()
			m.Add(MailmapWorkTree, text)
			m.Add(MailmapFile, mine)
			return m
		}
	}
	history := historyOf([]Identity{sarah, slind})

	plan, err := PlanJoin("", with("Mine <mine@example.com> <slind@users.noreply.example>\n"), history, []Identity{sarah, slind}, sarah)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Sarah Lind <sarah@tidewater.example> sarah lind <slind@users.noreply.example>"}; !slices.Equal(plan.Lines, want) || !slices.Equal(plan.Named, []Identity{slind}) || len(plan.Blocked) != 0 {
		t.Fatalf("plan = %+v, want the line that names the commit's name", plan)
	}

	plan, err = PlanJoin("", with("Mine <mine@example.com> sarah lind <slind@users.noreply.example>\n"), history, []Identity{sarah, slind}, sarah)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Lines) != 0 || plan.Text != "" || !slices.Equal(plan.Blocked, []Identity{slind}) {
		t.Fatalf("plan = %+v, want nothing written and the identity blocked", plan)
	}
}

// The first case above, asked of git: a .mailmap line that names the commit's name wins
// over a plain line for the same address in mailmap.file.
func TestANamedLineWinsOverAPlainOneInMailmapFile(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	fx.WriteFile("mine.txt", "Mine <mine@example.com> <slind@users.noreply.example>\n")
	fx.Git("config", "mailmap.file", "mine.txt")
	fx.WriteFile(".mailmap", "Sarah Lind <sarah@tidewater.example> sarah lind <slind@users.noreply.example>\n")
	r := OpenRepo(newTestRunner(t), fx.Dir)
	got, err := r.Mailmap(context.Background(), []Identity{slind, {Name: "someone else", Email: slind.Email}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != sarah {
		t.Errorf("git reads the named identity as %+v, want Sarah Lind", got[0])
	}
	if got[1].Name != "Mine" {
		t.Errorf("git reads another name on that address as %+v, want mailmap.file's plain line", got[1])
	}
}

func TestPlanJoinNeedsSomeoneToShow(t *testing.T) {
	t.Parallel()
	for _, shown := range []Identity{{}, {Name: "Sarah Lind"}, {Email: "sarah@tidewater.example"}, {Name: "Sarah <Lind>", Email: "s@x"}, {Name: "Sarah", Email: "s@x\nEvil <e@x> <maya@tidewater.example>"}} {
		if _, err := PlanJoin("", onlyWorkTree, nil, []Identity{slind}, shown); !errors.Is(err, ErrJoinNeedsIdentity) {
			t.Errorf("shown as %+v: err = %v, want ErrJoinNeedsIdentity", shown, err)
		}
	}
}

func TestMailmapReaderReadsTheOtherSourcesOnce(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	fx.WriteFile("mine.txt", "My Sam <sam@tidewater.example>\n")
	fx.Git("config", "mailmap.file", "mine.txt")
	r := OpenRepo(newTestRunner(t), fx.Dir)
	read, err := r.MailmapReader(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Each text is read with the user's own file on top, as git would.
	if got, _ := read("").Resolve(samToo); got.Name != "My Sam" {
		t.Errorf("with no .mailmap text, sam = %+v", got)
	}
	if got, _ := read("Sarah Lind <sarah@tidewater.example> <slind@users.noreply.example>\n").Resolve(slind); got != sarah {
		t.Errorf("with text, slind = %+v", got)
	}
}

func TestKeepApart(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	if got := r.KeptApart(ctx); len(got) != 0 {
		t.Fatalf("a new repository keeps %v apart", got)
	}
	for _, key := range []string{"name:sarah lind", "address:pair@tidewater.example", "name:sarah lind"} {
		if err := r.KeepApart(ctx, key, true); err != nil {
			t.Fatal(err)
		}
	}
	if got := r.KeptApart(ctx); !slices.Equal(got, []string{"name:sarah lind", "address:pair@tidewater.example"}) {
		t.Fatalf("kept apart = %q, each once", got)
	}
	// It's the repository's own setting, in its git config.
	if out := fx.Git("config", "--local", "--get-all", "gitrevenge.apart"); !strings.Contains(out, "name:sarah lind") {
		t.Fatalf("git config has %q", out)
	}
	if err := r.KeepApart(ctx, "name:sarah lind", false); err != nil {
		t.Fatal(err)
	}
	if err := r.KeepApart(ctx, "name:never there", false); err != nil {
		t.Fatal(err)
	}
	if got := r.KeptApart(ctx); !slices.Equal(got, []string{"address:pair@tidewater.example"}) {
		t.Fatalf("after forgetting one: %q", got)
	}
	if err := r.KeepApart(ctx, " ", true); err == nil {
		t.Fatal("a blank key was accepted")
	}
}
