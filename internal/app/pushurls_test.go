package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestPushURLsFollowAndStayTidy(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Commit("one")
	fx.Git("remote", "add", "origin", "https://github.com/me/r.git")
	fx.Git("remote", "add", "mirror", "https://gitlab.com/me/r.git")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	pushURLs := func(remote string) []string {
		// Not --get-all: the fixture fails a test on git's exit 1 for a key that isn't set.
		var urls []string
		for line := range strings.SplitSeq(fx.Git("config", "--local", "--list"), "\n") {
			if u, ok := strings.CutPrefix(line, "remote."+remote+".pushurl="); ok {
				urls = append(urls, u)
			}
		}
		return urls
	}

	// Also push to mirror: git needs origin's own URL written too.
	if err := a.SetRemotePushURLs(tab.Path, "origin", []string{"https://github.com/me/r.git", "https://gitlab.com/me/r.git"}); err != nil {
		t.Fatal(err)
	}
	if got := pushURLs("origin"); !slices.Equal(got, []string{"https://github.com/me/r.git", "https://gitlab.com/me/r.git"}) {
		t.Fatalf("pushurls = %v", got)
	}
	rs, _ := a.Remotes(tab.Path)
	for _, r := range rs {
		if r.Name == "origin" && (len(r.Pushes) != 2 || r.Pushes[1].Remote != "mirror") {
			t.Fatalf("origin's pushes = %+v", r.Pushes)
		}
	}

	// Moving mirror moves the push URL that follows it; moving origin moves its own.
	if err := a.RemoteAction(tab.Path, "seturl", []string{"mirror", "https://codeberg.org/me/r.git"}); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoteAction(tab.Path, "seturl", []string{"origin", "git@github.com:me/r.git"}); err != nil {
		t.Fatal(err)
	}
	if got := pushURLs("origin"); !slices.Equal(got, []string{"git@github.com:me/r.git", "https://codeberg.org/me/r.git"}) {
		t.Fatalf("after moving both: %v", got)
	}

	// Back to its own URL alone: no pushurl is left behind.
	if err := a.SetRemotePushURLs(tab.Path, "origin", []string{"git@github.com:me/r.git"}); err != nil {
		t.Fatal(err)
	}
	if got := pushURLs("origin"); got != nil {
		t.Fatalf("stray pushurls: %v", got)
	}
}
