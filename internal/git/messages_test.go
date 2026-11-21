package git

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func messagesOf(t *testing.T, r *Repo, m MessageRange) (ids, messages []string) {
	t.Helper()
	err := r.CommitMessages(context.Background(), m, func(id, message string) error {
		ids = append(ids, id)
		messages = append(messages, message)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return ids, messages
}

func TestCommitMessagesReadsEveryMessageAsWritten(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	one := fx.CommitFile("a.txt", "a", "One")
	body := "Two\n\nA body, with a blank line in it.\n\n  And an indented one.\n\nCloses #12\nCo-authored-by: Maya <maya@tidewater.example>"
	fx.WriteFile("b.txt", "b")
	fx.Git("add", "-A")
	fx.Git("commit", "-q", "--cleanup=verbatim", "-m", body)
	two := fx.Head()
	r := OpenRepo(newTestRunner(t), fx.Dir)

	tips, err := r.HistoryTips(context.Background())
	if err != nil || len(tips) != 1 || tips[0] != two {
		t.Fatalf("tips = %v, %v, want only %s", tips, err, two)
	}
	ids, messages := messagesOf(t, r, MessageRange{Tips: tips})
	if !slices.Equal(ids, []string{two, one}) {
		t.Fatalf("ids = %v, want newest first", ids)
	}
	if strings.TrimRight(messages[0], "\n") != body || strings.TrimRight(messages[1], "\n") != "One" {
		t.Fatalf("messages = %q", messages)
	}
	if n, err := r.CountMessages(context.Background(), MessageRange{Tips: tips}); err != nil || n != 2 {
		t.Fatalf("count = %d, %v", n, err)
	}
}

// What was read before is left out by Not, and a read cut short carries on with Skip.
func TestCommitMessagesLeavesOutWhatIsKnownAndSkips(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	old := fx.Linear(3)
	fx.Branch("topic")
	fx.Checkout("topic")
	side := fx.CommitFile("s.txt", "s", "On a branch")
	fx.Checkout("main")
	top := fx.CommitFile("t.txt", "t", "On main")
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	tips, err := r.HistoryTips(ctx)
	if err != nil || len(tips) != 2 {
		t.Fatalf("tips = %v, %v", tips, err)
	}
	ids, _ := messagesOf(t, r, MessageRange{Tips: tips, Not: []string{old[2]}})
	slices.Sort(ids)
	want := []string{side, top}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Fatalf("with the old tip left out: %v, want %v", ids, want)
	}
	if n, _ := r.CountMessages(ctx, MessageRange{Tips: tips, Not: []string{old[2]}}); n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}

	all, _ := messagesOf(t, r, MessageRange{Tips: tips})
	rest, _ := messagesOf(t, r, MessageRange{Tips: tips, Skip: 2})
	if len(all) != 5 || !slices.Equal(rest, all[2:]) {
		t.Fatalf("skipping two gave %v, want the rest of %v", rest, all)
	}
}

func TestCommitMessagesStopsWhenAsked(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(5)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	tips, _ := r.HistoryTips(context.Background())
	stop := errors.New("enough")
	n := 0
	err := r.CommitMessages(context.Background(), MessageRange{Tips: tips}, func(string, string) error {
		n++
		if n == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || n != 2 {
		t.Fatalf("err = %v after %d, want to stop at 2", err, n)
	}
}

func TestMessageRangeTakesOnlyObjectIDs(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	r := OpenRepo(newTestRunner(t), fx.Dir)
	err := r.CommitMessages(context.Background(), MessageRange{Tips: []string{"--all"}}, func(string, string) error { return nil })
	if err == nil {
		t.Fatal("an option was taken as a tip")
	}
	// No tips, no commits, and no git.
	if err := r.CommitMessages(context.Background(), MessageRange{}, func(string, string) error { t.Fatal("read a commit"); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestMissingObjects(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	head := fx.Linear(1)[0]
	r := OpenRepo(newTestRunner(t), fx.Dir)
	gone := strings.Repeat("0", len(head)-1) + "1"
	missing, err := r.MissingObjects(context.Background(), []string{head, gone})
	if err != nil || !slices.Equal(missing, []string{gone}) {
		t.Fatalf("missing = %v, %v, want only %s", missing, err, gone)
	}
}
