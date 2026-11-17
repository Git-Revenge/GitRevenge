package app

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// promptOrigin is the operation a credential prompt came from. The prompt's own words are
// written by git, by ssh, or by the server (an SSH server can ask for anything), so the dialog
// says what the app itself knows: which operation is running, where, and which servers it
// was set up to talk to.
type promptOrigin struct {
	// Op is "fetch", "pull", "push" or "clone".
	Op string
	// Path is the repository, or the folder being cloned into.
	Path string
	// Hosts lists the servers the operation is set up to talk to. It's asked for only when
	// a prompt names a server.
	Hosts func() []string
}

// Servers a prompt can name: in a URL, as user@host, or as the host whose key ssh asks about.
var (
	promptURLHost  = regexp.MustCompile(`(?:https?|ssh|git)://(?:[^@/\s']*@)?([^/:\s']+)`)
	promptUserHost = regexp.MustCompile(`[\w.-]+@([\w-]+(?:\.[\w-]+)*)'s password`)
	promptKeyHost  = regexp.MustCompile(`authenticity of host '\[?([^'\]\s(]+)`)
)

// promptHost returns the server a prompt names, in lower case, or "" when it names none (a
// key's passphrase, say).
func promptHost(text string) string {
	for _, re := range []*regexp.Regexp{promptURLHost, promptUserHost, promptKeyHost} {
		if m := re.FindStringSubmatch(text); m != nil {
			return strings.ToLower(m[1])
		}
	}
	return ""
}

// describe fills in what the app knows about a prompt: the operation, the repository's name,
// the server the prompt names, and whether that server is one the operation was set up to
// talk to. A server that isn't can be innocent (an SSH alias, a url.insteadOf rewrite, a
// redirect), so it is said, not refused.
func (o promptOrigin) describe(p *Prompt) {
	p.Op = o.Op
	if o.Path != "" {
		p.Repo = filepath.Base(o.Path)
	}
	p.Host = promptHost(p.Text)
	if p.Host == "" || o.Hosts == nil {
		return
	}
	p.Unexpected = !slices.Contains(o.Hosts(), p.Host)
}
