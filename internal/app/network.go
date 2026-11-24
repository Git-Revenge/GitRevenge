package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/askpass"
	"github.com/git-revenge/gitrevenge/internal/fetchlog"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/weblinks"
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

// Prompt is the payload of the "askpass:prompt" event. Text is what git, ssh or the server
// wrote; the rest is what the app knows about where the prompt came from.
type Prompt struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Kind   string `json:"kind"`
	Expiry int64  `json:"expiry"`
	// Op is the operation that's running: "fetch", "pull", "push" or "clone".
	Op string `json:"op"`
	// Repo is the name of the repository's folder.
	Repo string `json:"repo"`
	// Host is the server the prompt names, or "".
	Host string `json:"host"`
	// Unexpected means Host isn't a server the operation was set up to talk to.
	Unexpected bool `json:"unexpected"`
}

// NetProgress is the payload of the "net:progress" event.
type NetProgress struct {
	Path    string `json:"path"`
	Op      string `json:"op"`
	Phase   string `json:"phase"`
	Percent int    `json:"percent"`
	Line    string `json:"line"`
}

// askEnv returns the environment that routes an operation's credential prompts to this app,
// and done, to call when the operation ends: its prompts are answered only until then.
func (a *App) askEnv(from promptOrigin) (env []string, done func(), err error) {
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
		return nil, nil, n.initErr
	}
	return n.server.Open(n.exe, from)
}

// handlePrompt shows a prompt in the UI and waits for AnswerPrompt.
func (a *App) handlePrompt(ctx context.Context, text string, kind askpass.Kind, from any) (string, bool) {
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
	p := Prompt{ID: id, Text: strings.TrimSpace(text), Kind: string(kind), Expiry: deadline.Unix()}
	if o, ok := from.(promptOrigin); ok {
		o.describe(&p)
	}
	a.emit("askpass:prompt", p)
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
	hosts := func() []string { return remoteHosts(context.Background(), r.Git) }
	err = a.track(r.Path, r.Path, op, hosts, "Another fetch, pull or push is already running in this repository.",
		func(ctx context.Context, o git.NetOptions) error { return fn(ctx, r, o) })
	return ownLinksOnly(err, hosts)
}

// track runs fn as the network operation registered under key (busy is the message when one
// is already running there), reporting progress as "net:progress" events for path. hosts
// lists the servers it's set up to talk to, for its credential prompts.
func (a *App) track(key, path, op string, hosts func() []string, busy userError, fn func(ctx context.Context, o git.NetOptions) error) error {
	env, done, err := a.askEnv(promptOrigin{Op: op, Path: path, Hosts: hosts})
	if err != nil {
		return err
	}
	defer done()
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
	return a.remember(path, netProblem(op, err))
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
		before := r.Git.RemoteBranches(ctx)
		reached, err := a.fetchRemotes(ctx, r.Git, remote, o)
		if msg := droppedMessage(remote, before, r.Git.RemoteBranches(ctx)); msg != "" {
			a.notice(r.Path, msg)
		}
		// Notes come from the remotes that answered, whether or not all of them did.
		a.syncNotesIn(ctx, r, reached, o)
		return err
	})
}

// Pull integrates the current branch's upstream. mode is "", "ff-only", "merge" or "rebase".
func (a *App) Pull(path, mode string) error {
	return a.runNetwork(path, "pull", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		a.upstreams.remember(ctx, r.Path, r.Git)
		before := r.Git.RemoteBranches(ctx)
		if err := a.record(r, "Pull", kindMove, "", func() error { return r.Git.Pull(ctx, git.PullMode(mode), o) }); err != nil {
			return err
		}
		if msg := droppedMessage(pullRemote(ctx, r.Git), before, r.Git.RemoteBranches(ctx)); msg != "" {
			a.notice(r.Path, msg)
		}
		a.cleanupAfterPull(ctx, r)
		pulled := []string{pullRemote(ctx, r.Git)}
		a.noteFetched(ctx, r.Git, pulled)
		a.syncNotesIn(ctx, r, pulled, o)
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
		a.undo.touch(r.Path)
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
		if _, err := a.pushReported(ctx, r, p, o); err != nil {
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
		multi, err := a.pushReported(ctx, r, p, o)
		if err != nil {
			return pushRefusal(ctx, r.Git, branch, upstream, err)
		}
		a.pushNotesTo(ctx, r, p.Remote, o)
		if multi {
			// The result lists each place it went.
			return nil
		}
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

// pushTarget works out where a branch pushes, by git's own rules (git.PlanPush): its
// pushRemote, remote.pushDefault or upstream, and push.default for the name there. upstream is
// the short name of what it tracks now, or "". A push git would refuse is refused in words; one
// with nowhere set asks first, in the frontend, through PushPlan and PushBranchTo.
func pushTarget(ctx context.Context, g *git.Repo, branch string) (p git.PushOptions, upstream string, err error) {
	if remote, merge, err := g.BranchUpstream(ctx, branch); err == nil && remote != "." {
		upstream = remote + "/" + strings.TrimPrefix(merge, "refs/heads/")
	}
	plan, err := g.PlanPush(ctx, branch)
	switch {
	case err != nil:
		return p, upstream, err
	case plan.Refusal != "":
		return p, upstream, userError(plan.Refusal)
	case plan.Ask:
		return p, upstream, userError(branch + " has never been pushed and nothing says where to. Choose a remote for it, or set Push goes to in Settings, Pushing.")
	}
	return git.PushOptions{Remote: plan.Remote, Refspec: "refs/heads/" + branch + ":refs/heads/" + plan.Ref, SetUpstream: plan.SetUpstream}, upstream, nil
}

// PushPlan says where pushing a branch would go, for the Push button's words and for asking
// about a first push. branch "" means the checked-out branch.
func (a *App) PushPlan(path, branch string) (git.PushPlan, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.PushPlan{}, err
	}
	ctx := context.Background()
	if branch == "" {
		st, err := r.Git.Status(ctx)
		if err != nil {
			return git.PushPlan{}, err
		}
		if st.Head == "" {
			return git.PushPlan{Remotes: []string{}, Refusal: "HEAD is detached. Check out a branch to push it."}, nil
		}
		branch = st.Head
	}
	return r.Git.PlanPush(ctx, branch)
}

// PushBranchTo pushes a branch's first push to a remote chosen for it, under the same name,
// and makes it track what that creates.
func (a *App) PushBranchTo(path, branch, remote string) error {
	return a.runNetwork(path, "push", func(ctx context.Context, r *session.Repo, o git.NetOptions) error {
		if err := git.CheckRevs(branch, remote); err != nil {
			return err
		}
		p := git.PushOptions{Remote: remote, Refspec: "refs/heads/" + branch + ":refs/heads/" + branch, SetUpstream: true}
		if _, err := a.pushReported(ctx, r, p, o); err != nil {
			return pushRefusal(ctx, r.Git, branch, "", err)
		}
		a.pushNotesTo(ctx, r, remote, o)
		a.notice(r.Path, "Pushed "+branch+" to "+remote+", and it tracks "+remote+"/"+branch+" now.")
		return nil
	})
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
		_, err := a.pushReported(ctx, r, git.PushOptions{Remote: remote, Refspec: refspec}, o)
		return err
	})
}

// Remotes lists the repository's remotes, each with where it is, its fetch settings, where
// pushes go and when it was last fetched: by the app (see fetchlog) or by anything else, as
// far as FETCH_HEAD says, whichever is later.
func (a *App) Remotes(path string) ([]git.Remote, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return nil, err
	}
	remotes, err := r.Git.RemoteDetails(context.Background())
	if err != nil {
		return nil, err
	}
	noted := fetchlog.Read(a.fetchDir(), r.CommonDir)
	hidden := a.hiddenRemotes(path)
	for i := range remotes {
		remotes[i].Hidden = slices.Contains(hidden, remotes[i].Name)
		if at, ok := noted[remotes[i].Name]; ok && at.Unix() > remotes[i].FetchedAt {
			remotes[i].FetchedAt = at.Unix()
		}
	}
	return remotes, nil
}

// RemoteSettings returns the repository's settings for where pushes go and whether fetches
// prune, from every scope of git config.
func (a *App) RemoteSettings(path string) (git.RemoteSettings, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.RemoteSettings{}, err
	}
	return r.Git.RemoteSettings(context.Background()), nil
}

// IssueURL returns a pattern for links to the repository's issues, with "{n}" for the
// issue number, from origin (or the first remote). It's "" when the remote isn't on a forge
// known by its host or a forge rule, or the forge's issues aren't numbered pages.
func (a *App) IssueURL(path string) (string, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	remotes, err := r.Git.Remotes(ctx)
	if err != nil || len(remotes) == 0 {
		return "", err
	}
	pick := remotes[0]
	for _, r := range remotes {
		if r.Name == "origin" {
			pick = r
		}
	}
	site, ok := weblinks.For(pick.FetchURL, a.linkConfig(ctx, r.Git))
	if !ok {
		return "", nil
	}
	return site.Issues(), nil
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
		if err = r.Git.RenameRemote(ctx, arg(0), arg(1)); err == nil {
			fetchlog.Rename(a.fetchDir(), r.CommonDir, arg(0), arg(1))
			a.followRemote(path, arg(0), arg(1))
		}
	case "seturl":
		old := ""
		if rs, rerr := r.Git.Remotes(ctx); rerr == nil {
			for _, rm := range rs {
				if rm.Name == arg(0) {
					old = rm.FetchURL
				}
			}
		}
		if err = r.Git.SetRemoteURL(ctx, arg(0), arg(1)); err == nil && old != "" {
			// Push URLs that followed it go where it went.
			followURL(ctx, r.Git, old, arg(1))
		}
	case "remove":
		if err = r.Git.RemoveRemote(ctx, arg(0)); err == nil {
			fetchlog.Forget(a.fetchDir(), r.CommonDir, arg(0))
			a.followRemote(path, arg(0), "")
		}
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

// netErr explains a network failure whose operation the caller doesn't name.
func netErr(err error) error {
	return netProblem("", err)
}
