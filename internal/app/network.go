package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/askpass"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// network holds the credential bridge and in-flight network operations.
type network struct {
	once    sync.Once
	server  *askpass.Server
	exe     string
	initErr error

	mu      sync.Mutex
	prompts map[string]chan promptAnswer  // pending prompts by ID
	running map[string]context.CancelFunc // network operation per repo path
}

type promptAnswer struct {
	answer string
	ok     bool
}

// Prompt is the payload of the "askpass:prompt" event.
type Prompt struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Kind   string `json:"kind"`
	Expiry int64  `json:"expiry"`
}

// NetProgress is the payload of the "net:progress" event.
type NetProgress struct {
	Path    string `json:"path"`
	Op      string `json:"op"`
	Phase   string `json:"phase"`
	Percent int    `json:"percent"`
	Line    string `json:"line"`
}

// askEnv returns the environment that routes credential prompts to this app.
func (a *App) askEnv() ([]string, error) {
	n := &a.net
	n.once.Do(func() {
		n.prompts = map[string]chan promptAnswer{}
		n.running = map[string]context.CancelFunc{}
		// Tests may preset exe to a small helper build.
		if n.exe == "" {
			if n.exe, n.initErr = os.Executable(); n.initErr != nil {
				return
			}
		}
		n.server, n.initErr = askpass.Listen(a.handlePrompt)
	})
	if n.initErr != nil {
		return nil, n.initErr
	}
	return n.server.Env(n.exe), nil
}

// handlePrompt shows a prompt in the UI and waits for AnswerPrompt.
func (a *App) handlePrompt(ctx context.Context, text string, kind askpass.Kind) (string, bool) {
	if a.promptHook != nil {
		return a.promptHook(text, kind)
	}
	if a.ctx == nil {
		return "", false // no UI to ask
	}
	idb := make([]byte, 8)
	_, _ = rand.Read(idb)
	id := hex.EncodeToString(idb)
	ch := make(chan promptAnswer, 1)
	a.net.mu.Lock()
	a.net.prompts[id] = ch
	a.net.mu.Unlock()
	defer func() {
		a.net.mu.Lock()
		delete(a.net.prompts, id)
		a.net.mu.Unlock()
	}()
	deadline, _ := ctx.Deadline()
	a.emit("askpass:prompt", Prompt{ID: id, Text: strings.TrimSpace(text), Kind: string(kind), Expiry: deadline.Unix()})
	select {
	case ans := <-ch:
		return ans.answer, ans.ok
	case <-ctx.Done():
		a.emit("askpass:expired", id)
		return "", false
	}
}

// AnswerPrompt delivers the user's answer to a credential prompt. ok=false cancels it.
func (a *App) AnswerPrompt(id, answer string, ok bool) {
	a.net.mu.Lock()
	ch := a.net.prompts[id]
	a.net.mu.Unlock()
	if ch != nil {
		ch <- promptAnswer{answer, ok}
	}
}

// runNetwork runs fn as the repository's one network operation, with progress events and
// cancellation via CancelNetwork.
func (a *App) runNetwork(path, op string, fn func(ctx context.Context, r *session.Repo, o git.NetOptions) error) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	return a.track(r.Path, r.Path, op, "Another fetch, pull or push is already running in this repository.",
		func(ctx context.Context, o git.NetOptions) error { return fn(ctx, r, o) })
}

// track runs fn as the network operation registered under key (busy is the message when one
// is already running there), reporting progress as "net:progress" events for path.
func (a *App) track(key, path, op string, busy userError, fn func(ctx context.Context, o git.NetOptions) error) error {
	env, err := a.askEnv()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.net.mu.Lock()
	if _, running := a.net.running[key]; running {
		a.net.mu.Unlock()
		return busy
	}
	a.net.running[key] = cancel
	a.net.mu.Unlock()
	defer func() {
		a.net.mu.Lock()
		delete(a.net.running, key)
		a.net.mu.Unlock()
	}()

	var last time.Time
	o := git.NetOptions{Env: env, OnProgress: func(p git.Progress) {
		// git redraws meters many times a second; the UI needs a few updates.
		if time.Since(last) < 100*time.Millisecond && p.Percent != 100 {
			return
		}
		last = time.Now()
		a.emit("net:progress", NetProgress{Path: path, Op: op, Phase: p.Phase, Percent: p.Percent, Line: p.Line})
	}}
	err = fn(ctx, o)
	if errors.Is(err, context.Canceled) {
		return userError("Cancelled.")
	}
	return netErr(err)
}

// CancelNetwork stops the repository's running fetch, pull or push.
func (a *App) CancelNetwork(path string) {
	r, err := a.openRepo(path)
	if err != nil {
		return
	}
	a.cancelNetwork(r.Path)
}

func (a *App) cancelNetwork(key string) {
	a.net.mu.Lock()
	cancel := a.net.running[key]
	a.net.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Fetch downloads from a remote, or all remotes when remote is empty.
func (a *App) Fetch(path, remote string) error {
	return a.runNetwork(path, "fetch", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		a.upstreams.remember(ctx, r.Path, r.Git)
		if err := r.Git.Fetch(ctx, remote, o); err != nil {
			return err
		}
		a.syncNotesIn(ctx, r, remotesFor(ctx, r.Git, remote), o)
		return nil
	})
}

// Pull integrates the current branch's upstream. mode is "", "ff-only", "merge" or "rebase".
func (a *App) Pull(path, mode string) error {
	return a.runNetwork(path, "pull", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		a.upstreams.remember(ctx, r.Path, r.Git)
		if err := a.record(r, "Pull", kindMove, "", func() error { return r.Git.Pull(ctx, git.PullMode(mode), o) }); err != nil {
			return err
		}
		a.cleanupAfterPull(ctx, r)
		a.syncNotesIn(ctx, r, []string{pullRemote(ctx, r.Git)}, o)
		return nil
	})
}

// PullBranch fast-forwards a branch that isn't checked out from its upstream. It never switches
// branches, so it can't disturb uncommitted changes, and it stops if the branch has diverged.
// Cleanup on pull doesn't run: pulling one branch shouldn't delete others.
func (a *App) PullBranch(path, branch string) error {
	return a.runNetwork(path, "pull", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		// Said before the network is involved, and names the worktree. A rebase or bisect
		// doesn't show here; git refuses those during the pull.
		if wts, err := r.Git.Worktrees(ctx); err == nil {
			for _, w := range wts {
				switch {
				case w.Branch != branch:
				case w.Current:
					return userError(branch + " is the checked-out branch. Use Pull in the toolbar.")
				default:
					return userError(branch + " is checked out in the worktree at " + w.Path + ". Pull it there.")
				}
			}
		}
		a.upstreams.remember(ctx, r.Path, r.Git)
		from, to, err := r.Git.PullBranch(ctx, branch, o)
		if err != nil {
			return pullBranchErr(branch, err)
		}
		if from == to {
			a.notice(r.Path, branch+" is already up to date.")
		} else {
			a.undo.push(r.Path, undoEntry{Label: "Pull " + branch, Kind: kindBranchMove, Ref: branch, From: from, Commit: to})
			a.undoChanged(r.Path)
			a.notice(r.Path, "Pulled the latest commits into "+branch+".")
		}
		if remote, _, err := r.Git.BranchUpstream(ctx, branch); err == nil && remote != "." {
			a.syncNotesIn(ctx, r, []string{remote}, o)
		}
		return nil
	})
}

// pullBranchErr explains why a branch couldn't be pulled.
func pullBranchErr(branch string, err error) error {
	switch {
	case errors.Is(err, git.ErrBranchDiverged):
		return userError(branch + " has diverged from its upstream. Check it out to merge or rebase.")
	case errors.Is(err, git.ErrBranchInUse):
		return userError(branch + " is in use by a rebase or bisect, so it can't be updated from here.")
	case errors.Is(err, git.ErrNoUpstream):
		return userError(branch + " has no upstream to pull from. Set one first.")
	case errors.Is(err, git.ErrUpstreamGone):
		return userError("The remote no longer has the branch " + branch + " tracks, so there's nothing to pull.")
	}
	return err
}

// PushCurrent pushes the checked-out branch: to its upstream if it has one, otherwise to
// the only remote (or "origin") and records that as the upstream.
func (a *App) PushCurrent(path string, force bool) error {
	return a.runNetwork(path, "push", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		st, err := r.Git.Status(ctx)
		if err != nil {
			return err
		}
		if st.Head == "" {
			return userError("HEAD is detached. Check out a branch to push it.")
		}
		p, upstream, err := pushTarget(ctx, r.Git, st.Head)
		if err != nil {
			return err
		}
		p.ForceWithLease = force
		if err := r.Git.Push(ctx, p, o); err != nil {
			return pushRefusal(ctx, r.Git, st.Head, upstream, err)
		}
		a.pushNotesTo(ctx, r, p.Remote, o)
		return nil
	})
}

// PushBranch pushes one branch, checked out or not, to the same place PushCurrent would. It says
// what happened, since a branch that isn't checked out changes nothing else on screen.
func (a *App) PushBranch(path, branch string) error {
	return a.runNetwork(path, "push", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		p, upstream, err := pushTarget(ctx, r.Git, branch)
		if err != nil {
			return err
		}
		// Asked before pushing, which moves the remote-tracking branch.
		current := upstream != "" && sameCommit(ctx, r.Git, "refs/heads/"+branch, "refs/remotes/"+upstream)
		if err := r.Git.Push(ctx, p, o); err != nil {
			return pushRefusal(ctx, r.Git, branch, upstream, err)
		}
		a.pushNotesTo(ctx, r, p.Remote, o)
		where := p.Remote
		if upstream != "" && upstream != p.Remote+"/"+branch {
			where = upstream
		}
		if current {
			a.notice(r.Path, branch+" is already up to date on "+where+".")
		} else {
			a.notice(r.Path, "Pushed "+branch+" to "+where+".")
		}
		return nil
	})
}

// pushTarget works out where a branch pushes: to its upstream if it tracks a remote branch,
// otherwise to the only remote (or "origin"), recording that as the upstream. upstream is the
// short name of what it already tracks there, or "".
func pushTarget(ctx context.Context, g *git.Repo, branch string) (p git.PushOptions, upstream string, err error) {
	source := "refs/heads/" + branch
	// A remote of "." means it tracks a local branch, which isn't somewhere to push.
	if remote, merge, err := g.BranchUpstream(ctx, branch); err == nil && remote != "." {
		upstream = remote + "/" + strings.TrimPrefix(merge, "refs/heads/")
		return git.PushOptions{Remote: remote, Refspec: source + ":" + merge}, upstream, nil
	}
	remotes, err := g.Remotes(ctx)
	if err != nil {
		return p, "", err
	}
	p = git.PushOptions{Refspec: source, SetUpstream: true}
	switch {
	case len(remotes) == 0:
		return p, "", userError("This repository has no remotes. Add one first.")
	case len(remotes) == 1:
		p.Remote = remotes[0].Name
	case hasRemote(remotes, "origin"):
		p.Remote = "origin"
	default:
		return p, "", userError("This branch has no upstream and there's no \"origin\" remote. Set an upstream first.")
	}
	return p, "", nil
}

// sameCommit reports whether two refs both exist and point at the same commit.
func sameCommit(ctx context.Context, g *git.Repo, a, b string) bool {
	first, err := g.ReadString(ctx, "rev-parse", "--verify", "-q", a)
	if err != nil {
		return false
	}
	second, err := g.ReadString(ctx, "rev-parse", "--verify", "-q", b)
	return err == nil && first == second
}

func hasRemote(rs []git.Remote, name string) bool {
	for _, r := range rs {
		if r.Name == name {
			return true
		}
	}
	return false
}

// PushRef pushes a tag, or deletes a ref from a remote: kind is "tag", "delete-branch" or
// "delete-tag". PushBranch pushes branches.
func (a *App) PushRef(path, remote, kind, name string) error {
	refspec := map[string]string{
		"tag":           "refs/tags/" + name,
		"delete-branch": ":refs/heads/" + name,
		"delete-tag":    ":refs/tags/" + name,
	}[kind]
	if refspec == "" {
		return userError("Unknown push kind " + kind + ".")
	}
	return a.runNetwork(path, "push", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		return r.Git.Push(ctx, git.PushOptions{Remote: remote, Refspec: refspec}, o)
	})
}

// Remotes lists the repository's remotes.
func (a *App) Remotes(path string) ([]git.Remote, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	return r.Git.Remotes(context.Background())
}

// IssueURL returns a pattern for links to the repository's issues, with "{n}" for the
// issue number, from origin (or the first remote). It's "" when the remote isn't on GitHub
// or GitLab.
func (a *App) IssueURL(path string) (string, error) {
	remotes, err := a.Remotes(path)
	if err != nil || len(remotes) == 0 {
		return "", err
	}
	pick := remotes[0]
	for _, r := range remotes {
		if r.Name == "origin" {
			pick = r
		}
	}
	return git.IssueURL(pick.FetchURL), nil
}

// RemoteAction edits remotes: add (name, url), rename (from, to), seturl (name, url),
// remove (name).
func (a *App) RemoteAction(path, action string, args []string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	arg := func(i int) string {
		if i < len(args) {
			return strings.TrimSpace(args[i])
		}
		return ""
	}
	ctx := context.Background()
	switch action {
	case "add":
		err = r.Git.AddRemote(ctx, arg(0), arg(1))
	case "rename":
		err = r.Git.RenameRemote(ctx, arg(0), arg(1))
	case "seturl":
		err = r.Git.SetRemoteURL(ctx, arg(0), arg(1))
	case "remove":
		err = r.Git.RemoveRemote(ctx, arg(0))
	default:
		return userError("Unknown remote action " + action + ".")
	}
	if errors.Is(err, git.ErrInvalidName) {
		return userError("That remote name isn't allowed. Use letters, digits, '-', '_' or '.', and don't start with '-'.")
	}
	return branchErr(err)
}

// rejectedPushText says whether git's message is a push refused for being behind the remote.
func rejectedPushText(s string) bool {
	return strings.Contains(s, "[rejected]") || strings.Contains(s, "fetch first") || strings.Contains(s, "non-fast-forward")
}

// isRejectedPush is rejectedPushText for an error from git.
func isRejectedPush(err error) bool {
	var gerr *git.Error
	return errors.As(err, &gerr) && rejectedPushText(gerr.Stderr)
}

// netErr explains common network failures.
func netErr(err error) error {
	var gerr *git.Error
	if !errors.As(err, &gerr) {
		return writeErr(err)
	}
	s := gerr.Stderr
	switch {
	case strings.Contains(s, "Authentication failed"), strings.Contains(s, "could not read Username"),
		strings.Contains(s, "Invalid username or password"):
		return userError("Authentication failed. Check your credentials (or Git Credential Manager) and try again.\n\n" + string(gitMessage(s)))
	case strings.Contains(s, "returned error: 403"):
		return userError("The server refused this operation (HTTP 403). Your account may not have permission to push to or read this repository.")
	case strings.Contains(s, "returned error: 401"):
		return userError("Authentication failed (HTTP 401). Check your credentials and try again.")
	case strings.Contains(s, "Permission denied (publickey"):
		return userError("The server refused your SSH key. Make sure your key is loaded in ssh-agent and added to your account.\n\n" + string(gitMessage(s)))
	case strings.Contains(s, "Host key verification failed"):
		return userError("The server's SSH host key wasn't trusted, so the connection was stopped.")
	case strings.Contains(s, "Repository not found"), strings.Contains(s, "' does not exist"),
		strings.Contains(s, "does not appear to be a git repository"),
		strings.Contains(s, "repository '") && strings.Contains(s, "' not found"):
		return userError("No repository was found at that URL. Check the URL, and that your account can access the repository.")
	case strings.Contains(s, "Could not resolve host"), strings.Contains(s, "unable to access"):
		return userError("Couldn't reach the remote. Check the URL and your network connection.\n\n" + string(gitMessage(s)))
	case rejectedPushText(s):
		return userError("The remote has commits you don't have. Pull first, or force push (with lease) to overwrite them.")
	case strings.Contains(s, "stale info"):
		return userError("The remote branch moved since you last fetched, so force push was refused to protect it. Fetch, review, then try again.")
	case strings.Contains(s, "Not possible to fast-forward"), strings.Contains(s, "Diverging branches"):
		return userError("Your branch and its upstream have diverged. Pull with merge or rebase instead of fast-forward only.")
	}
	if strings.TrimSpace(s) != "" {
		return gitMessage(s)
	}
	return err
}
