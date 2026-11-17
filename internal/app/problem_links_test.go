package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// The "See the rules" button opens an address the server printed. It's only offered when the
// address is on a server the repository has as a remote.
func TestRulesLinkMustLeadToARemote(t *testing.T) {
	t.Parallel()
	refusal := func(link string) error {
		return netProblem("push", &git.Error{Stderr: strings.Replace(ruleRefusal, "https://github.com/Git-Revenge/GitRevenge/rules?ref=refs%2Fheads%2Fmain", link, 1)})
	}
	kinds := func(err error) string {
		var names []string
		for _, a := range problemOf2(t, err).Actions {
			names = append(names, a.Kind)
		}
		return strings.Join(names, " ")
	}
	for _, c := range []struct {
		name, link string
		hosts      []string
		want       string
	}{
		{"the remote's own host", "https://github.com/o/r/rules", []string{"github.com"}, "push-new-branch open-url"},
		{"host in another case", "https://GitHub.com/o/r/rules", []string{"github.com"}, "push-new-branch open-url"},
		{"one of several remotes", "https://git.example.com/rules", []string{"github.com", "git.example.com"}, "push-new-branch open-url"},
		{"another host", "https://attacker.example/login", []string{"github.com"}, "push-new-branch"},
		{"the host as a user name", "https://github.com@attacker.example/login", []string{"github.com"}, "push-new-branch"},
		{"the host as a subdomain", "https://github.com.attacker.example/login", []string{"github.com"}, "push-new-branch"},
		{"no remote with a host", "https://github.com/o/r/rules", nil, "push-new-branch"},
	} {
		asked := 0
		err := ownLinksOnly(refusal(c.link), func() []string { asked++; return c.hosts })
		if got := kinds(err); got != c.want {
			t.Errorf("%s: actions = %q, want %q", c.name, got, c.want)
		}
		if asked != 1 {
			t.Errorf("%s: asked for the hosts %d times, want 1", c.name, asked)
		}
	}

	// Nothing to check: the remotes aren't read.
	plain := netProblem("push", &git.Error{Stderr: "fatal: Authentication failed for 'https://github.com/a/b.git/'"})
	if got := ownLinksOnly(plain, func() []string { t.Error("read the remotes with no link to check"); return nil }); !errors.Is(got, plain) {
		t.Errorf("err = %v, want it as it came", got)
	}
	if got := ownLinksOnly(nil, func() []string { return nil }); got != nil {
		t.Errorf("err = %v, want nil", got)
	}
}

func problemOf2(t *testing.T, err error) *Problem {
	t.Helper()
	var pe *problemErr
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v (%T), want a Problem", err, err)
	}
	return pe.p
}

// End to end: a server whose hook prints a rules link to somewhere else. The push is still
// explained, but the link isn't a button.
func TestPushHookCannotOfferALinkElsewhere(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	hook := "#!/bin/sh\n" +
		"echo 'Review all repository rules at https://attacker.example/login'\n" +
		"echo '- Changes must be made through a pull request.'\n" +
		"exit 1\n"
	hooks := filepath.Join(remote.Dir, "hooks")
	if _, err := os.Stat(hooks); err != nil {
		hooks = filepath.Join(remote.Dir, ".git", "hooks")
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-receive"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	tab, _ := a.OpenRepo(local.Dir)
	local.CommitFile("l.txt", "l", "mine")

	err := a.PushCurrent(tab.Path, false)
	if err == nil || !strings.Contains(err.Error(), "Push refused") {
		t.Fatalf("err = %v, want a refused push", err)
	}
	p := a.TakeProblem(err.Error())
	if p == nil {
		t.Fatal("no Problem kept for the refused push")
	}
	if !strings.Contains(p.Full, "https://attacker.example/login") {
		t.Fatalf("the hook's link never arrived, so this checks nothing:\n%s", p.Full)
	}
	for _, act := range p.Actions {
		if act.Kind == "open-url" {
			t.Errorf("offered %q as a button; it isn't on any of the repository's remotes", act.URL)
		}
	}
}
