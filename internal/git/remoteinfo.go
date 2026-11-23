package git

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Place says what kind of address a remote URL is, which decides what can be done with it
// beyond git: a web address can open in a browser once its forge is known, a server is
// reached over SSH or git's own protocol, and a local remote is a folder on this machine.
type Place string

// The places a remote can be.
const (
	PlaceWeb    Place = "web"
	PlaceServer Place = "server"
	PlaceLocal  Place = "local"
)

// Forge is the hosting service a remote is on, when it's one known by its host name. A
// self-hosted forge is never guessed from its name: a link rule has to say what it is.
type Forge string

// The forges known by host.
const (
	ForgeNone      Forge = ""
	ForgeGitHub    Forge = "github"
	ForgeGitLab    Forge = "gitlab"
	ForgeGitea     Forge = "gitea"
	ForgeForgejo   Forge = "forgejo"
	ForgeCodeberg  Forge = "codeberg"
	ForgeBitbucket Forge = "bitbucket"
	ForgeAzure     Forge = "azure"
)

// knownHosts maps a host, as it appears in a remote URL, to its forge and the host its web
// pages are on. The SSH hosts some forges use beside their web host are listed too.
var knownHosts = map[string]struct {
	forge Forge
	web   string
}{
	"github.com":           {ForgeGitHub, "github.com"},
	"www.github.com":       {ForgeGitHub, "github.com"},
	"ssh.github.com":       {ForgeGitHub, "github.com"},
	"gitlab.com":           {ForgeGitLab, "gitlab.com"},
	"altssh.gitlab.com":    {ForgeGitLab, "gitlab.com"},
	"gitea.com":            {ForgeGitea, "gitea.com"},
	"codeberg.org":         {ForgeCodeberg, "codeberg.org"},
	"bitbucket.org":        {ForgeBitbucket, "bitbucket.org"},
	"altssh.bitbucket.org": {ForgeBitbucket, "bitbucket.org"},
	"dev.azure.com":        {ForgeAzure, "dev.azure.com"},
	"ssh.dev.azure.com":    {ForgeAzure, "dev.azure.com"},
}

// Location is where a remote URL points: the host its web pages would be on (lower case),
// the port when a web address names one, and the repository's path there ("owner/repo",
// without ".git"). Host and Path are empty for a local remote.
type Location struct {
	Place Place  `json:"place"`
	Forge Forge  `json:"forge"`
	Host  string `json:"host"`
	Port  string `json:"port"`
	Path  string `json:"path"`
}

// ParseLocation reads a remote URL in any form git accepts: https, ssh://, git://, scp-style
// (git@host:owner/repo.git), file:// and plain paths. Azure DevOps SSH addresses
// (git@ssh.dev.azure.com:v3/org/project/repo) are given their web host and path.
func ParseLocation(remoteURL string) Location {
	u := strings.TrimSpace(remoteURL)
	host, path := webLocation(u)
	if host == "" {
		return Location{Place: PlaceLocal}
	}
	loc := Location{Place: PlaceServer, Host: host, Path: path}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		loc.Place = PlaceWeb
		loc.Port = urlPort(u)
	}
	switch {
	case strings.HasSuffix(host, ".visualstudio.com"):
		loc.Forge = ForgeAzure
		if host == "vs-ssh.visualstudio.com" {
			// org@vs-ssh.visualstudio.com:v3/org/project/repo
			if org, rest, ok := strings.Cut(strings.TrimPrefix(path, "v3/"), "/"); ok && strings.HasPrefix(path, "v3/") {
				loc.Host = strings.ToLower(org) + ".visualstudio.com"
				loc.Path = azureGitPath(rest)
			}
		}
	default:
		if k, ok := knownHosts[host]; ok {
			loc.Forge, loc.Host = k.forge, k.web
			if loc.Forge != ForgeNone && loc.Place == PlaceServer {
				// A forge's web pages don't share its SSH port.
				loc.Port = ""
			}
			if host == "ssh.dev.azure.com" && strings.HasPrefix(path, "v3/") {
				loc.Path = azureGitPath(strings.TrimPrefix(path, "v3/"))
			}
		}
	}
	return loc
}

// azureGitPath turns Azure DevOps' SSH path "org/project/repo" into its web path
// "org/project/_git/repo", or "project/repo" into "project/_git/repo".
func azureGitPath(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return p
	}
	return p[:i] + "/_git/" + p[i+1:]
}

// urlPort returns the port a scheme URL names after its host, or "".
func urlPort(u string) string {
	rest := u[strings.Index(u, "://")+3:]
	hostPart, _, _ := strings.Cut(rest, "/")
	if at := strings.LastIndex(hostPart, "@"); at >= 0 {
		hostPart = hostPart[at+1:]
	}
	if _, port, ok := strings.Cut(hostPart, ":"); ok {
		return port
	}
	return ""
}

// SameRemoteURL says whether two remote URLs name the same repository as git writes them
// down: user names and passwords, a trailing slash and ".git" don't count, nor does the
// host's letter case.
func SameRemoteURL(a, b string) bool {
	return a != "" && b != "" && comparableURL(a) == comparableURL(b)
}

func comparableURL(u string) string {
	u = strings.TrimSpace(u)
	scheme := ""
	if i := strings.Index(u, "://"); i >= 0 {
		scheme, u = strings.ToLower(u[:i+3]), u[i+3:]
		hostPart, path, _ := strings.Cut(u, "/")
		if at := strings.LastIndex(hostPart, "@"); at >= 0 {
			hostPart = hostPart[at+1:]
		}
		u = strings.ToLower(hostPart) + "/" + path
	} else if host, path, ok := strings.Cut(u, ":"); ok && !localPath(u) {
		if at := strings.Index(host, "@"); at >= 0 {
			host = host[at+1:]
		}
		u = strings.ToLower(host) + ":" + path
	}
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return scheme + strings.TrimRight(u, "/")
}

// driveLetter matches a Windows path that starts with a drive: C:\ or C:/.
var driveLetter = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// localPath says whether a URL without a scheme is a path on this machine rather than
// scp-style host:path. Git reads it as a path when a slash comes before the first colon, or
// when it starts with a Windows drive.
func localPath(u string) bool {
	colon := strings.Index(u, ":")
	if colon < 0 || driveLetter.MatchString(u) {
		return true
	}
	slash := strings.IndexAny(u, `/\`)
	return slash >= 0 && slash < colon
}

// PushTarget is one URL a push to a remote goes to. Remote names the remote whose own URL
// it is, which may be the remote itself: a push URL that matches another remote's URL
// follows that remote.
type PushTarget struct {
	URL    string `json:"url"`
	Remote string `json:"remote"`
}

// RemoteSettings are the repository's settings, from every scope of git config, for where
// pushes go and whether fetches prune. A pointer is nil when the setting isn't set.
type RemoteSettings struct {
	// PushDefault is remote.pushDefault, the remote pushes go to when a branch names none.
	PushDefault string `json:"pushDefault"`
	// FetchPrune is fetch.prune.
	FetchPrune *bool `json:"fetchPrune"`
	// PushMode is push.default: simple, current, upstream, matching or nothing.
	PushMode string `json:"pushMode"`
	// AutoSetupRemote is push.autoSetupRemote.
	AutoSetupRemote *bool `json:"autoSetupRemote"`
	// FollowTags is push.followTags.
	FollowTags *bool `json:"followTags"`
}

// RemoteDetails lists the remotes as Remotes does, with each one's location, fetch
// settings, push targets and when it was last fetched, as far as git can tell
// (FETCH_HEAD only remembers the latest fetch).
func (r *Repo) RemoteDetails(ctx context.Context) ([]Remote, error) {
	remotes, err := r.Remotes(ctx)
	if err != nil || len(remotes) == 0 {
		return remotes, err
	}
	cfg := r.configMatching(ctx, `^remote\.`)
	fetched := r.fetchHeadTimes(ctx)
	for i := range remotes {
		rm := &remotes[i]
		rm.Location = ParseLocation(rm.FetchURL)
		rm.FetchRefspecs = []string{}
		rm.Pushes = []PushTarget{}
		prefix := "remote." + rm.Name + "."
		for _, kv := range cfg {
			// Section and key names come back in lower case, the remote's name as written.
			if !strings.HasPrefix(kv.key, prefix) || strings.Contains(kv.key[len(prefix):], ".") {
				continue
			}
			switch kv.key[len(prefix):] {
			case "prune":
				b := configBool(kv)
				rm.Prune = &b
			case "tagopt":
				rm.TagOpt = kv.value
			case "skipfetchall", "skipdefaultupdate":
				// Git reads both keys into one setting; the last one wins.
				rm.SkipFetchAll = configBool(kv)
			case "fetch":
				rm.FetchRefspecs = append(rm.FetchRefspecs, kv.value)
			}
		}
		for _, u := range rm.PushURLs {
			t := PushTarget{URL: u}
			if SameRemoteURL(u, rm.FetchURL) {
				t.Remote = rm.Name
			} else {
				for _, other := range remotes {
					if SameRemoteURL(u, other.FetchURL) {
						t.Remote = other.Name
						break
					}
				}
			}
			rm.Pushes = append(rm.Pushes, t)
		}
		for u, at := range fetched {
			if SameRemoteURL(u, rm.FetchURL) {
				rm.FetchedAt = at.Unix()
			}
		}
	}
	return remotes, nil
}

// RemoteSettings reads the repository's push and prune settings.
func (r *Repo) RemoteSettings(ctx context.Context) RemoteSettings {
	var s RemoteSettings
	for _, kv := range r.configMatching(ctx, `^(remote\.pushdefault|fetch\.prune|push\.(default|autosetupremote|followtags))$`) {
		b := configBool(kv)
		switch kv.key {
		case "remote.pushdefault":
			s.PushDefault = kv.value
		case "fetch.prune":
			s.FetchPrune = &b
		case "push.default":
			s.PushMode = kv.value
		case "push.autosetupremote":
			s.AutoSetupRemote = &b
		case "push.followtags":
			s.FollowTags = &b
		}
	}
	return s
}

type configValue struct {
	key, value string
	// bare is a key written without "= value", which git reads as true.
	bare bool
}

// configMatching returns the config entries whose keys match pattern, in the order git reads
// them (so a later one overrides an earlier one), from every scope.
func (r *Repo) configMatching(ctx context.Context, pattern string) []configValue {
	// Exits 1 when nothing matches.
	out, _ := r.ReadString(ctx, "config", "--null", "--get-regexp", pattern)
	var kvs []configValue
	for entry := range strings.SplitSeq(out, "\x00") {
		if entry == "" {
			continue
		}
		key, value, hasValue := strings.Cut(entry, "\n")
		kvs = append(kvs, configValue{key: key, value: value, bare: !hasValue})
	}
	return kvs
}

// configBool reads a value as git reads a boolean. A value it wouldn't accept counts as
// false here rather than failing the whole read.
func configBool(kv configValue) bool {
	if kv.bare {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(kv.value)) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off", "":
		return false
	}
	v := strings.TrimLeft(strings.TrimSpace(kv.value), "+-0")
	return v != ""
}

// CommonDir returns the absolute git directory shared by all of the repository's worktrees.
func (r *Repo) CommonDir(ctx context.Context) (string, error) {
	return r.ReadString(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

// fetchHeadTimes reads FETCH_HEAD, which the latest fetch writes, and returns the time it
// was written for each URL it fetched from. A fetch of every remote lists each of them.
func (r *Repo) fetchHeadTimes(ctx context.Context) map[string]time.Time {
	p, err := r.ReadString(ctx, "rev-parse", "--path-format=absolute", "--git-path", "FETCH_HEAD")
	if err != nil {
		return nil
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.dir, p)
	}
	info, err := os.Stat(p)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	times := map[string]time.Time{}
	for _, u := range FetchHeadURLs(string(data)) {
		times[u] = info.ModTime()
	}
	return times
}

// FetchHeadURLs lists the URLs a FETCH_HEAD's lines came from, each once. A line is
// "<id>\t[not-for-merge]\t<what> of <url>", or "<id>\t\t<url>" when a fetch named no
// branch.
func FetchHeadURLs(content string) []string {
	var urls []string
	for line := range strings.SplitSeq(content, "\n") {
		parts := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 3)
		if len(parts) < 3 || parts[2] == "" {
			continue
		}
		u := parts[2]
		if i := strings.LastIndex(u, " of "); i >= 0 {
			u = u[i+4:]
		}
		if !slices.Contains(urls, u) {
			urls = append(urls, u)
		}
	}
	return urls
}
