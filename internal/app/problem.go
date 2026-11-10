package app

import (
	"errors"
	"regexp"
	"strings"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Problem is an error told the way the app shows it: what failed and why in one line, where
// that leaves the user, what they can do next, and what git said behind a Details toggle.
// It travels as a problemErr, which reads as the title and detail, so anything that only
// wants text gets a short message; the frontend asks for the rest with TakeProblem.
//
// Problem itself must not be an error: Wails takes a bound method's result that implements
// error for a failure, and TakeProblem would reject with the very thing it returns.
type Problem struct {
	// Title says what failed and why: "Push refused: main only takes changes through a pull request".
	Title string `json:"title"`
	// Detail says where that leaves things, or what to check.
	Detail string `json:"detail"`
	// Quote is git's own last word, shown as git wrote it when the app has no better one.
	Quote string `json:"quote"`
	// Actions are the next steps on offer, at most two.
	Actions []ProblemAction `json:"actions"`
	// Output is what git said without its progress lines; Hidden is how many were left out.
	Output string `json:"output"`
	Hidden int    `json:"hidden"`
	// Full is everything git said, for copying into a bug report.
	Full string `json:"full"`
	// Path is the repository it happened in.
	Path string `json:"path"`
}

// ProblemAction is one next step. The frontend knows how to do each kind.
type ProblemAction struct {
	// Kind is "push-new-branch", "open-url", "pull-then-push", "force-push" or "retry".
	Kind  string `json:"kind"`
	Label string `json:"label"`
	// URL is where "open-url" goes.
	URL string `json:"url"`
}

// Text is the problem as a message: its title, then its detail and git's last word.
func (p *Problem) Text() string {
	parts := []string{p.Title}
	for _, s := range []string{p.Detail, p.Quote} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}

// problemErr is a Problem as an error.
type problemErr struct{ p *Problem }

func (e *problemErr) Error() string { return e.p.Text() }

// problems keeps the last Problem returned to the frontend, which fetches it by its text.
type problems struct {
	mu   sync.Mutex
	last *Problem
}

// remember keeps err for TakeProblem when it's a Problem, noting the repository, and returns
// err as it came.
func (a *App) remember(path string, err error) error {
	var pe *problemErr
	if errors.As(err, &pe) {
		pe.p.Path = path
		a.problems.mu.Lock()
		a.problems.last = pe.p
		a.problems.mu.Unlock()
	}
	return err
}

// TakeProblem returns the Problem behind an error message the frontend just received, or nil
// when that error was only text. It's handed out once.
func (a *App) TakeProblem(message string) *Problem {
	a.problems.mu.Lock()
	defer a.problems.mu.Unlock()
	p := a.problems.last
	if p == nil || p.Text() != message {
		return nil
	}
	a.problems.last = nil
	return p
}

// progressLine matches the meters and counts git prints while it works. None of it says why
// something failed.
var progressLine = regexp.MustCompile(`^(remote: )?(Enumerating objects|Counting objects|Compressing objects|Writing objects|Resolving deltas|Receiving objects|Unpacking objects|Updating files|Checking out files|Delta compression using|Path-based delta compression|Total \d+)`)

// tidyOutput returns git's output without progress and empty "remote:" lines, and how many
// lines it left out. A meter redrawn with carriage returns counts as one line.
func tidyOutput(stderr string) (kept string, hidden int) {
	var lines []string
	for l := range strings.SplitSeq(stderr, "\n") {
		meter := false
		for seg := range strings.SplitSeq(l, "\r") {
			seg = strings.TrimRight(seg, " \t")
			switch {
			case progressLine.MatchString(seg):
				meter = true
			case seg == "remote:" || seg == "":
			default:
				lines = append(lines, seg)
			}
		}
		if meter {
			hidden++
		}
	}
	return strings.Join(lines, "\n"), hidden
}

// lastWord is the line of git's output that says what went wrong: its last "fatal:" or
// "error:" line, else its last line.
func lastWord(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "fatal: ") || strings.HasPrefix(l, "error: ") {
			return l
		}
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

// opNames are the operations a Problem's title can start with.
var opNames = map[string]string{"fetch": "Fetch", "pull": "Pull", "push": "Push", "clone": "Clone"}

// titled puts the operation in front of why it failed: "Fetch failed: couldn't reach
// github.com". Without a known operation the reason stands alone, capitalised unless it
// starts with a host's name.
func titled(op, verdict, why, host string) string {
	if name, ok := opNames[op]; ok {
		return name + " " + verdict + ": " + why
	}
	if host != "the remote" && strings.HasPrefix(why, host) {
		return why
	}
	return strings.ToUpper(why[:1]) + why[1:]
}

// plainErr is err on one line, for a notice or a list: a Problem's title and detail as two
// sentences.
func plainErr(err error) string {
	var pe *problemErr
	if !errors.As(err, &pe) {
		return err.Error()
	}
	p := pe.p
	if p.Detail == "" {
		return strings.TrimSpace(p.Title + ". " + p.Quote)
	}
	return p.Title + ". " + p.Detail
}

var (
	pushedTo       = regexp.MustCompile(`(?m)^To (\S+)$`)
	urlHost        = regexp.MustCompile(`(?:https?|ssh|git)://(?:[^@/\s']+@)?([^/:\s']+)`)
	userAtHost     = regexp.MustCompile(`[\w.-]+@([\w-]+(?:\.[\w-]+)+):`)
	scpHost        = regexp.MustCompile(`(?:^|[\s'])(?:[\w.-]+@)?([\w-]+(?:\.[\w-]+)+):[^\s/]`)
	remoteRejected = regexp.MustCompile(`(?m)^ ! \[remote rejected\]\s+(\S+) -> (\S+) \((.*)\)$`)
	ruleLine       = regexp.MustCompile(`(?m)^remote: - (.+)$`)
	rulesURL       = regexp.MustCompile(`(?m)^remote: Review all repository rules at (https://\S+)`)
)

// remoteHost names the server git was talking to, from its own output, or "the remote".
func remoteHost(stderr string) string {
	target := stderr
	if m := pushedTo.FindStringSubmatch(stderr); m != nil {
		target = " " + m[1]
	}
	if m := urlHost.FindStringSubmatch(target); m != nil {
		return m[1]
	}
	if m := userAtHost.FindStringSubmatch(target); m != nil {
		return m[1]
	}
	if m := scpHost.FindStringSubmatch(target); m != nil {
		return m[1]
	}
	return "the remote"
}

// sentenceStart capitalises a sentence that starts with a host or "the remote".
func sentenceStart(s string) string {
	if strings.HasPrefix(s, "the ") {
		return "T" + s[1:]
	}
	return s
}

// netProblem explains a failed fetch, pull, push or clone. op names which, or is "" when the
// caller doesn't know. Errors that didn't come from git pass through writeErr.
func netProblem(op string, err error) error {
	var gerr *git.Error
	if !errors.As(err, &gerr) {
		return writeErr(err)
	}
	s := gerr.Stderr
	output, hidden := tidyOutput(s)
	host := remoteHost(s)
	p := &Problem{Output: output, Hidden: hidden, Full: strings.TrimSpace(s), Actions: []ProblemAction{}}
	retry := []ProblemAction{{Kind: "retry", Label: "Try again"}}
	switch {
	case remoteRejected.MatchString(s):
		return ruleProblem(p, s, host)
	case strings.Contains(s, "Authentication failed"), strings.Contains(s, "could not read Username"),
		strings.Contains(s, "Invalid username or password"), strings.Contains(s, "returned error: 401"):
		p.Title = titled(op, "failed", host+" didn't accept your sign-in", host)
		p.Detail = "Check your credentials, or Git Credential Manager, and try again."
		p.Actions = retry
	case strings.Contains(s, "returned error: 403"):
		p.Title = titled(op, "refused", host+" says you aren't allowed to", host)
		p.Detail = "The server answered HTTP 403. Your account may not have permission to push to or read this repository."
	case strings.Contains(s, "Permission denied (publickey"):
		p.Title = titled(op, "failed", host+" refused your SSH key", host)
		p.Detail = "Make sure the key is loaded in ssh-agent and added to your account."
		p.Actions = retry
	case strings.Contains(s, "Host key verification failed"):
		p.Title = titled(op, "stopped", host+"'s SSH host key isn't trusted", host)
		p.Detail = "The connection was stopped before anything was sent."
	case strings.Contains(s, "Repository not found"), strings.Contains(s, "' does not exist"),
		strings.Contains(s, "does not appear to be a git repository"),
		strings.Contains(s, "repository '") && strings.Contains(s, "' not found"):
		p.Title = titled(op, "failed", "no repository was found at that URL", host)
		p.Detail = "Check the URL, and that your account can access the repository."
	case strings.Contains(s, "Could not resolve host"), strings.Contains(s, "unable to access"):
		p.Title = titled(op, "failed", "couldn't reach "+host, host)
		p.Detail = "Check the URL and your network connection. Your repository is as it was."
		p.Actions = retry
	case rejectedPushText(s):
		p.Title = titled(op, "refused", host+" has commits you don't have", host)
		p.Detail = "Pull first, then push again. Nothing was pushed."
		p.Actions = []ProblemAction{{Kind: "pull-then-push", Label: "Pull, then push"}, {Kind: "force-push", Label: "Force push…"}}
	case strings.Contains(s, "stale info"):
		p.Title = titled(op, "refused", "the remote branch moved since you last fetched", host)
		p.Detail = "Force push was stopped to protect it. Fetch, look at what changed, then try again."
	case strings.Contains(s, "Not possible to fast-forward"), strings.Contains(s, "Diverging branches"):
		p.Title = titled(op, "stopped", "your branch and its upstream have diverged", host)
		p.Detail = "Pull with merge or rebase instead of fast-forward only."
	case strings.TrimSpace(s) == "":
		return err
	default:
		if _, ok := opNames[op]; !ok {
			return gitMessage(s)
		}
		p.Title = opNames[op] + " didn't finish"
		p.Quote = lastWord(output)
	}
	return &problemErr{p}
}

// ruleProblem explains a push the server turned down by rule: a protected branch, a
// required pull request, a hook. Git's own wording is one line among progress and URLs.
func ruleProblem(p *Problem, stderr, host string) error {
	m := remoteRejected.FindStringSubmatch(stderr)
	branch, reason := m[2], m[3]
	var rules []string
	for _, r := range ruleLine.FindAllStringSubmatch(stderr, -1) {
		rules = append(rules, strings.TrimSpace(r[1]))
	}
	switch {
	case len(rules) == 1 && strings.HasPrefix(rules[0], "Changes must be made through a pull request"):
		p.Title = "Push refused: " + branch + " only takes changes through a pull request"
		p.Detail = sentenceStart(host) + " turned the push down. Nothing was pushed, and your commits are still on your " + branch + "."
	case len(rules) > 0:
		p.Title = "Push refused: " + host + "'s rules for " + branch + " don't allow it"
		p.Detail = strings.Join(rules, " ") + " Nothing was pushed."
	default:
		p.Title = "Push refused: " + host + " didn't accept " + branch
		p.Detail = "It said: " + reason + ". Nothing was pushed."
	}
	p.Actions = []ProblemAction{{Kind: "push-new-branch", Label: "Push to a new branch…"}}
	if u := rulesURL.FindStringSubmatch(stderr); u != nil {
		p.Actions = append(p.Actions, ProblemAction{Kind: "open-url", Label: "See the rules", URL: u[1]})
	}
	return &problemErr{p}
}
