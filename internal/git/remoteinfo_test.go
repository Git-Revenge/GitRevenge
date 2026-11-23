package git

import (
	"context"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestParseLocation(t *testing.T) {
	t.Parallel()
	cases := map[string]Location{
		// Forges known by host, in every form.
		"https://github.com/Git-Revenge/GitRevenge.git":  {PlaceWeb, ForgeGitHub, "github.com", "", "Git-Revenge/GitRevenge"},
		"git@github.com:Git-Revenge/GitRevenge.git":      {PlaceServer, ForgeGitHub, "github.com", "", "Git-Revenge/GitRevenge"},
		"ssh://git@ssh.github.com:443/o/r.git":           {PlaceServer, ForgeGitHub, "github.com", "", "o/r"},
		"https://user:token@GitHub.com/o/r/":             {PlaceWeb, ForgeGitHub, "github.com", "", "o/r"},
		"https://gitlab.com/group/sub/proj.git":          {PlaceWeb, ForgeGitLab, "gitlab.com", "", "group/sub/proj"},
		"ssh://git@altssh.gitlab.com:443/group/proj.git": {PlaceServer, ForgeGitLab, "gitlab.com", "", "group/proj"},
		"https://gitea.com/o/r":                          {PlaceWeb, ForgeGitea, "gitea.com", "", "o/r"},
		"git@codeberg.org:o/r.git":                       {PlaceServer, ForgeCodeberg, "codeberg.org", "", "o/r"},
		"git@bitbucket.org:team/r.git":                   {PlaceServer, ForgeBitbucket, "bitbucket.org", "", "team/r"},
		"https://dev.azure.com/org/proj/_git/repo":       {PlaceWeb, ForgeAzure, "dev.azure.com", "", "org/proj/_git/repo"},
		"https://org@dev.azure.com/org/proj/_git/repo":   {PlaceWeb, ForgeAzure, "dev.azure.com", "", "org/proj/_git/repo"},
		"git@ssh.dev.azure.com:v3/org/proj/repo":         {PlaceServer, ForgeAzure, "dev.azure.com", "", "org/proj/_git/repo"},
		"https://org.visualstudio.com/proj/_git/repo":    {PlaceWeb, ForgeAzure, "org.visualstudio.com", "", "proj/_git/repo"},
		"org@vs-ssh.visualstudio.com:v3/Org/proj/repo":   {PlaceServer, ForgeAzure, "org.visualstudio.com", "", "proj/_git/repo"},
		// A self-hosted forge is never guessed, whatever its name says.
		"https://gitlab.example.com:8443/team/app.git":  {PlaceWeb, ForgeNone, "gitlab.example.com", "8443", "team/app"},
		"ssh://git@git.example.com:2222/~ris/tidewater": {PlaceServer, ForgeNone, "git.example.com", "", "~ris/tidewater"},
		"git@nas.local:srv/git/tidewater.git":           {PlaceServer, ForgeNone, "nas.local", "", "srv/git/tidewater"},
		"git://git.kernel.org/pub/scm/git/git.git":      {PlaceServer, ForgeNone, "git.kernel.org", "", "pub/scm/git/git"},
		// Folders on this machine.
		"file:///srv/git/tidewater.git": {Place: PlaceLocal},
		"/srv/git/tidewater.git":        {Place: PlaceLocal},
		"../tidewater":                  {Place: PlaceLocal},
		`C:\repos\tidewater`:            {Place: PlaceLocal},
		"C:/repos/tidewater":            {Place: PlaceLocal},
		"./odd:name":                    {Place: PlaceLocal},
		"":                              {Place: PlaceLocal},
	}
	for in, want := range cases {
		if got := ParseLocation(in); got != want {
			t.Errorf("ParseLocation(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestSameRemoteURL(t *testing.T) {
	t.Parallel()
	same := [][2]string{
		{"https://github.com/o/r.git", "https://github.com/o/r"},
		{"https://user:token@GitHub.com/o/r.git/", "https://github.com/o/r"},
		{"git@github.com:o/r.git", "github.com:o/r"},
		{"/srv/git/x.git", "/srv/git/x"},
	}
	for _, p := range same {
		if !SameRemoteURL(p[0], p[1]) {
			t.Errorf("%q and %q should match", p[0], p[1])
		}
	}
	differ := [][2]string{
		{"https://github.com/o/r", "https://github.com/o/R"},
		{"https://github.com/o/r", "git@github.com:o/r"},
		{"", ""},
	}
	for _, p := range differ {
		if SameRemoteURL(p[0], p[1]) {
			t.Errorf("%q and %q shouldn't match", p[0], p[1])
		}
	}
}

func TestFetchHeadURLs(t *testing.T) {
	t.Parallel()
	content := "1111\t\tbranch 'main' of https://github.com/o/r\n" +
		"2222\tnot-for-merge\tbranch 'dev' of https://github.com/o/r\n" +
		"3333\tnot-for-merge\ttag 'v1' of https://gitlab.com/o/r\r\n" +
		"4444\t\t/srv/git/x\n" +
		"\n"
	got := FetchHeadURLs(content)
	want := []string{"https://github.com/o/r", "https://gitlab.com/o/r", "/srv/git/x"}
	if !slices.Equal(got, want) {
		t.Fatalf("FetchHeadURLs = %q, want %q", got, want)
	}
}

func TestConfigBool(t *testing.T) {
	t.Parallel()
	for v, want := range map[string]bool{"true": true, "Yes": true, "on": true, "1": true, "-2": true,
		"false": false, "no": false, "OFF": false, "0": false, "": false, "000": false} {
		if got := configBool(configValue{value: v}); got != want {
			t.Errorf("configBool(%q) = %v, want %v", v, got, want)
		}
	}
	if !configBool(configValue{bare: true}) {
		t.Error("a key without a value is true")
	}
}

func TestRemoteDetails(t *testing.T) {
	t.Parallel()
	local, origin := gittest.WithRemote(t, 1)
	_, mirror := gittest.WithRemote(t, 1)
	local.Git("remote", "add", "mirror", mirror.Dir)
	local.Git("remote", "add", "web", "https://gitlab.com/tidewater/tidewater.git")
	originURL := local.Git("remote", "get-url", "origin")
	// Pushing to origin also pushes to mirror: git wants origin's own URL listed too.
	local.Git("config", "--add", "remote.origin.pushurl", originURL)
	local.Git("config", "--add", "remote.origin.pushurl", mirror.Dir)
	local.Git("config", "remote.mirror.prune", "false")
	local.Git("config", "remote.mirror.tagOpt", "--no-tags")
	local.Git("config", "remote.mirror.skipFetchAll", "true")
	local.Git("config", "--add", "remote.mirror.fetch", "+refs/heads/main:refs/remotes/mirror/main")
	local.Git("config", "remote.pushDefault", "origin")
	local.Git("config", "push.autoSetupRemote", "true")
	local.Git("config", "push.followTags", "false")
	local.Git("config", "push.default", "current")
	repo := OpenRepo(newTestRunner(t), local.Dir)
	ctx := context.Background()

	// Mirror skips a fetch of every remote, so fetch it on its own first; the fetch of every
	// remote then writes FETCH_HEAD for origin alone (web isn't reachable, so leave it out).
	local.Git("config", "remote.web.skipFetchAll", "true")
	if err := repo.Fetch(ctx, "mirror", NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Fetch(ctx, "", NetOptions{}); err != nil {
		t.Fatal(err)
	}

	rs, err := repo.RemoteDetails(ctx)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Remote{}
	for _, r := range rs {
		by[r.Name] = r
	}
	o, m, w := by["origin"], by["mirror"], by["web"]

	if len(o.PushURLs) != 2 || o.PushURL != originURL {
		t.Errorf("origin pushes to %q, first %q", o.PushURLs, o.PushURL)
	}
	if len(o.Pushes) != 2 || o.Pushes[0].Remote != "origin" || o.Pushes[1].Remote != "mirror" {
		t.Errorf("origin's push targets = %+v", o.Pushes)
	}
	if len(m.Pushes) != 1 || m.Pushes[0].Remote != "mirror" {
		t.Errorf("mirror's push targets = %+v", m.Pushes)
	}
	if o.Prune != nil || o.TagOpt != "" || o.SkipFetchAll || len(o.FetchRefspecs) != 1 {
		t.Errorf("origin's fetch settings = %+v", o)
	}
	if m.Prune == nil || *m.Prune || m.TagOpt != "--no-tags" || !m.SkipFetchAll || len(m.FetchRefspecs) != 2 {
		t.Errorf("mirror's fetch settings = %+v", m)
	}
	if o.Location.Place != PlaceLocal || w.Location != (Location{PlaceWeb, ForgeGitLab, "gitlab.com", "", "tidewater/tidewater"}) {
		t.Errorf("locations: origin %+v, web %+v", o.Location, w.Location)
	}
	// FETCH_HEAD only remembers the latest fetch: origin's. Mirror's own fetch came before.
	if o.FetchedAt == 0 || m.FetchedAt != 0 || w.FetchedAt != 0 {
		t.Errorf("fetched at: origin %d, mirror %d, web %d", o.FetchedAt, m.FetchedAt, w.FetchedAt)
	}
	_ = origin

	s := repo.RemoteSettings(ctx)
	if s.PushDefault != "origin" || s.PushMode != "current" || s.FetchPrune != nil ||
		s.AutoSetupRemote == nil || !*s.AutoSetupRemote || s.FollowTags == nil || *s.FollowTags {
		t.Errorf("settings = %+v", s)
	}
}

func TestRemoteDetailsNoRemotes(t *testing.T) {
	t.Parallel()
	repo := OpenRepo(newTestRunner(t), gittest.New(t).Dir)
	rs, err := repo.RemoteDetails(context.Background())
	if err != nil || len(rs) != 0 {
		t.Fatalf("got %+v, %v", rs, err)
	}
	if s := repo.RemoteSettings(context.Background()); s != (RemoteSettings{}) {
		t.Fatalf("settings = %+v", s)
	}
}
