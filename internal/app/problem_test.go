package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// What git printed when GitHub refused a push to a branch that needs a pull request (#614).
const ruleRefusal = "Enumerating objects: 8, done.\n" +
	"Counting objects:  12% (1/8)\rCounting objects:  25% (2/8)\rCounting objects: 100% (8/8), done.\n" +
	"Delta compression using up to 16 threads\n" +
	"Compressing objects: 100% (1/1)\rCompressing objects: 100% (1/1), done.\n" +
	"Writing objects:  25% (1/4)\rWriting objects: 100% (4/4), 1.07 KiB | 1.07 MiB/s, done.\n" +
	"Total 4 (delta 3), reused 0 (delta 0), pack-reused 0 (from 0)\n" +
	"remote: Resolving deltas:   0% (0/3)        \rremote: Resolving deltas: 100% (3/3), completed with 3 local objects.        \n" +
	"remote: error: GH013: Repository rule violations found for refs/heads/main.        \n" +
	"remote: Review all repository rules at https://github.com/Git-Revenge/GitRevenge/rules?ref=refs%2Fheads%2Fmain        \n" +
	"remote: \n" +
	"remote: - Changes must be made through a pull request.        \n" +
	"remote: \n" +
	"To github.com:Git-Revenge/GitRevenge.git\n" +
	" ! [remote rejected] main -> main (push declined due to repository rule violations)\n" +
	"error: failed to push some refs to 'github.com:Git-Revenge/GitRevenge.git'\n"

func problemOf(t *testing.T, op, stderr string) *Problem {
	t.Helper()
	var pe *problemErr
	if err := netProblem(op, &git.Error{Stderr: stderr}); !errors.As(err, &pe) {
		t.Fatalf("netProblem = %v (%T), want a Problem", err, err)
	}
	return pe.p
}

func TestRuleRefusalIsShortAndSaysWhatToDo(t *testing.T) {
	p := problemOf(t, "push", ruleRefusal)
	if want := "Push refused: main only takes changes through a pull request"; p.Title != want {
		t.Errorf("Title = %q, want %q", p.Title, want)
	}
	if want := "github.com turned the push down. Nothing was pushed, and your commits are still on your main."; p.Detail != want {
		t.Errorf("Detail = %q, want %q", p.Detail, want)
	}
	want := []ProblemAction{
		{Kind: "push-new-branch", Label: "Push to a new branch…"},
		{Kind: "open-url", Label: "See the rules", URL: "https://github.com/Git-Revenge/GitRevenge/rules?ref=refs%2Fheads%2Fmain"},
	}
	if len(p.Actions) != len(want) || p.Actions[0] != want[0] || p.Actions[1] != want[1] {
		t.Errorf("Actions = %+v, want %+v", p.Actions, want)
	}
	wantOutput := "remote: error: GH013: Repository rule violations found for refs/heads/main.\n" +
		"remote: Review all repository rules at https://github.com/Git-Revenge/GitRevenge/rules?ref=refs%2Fheads%2Fmain\n" +
		"remote: - Changes must be made through a pull request.\n" +
		"To github.com:Git-Revenge/GitRevenge.git\n" +
		" ! [remote rejected] main -> main (push declined due to repository rule violations)\n" +
		"error: failed to push some refs to 'github.com:Git-Revenge/GitRevenge.git'"
	if p.Output != wantOutput {
		t.Errorf("Output =\n%s\nwant\n%s", p.Output, wantOutput)
	}
	if p.Hidden != 7 {
		t.Errorf("Hidden = %d, want 7", p.Hidden)
	}
	if !strings.Contains(p.Full, "Counting objects") {
		t.Error("Full lost git's progress lines; it's for copying everything")
	}
	if got := (&problemErr{p}).Error(); strings.Contains(got, "Counting") || strings.Count(got, "\n") != 1 {
		t.Errorf("Error() = %q, want the title and detail only", got)
	}
}

func TestOtherRefusalsByTheServer(t *testing.T) {
	const to = "To https://gitlab.example.com/team/app.git\n"
	hook := problemOf(t, "push", to+" ! [remote rejected] main -> main (pre-receive hook declined)\nerror: failed to push some refs\n")
	if want := "Push refused: gitlab.example.com didn't accept main"; hook.Title != want {
		t.Errorf("Title = %q, want %q", hook.Title, want)
	}
	if want := "It said: pre-receive hook declined. Nothing was pushed."; hook.Detail != want {
		t.Errorf("Detail = %q, want %q", hook.Detail, want)
	}
	if len(hook.Actions) != 1 || hook.Actions[0].Kind != "push-new-branch" {
		t.Errorf("Actions = %+v, want only push-new-branch", hook.Actions)
	}

	rules := problemOf(t, "push", "remote: - Commits must have verified signatures.\nremote: - Required status check \"ci\" is expected.\n"+to+" ! [remote rejected] topic -> release (push declined due to repository rule violations)\n")
	if want := "Push refused: gitlab.example.com's rules for release don't allow it"; rules.Title != want {
		t.Errorf("Title = %q, want %q", rules.Title, want)
	}
	if want := "Commits must have verified signatures. Required status check \"ci\" is expected. Nothing was pushed."; rules.Detail != want {
		t.Errorf("Detail = %q, want %q", rules.Detail, want)
	}
}

func TestNetProblemTitles(t *testing.T) {
	for _, c := range []struct {
		op, stderr, title string
		actions           []string
	}{
		{"fetch", "fatal: Authentication failed for 'https://github.com/a/b.git/'", "Fetch failed: github.com didn't accept your sign-in", []string{"retry"}},
		{"pull", "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.", "Pull failed: github.com refused your SSH key", []string{"retry"}},
		{"fetch", "fatal: unable to access 'https://github.com/a/b.git/': Could not resolve host: github.com", "Fetch failed: couldn't reach github.com", []string{"retry"}},
		{"push", "To github.com:a/b.git\n ! [rejected]        main -> main (fetch first)\nerror: failed to push some refs to 'github.com:a/b.git'", "Push refused: github.com has commits you don't have", []string{"pull-then-push", "force-push"}},
		{"push", "To /tmp/remote.git\n ! [rejected]        main -> main (non-fast-forward)", "Push refused: the remote has commits you don't have", []string{"pull-then-push", "force-push"}},
		{"clone", "remote: Repository not found.\nfatal: repository 'https://github.com/a/missing.git/' not found", "Clone failed: no repository was found at that URL", nil},
		// No operation named: the reason stands alone, and a host keeps its own spelling.
		{"", "fatal: Authentication failed for 'https://github.com/a/b.git/'", "github.com didn't accept your sign-in", []string{"retry"}},
		{"", "fatal: unable to access 'https://example.org/a.git/': Could not resolve host: example.org", "Couldn't reach example.org", []string{"retry"}},
	} {
		p := problemOf(t, c.op, c.stderr)
		if p.Title != c.title {
			t.Errorf("%s: Title = %q, want %q", c.stderr, p.Title, c.title)
		}
		var kinds []string
		for _, a := range p.Actions {
			kinds = append(kinds, a.Kind)
		}
		if strings.Join(kinds, ",") != strings.Join(c.actions, ",") {
			t.Errorf("%s: actions = %v, want %v", c.title, kinds, c.actions)
		}
		if p.Detail == "" {
			t.Errorf("%s: no Detail", c.title)
		}
	}
}

func TestUnknownFailureQuotesGitsLastWord(t *testing.T) {
	p := problemOf(t, "pull", "remote: Counting objects: 100% (3/3), done.\nFrom github.com:a/b\n * branch            main       -> FETCH_HEAD\nfatal: refusing to merge unrelated histories\n")
	if p.Title != "Pull didn't finish" || p.Quote != "fatal: refusing to merge unrelated histories" || p.Detail != "" {
		t.Errorf("got Title %q, Quote %q, Detail %q", p.Title, p.Quote, p.Detail)
	}
	if len(p.Actions) != 0 {
		t.Errorf("Actions = %+v, want none", p.Actions)
	}

	// Without an operation to name there's no title to build, so git's message stands.
	err := netProblem("", &git.Error{Stderr: "fatal: refusing to merge unrelated histories\n"})
	var asProblem *problemErr
	if errors.As(err, &asProblem) || err.Error() != "refusing to merge unrelated histories" {
		t.Errorf("err = %v (%T), want git's message as text", err, err)
	}
}

func TestTidyOutputDropsOnlyProgress(t *testing.T) {
	kept, hidden := tidyOutput("Receiving objects:  50% (1/2)\rReceiving objects: 100% (2/2), done.\nerror: something real\nremote: \nhint: a hint\n")
	if kept != "error: something real\nhint: a hint" || hidden != 1 {
		t.Errorf("kept = %q, hidden = %d", kept, hidden)
	}
}

func TestTakeProblemHandsItOutOnceByItsText(t *testing.T) {
	a := New()
	failed := netProblem("push", &git.Error{Stderr: ruleRefusal})
	if err := a.remember("/repo", failed); !errors.Is(err, failed) {
		t.Fatalf("remember changed the error: %v", err)
	}
	p := problemOf(t, "push", ruleRefusal)
	// Wails rejects a call whose result implements error, so what TakeProblem returns must not.
	if _, isErr := any(p).(error); isErr {
		t.Fatal("Problem implements error: Wails would turn TakeProblem's answer into a failure")
	}
	if got := a.TakeProblem("some other error"); got != nil {
		t.Errorf("TakeProblem for another message = %+v, want nil", got)
	}
	got := a.TakeProblem(failed.Error())
	if got == nil || got.Path != "/repo" || got.Title != p.Title {
		t.Fatalf("TakeProblem = %+v", got)
	}
	if again := a.TakeProblem(failed.Error()); again != nil {
		t.Error("TakeProblem handed the same problem out twice")
	}
	// Plain errors aren't kept.
	_ = a.remember("/repo", userError("Cancelled."))
	if got := a.TakeProblem("Cancelled."); got != nil {
		t.Errorf("TakeProblem for a plain error = %+v, want nil", got)
	}
}

func TestPlainErrIsOneLine(t *testing.T) {
	p := netProblem("", &git.Error{Stderr: "fatal: unable to access 'https://example.org/a.git/': Could not resolve host: example.org"})
	if got, want := plainErr(p), "Couldn't reach example.org. Check the URL and your network connection. Your repository is as it was."; got != want {
		t.Errorf("plainErr = %q, want %q", got, want)
	}
	if got := plainErr(userError("Cancelled.")); got != "Cancelled." {
		t.Errorf("plainErr of text = %q", got)
	}
}
