package session

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/git-revenge/gitrevenge/internal/branchkind"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/graph"
)

// DefaultReadAhead is how many rows the loader reads past the furthest row the UI has
// asked for before it pauses. Memory then grows with how far someone has scrolled, not
// with the size of the repository.
const DefaultReadAhead = 30000

// GraphProgress reports loading progress for a repository's commit graph.
type GraphProgress struct {
	Rows int
	Done bool
	// Paused means the reader stopped ahead of the viewport; scrolling further, jumping to
	// a commit or searching reads on.
	Paused bool
	// Estimate is about how many rows the whole history has (from git rev-list --count),
	// or 0 until it's known. Once Done, it equals Rows.
	Estimate int
	// Reset means a reloaded graph replaced the displayed one, so cached rows are stale.
	Reset bool
	// RefsReady is set on the update sent when refs finish loading during a first load,
	// so the UI can re-fetch rows to pick up their labels.
	RefsReady bool
	Err       error
}

// graphLoader owns a repository's commit graph and reloads it on demand. It streams
// `git log` into the graph and pauses the reader once it is ahead of what the UI needs;
// blocking the reader blocks git on its output pipe, so nothing more is read or parsed.
type graphLoader struct {
	repo       *git.Repo
	onProgress func(GraphProgress)
	ahead      int
	// rules returns the repository's current branch rules (hide patterns, kinds); nil means
	// the defaults.
	rules func() branchkind.Rules
	// stackOverrides returns the repository's stack parent overrides; nil skips stacks.
	stackOverrides func() map[string]string
	// onReload, if set, is called as each load of the graph starts: whatever else is kept
	// about the history may be behind too.
	onReload func()

	mu sync.Mutex
	// started is set once a first load has been asked for. A tab restored at launch isn't
	// loaded until it's shown or its turn comes to warm up (see Manager.warm).
	started bool
	// sig is where the refs stood just before the latest load read them (git.RefsState), or
	// "" while that isn't known; sigReady is closed once it is. See refresh.
	sig      string
	sigReady chan struct{}
	// mailmap is how the mailmap stood just before the latest load, as sig is for refs: a
	// row's author is the name the mailmap gave it when it was read.
	mailmap git.MailmapState
	// stopped is set by stop: the repository is closing, and nothing loads after that.
	stopped bool
	// showHidden puts hidden branches back in the graph, for this session only.
	showHidden bool
	// focus narrows the graph to one branch or stack, for this session only.
	focus Focus
	// filter narrows it to the commits a smart filter keeps, for this session only, and
	// looked is how many commits the load under it has looked through.
	filter *Filter
	looked int
	// times are the author times of every commit the last finished load under a filter
	// looked through, sorted: what a day held before the filter took commits out of it.
	times []int64
	// total is how many commits the history being loaded has, filter aside; 0 until counted.
	total   int
	current *graph.Graph
	// narrowed says current was loaded under a filter: it holds only what the filter keeps.
	// filter is what the next load will be under, which isn't always what's shown.
	narrowed bool
	refs     *RefIndex
	loading  bool
	paused   bool
	estimate int
	// want is the furthest row the UI has asked for.
	want int
	// unbounded counts callers that need the reader to keep going (finding a commit,
	// searching), so it doesn't pause while they wait.
	unbounded int
	// wake nudges a paused reader to check whether it may go on.
	wake chan struct{}
	// changed is closed and replaced whenever the graph advances, waking waiters.
	changed chan struct{}
	cancel  context.CancelFunc
	// wg tracks load goroutines so stop can wait for their git processes to exit.
	wg sync.WaitGroup
}

func newGraphLoader(repo *git.Repo, ahead int, onProgress func(GraphProgress)) *graphLoader {
	ready := make(chan struct{})
	close(ready)
	return &graphLoader{
		repo:       repo,
		onProgress: onProgress,
		ahead:      ahead,
		wake:       make(chan struct{}, 1),
		changed:    make(chan struct{}),
		sigReady:   ready,
	}
}

// RefIndex holds a repository's refs, indexed by the commit they point at.
type RefIndex struct {
	All      []git.Ref
	ByCommit map[string][]git.Ref
}

func newRefIndex(refs []git.Ref) *RefIndex {
	idx := &RefIndex{All: refs, ByCommit: make(map[string][]git.Ref, len(refs))}
	for _, r := range refs {
		idx.ByCommit[r.Commit] = append(idx.ByCommit[r.Commit], r)
	}
	return idx
}

// Focus narrows the graph to one branch, or to the whole stack it's in: the opposite of hiding
// a branch. As with hiding, the name has no remote, so its remote-tracking copies come too. The
// zero Focus is no focus.
type Focus struct {
	Branch string `json:"branch"`
	Stack  bool   `json:"stack"`
}

// annotate sets each branch's kind and whether the rules hide it. Unless they're being
// shown, hidden branches stay in All (the sidebar counts them) but get no label in the
// graph. A focused branch (focused holds full ref names) always gets its label.
func (idx *RefIndex) annotate(rules branchkind.Rules, remotes []string, show bool, focused map[string]bool) {
	idx.ByCommit = make(map[string][]git.Ref, len(idx.All))
	for i, r := range idx.All {
		name := r.Short
		switch r.Kind {
		case git.RemoteBranch:
			name = branchkind.Strip(r.Short, remotes)
			idx.All[i].Hidden = rules.Hidden(name) || rules.RemoteHidden(strings.TrimSuffix(r.Short, "/"+name))
			idx.All[i].BranchKind = string(rules.Kind(name))
		case git.LocalBranch:
			idx.All[i].Hidden = rules.Hidden(name)
			idx.All[i].BranchKind = string(rules.Kind(name))
		}
		idx.All[i].Focused = focused[r.Name]
		if !idx.All[i].Hidden || show || idx.All[i].Focused {
			idx.ByCommit[r.Commit] = append(idx.ByCommit[r.Commit], idx.All[i])
		}
	}
}

// SetShowHidden shows or hides the hidden branches again, and reloads the graph.
func (l *graphLoader) SetShowHidden(show bool) {
	l.mu.Lock()
	changed := l.showHidden != show
	l.showHidden = show
	l.mu.Unlock()
	if changed {
		l.reload()
	}
}

// ShowingHidden reports whether hidden branches are being shown.
func (l *graphLoader) ShowingHidden() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.showHidden
}

// SetFocus narrows the graph to focus, replacing any focus it had, and reloads it.
func (l *graphLoader) SetFocus(focus Focus) {
	l.mu.Lock()
	changed := l.focus != focus
	l.focus = focus
	l.mu.Unlock()
	if changed {
		l.reload()
	}
}

// Focus returns what the graph is focused on.
func (l *graphLoader) Focus() Focus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.focus
}

// Filter narrows the graph to some of its commits: the ones a smart filter keeps.
//
// The commits that stay are no longer neighbours, so the graph doesn't pretend they are:
// they're strung on one lane, newest first, each shown as the child of the next. Everything
// else that narrows the graph still does: a filter shows what passes it and the focus and
// the hidden branches too.
type Filter struct {
	// Keep holds the ids of the commits that stay.
	Keep map[string]struct{}
	// Sig tells one filter from another, so setting the same one again reloads nothing.
	Sig string
}

// SetFilter narrows the graph to what filter keeps, or with nil shows everything again, and
// reloads it if that's a change.
func (l *graphLoader) SetFilter(filter *Filter) {
	l.mu.Lock()
	was, now := "", ""
	if l.filter != nil {
		was = l.filter.Sig
	}
	if filter != nil {
		now = filter.Sig
	}
	changed := was != now || (l.filter == nil) != (filter == nil)
	l.filter = filter
	started := l.started
	l.mu.Unlock()
	if changed && started {
		l.reload()
	}
}

// Authored says how many commits of the history under the filter were authored in each of
// some spans of time, each a start and an end in Unix seconds, the end not included: what a
// day held before the filter took commits out of it. ok is false with no filter, or while
// the history is still being looked through.
func (l *graphLoader) Authored(spans [][2]int64) (counts []int, ok bool) {
	l.mu.Lock()
	times := l.times
	on := l.filter != nil
	l.mu.Unlock()
	if !on || times == nil {
		return nil, false
	}
	counts = make([]int, len(spans))
	for i, span := range spans {
		from, _ := slices.BinarySearch(times, span[0])
		to, _ := slices.BinarySearch(times, span[1])
		counts[i] = max(0, to-from)
	}
	return counts, true
}

// Filtered reports whether a filter is narrowing the graph, how many commits the load
// under it has looked through so far, kept or passed over, and how many the history has
// in all (0 until that's counted).
func (l *graphLoader) Filtered() (on bool, looked, total int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.filter != nil, l.looked, l.total
}

// focusedRefs returns the full names of the refs focus covers, which is none once its branch
// has gone.
func (l *graphLoader) focusedRefs(ctx context.Context, focus Focus, rules branchkind.Rules, remotes []string) (map[string]bool, error) {
	names := []string{focus.Branch}
	if focus.Stack && l.stackOverrides != nil {
		list, err := l.repo.Refs(ctx)
		if err != nil {
			return nil, err
		}
		names = StackMembers(focus.Branch, Stacks(ctx, l.repo, list, rules, l.stackOverrides()))
	}
	wanted := map[string]bool{}
	for _, n := range names {
		wanted["refs/heads/"+n] = true
		for _, rm := range remotes {
			wanted["refs/remotes/"+rm+"/"+n] = true
		}
	}
	args := []string{"for-each-ref", "--format=%(refname)"}
	for ref := range wanted {
		args = append(args, ref)
	}
	out, err := l.repo.ReadString(ctx, args...)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	// A pattern also matches what's under it, so refs/heads/a would bring in refs/heads/a/b.
	for ref := range strings.Lines(out) {
		if ref = strings.TrimSpace(ref); wanted[ref] {
			found[ref] = true
		}
	}
	return found, nil
}

// refResult is a load's refs, or the error reading them.
type refResult struct {
	idx *RefIndex
	err error
}

// start begins the first load, unless one has begun.
func (l *graphLoader) start() {
	l.mu.Lock()
	first := !l.started
	l.started = true
	l.mu.Unlock()
	if first {
		l.reload()
	}
}

// reloadStarted reloads a graph that has loaded before. One that hasn't will read
// everything as it is when its first load comes.
func (l *graphLoader) reloadStarted() {
	l.mu.Lock()
	started := l.started
	l.mu.Unlock()
	if started {
		l.reload()
	}
}

// refresh brings up to date a graph that nothing has been keeping so: a repository is only
// watched while it's on screen, so its refs may have moved since its graph loaded. It loads
// a graph that never has, and reloads one only if HEAD or a ref isn't where the last load
// found it, which two short git calls can say. Reading the history again is the expensive
// part, and a tab that hasn't changed should come back as it was.
func (l *graphLoader) refresh(ctx context.Context) {
	l.mu.Lock()
	first := !l.started
	l.started = true
	l.mu.Unlock()
	if first {
		l.reload()
		return
	}
	var was string
	var names git.MailmapState
	for {
		// A load that has only just begun hasn't said where the refs stood yet.
		l.mu.Lock()
		ready := l.sigReady
		l.mu.Unlock()
		select {
		case <-ready:
		case <-ctx.Done():
			return
		}
		l.mu.Lock()
		latest := l.sigReady == ready
		was = l.sig
		names = l.mailmap
		l.mu.Unlock()
		if latest {
			break
		}
	}
	if was != "" {
		now, err := l.repo.RefsState(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil && now == was && l.repo.MailmapState(ctx).Same(names) {
			return
		}
	}
	l.reload()
}

// mailmapMoved reports whether names may read differently from the way the displayed graph
// has them, after a change to the work tree or, with config, to the repository's settings.
// A change to the work tree only asks the file system, since most of them are someone
// saving a source file.
func (l *graphLoader) mailmapMoved(ctx context.Context, config bool) bool {
	l.mu.Lock()
	started, was := l.started, l.mailmap
	l.mu.Unlock()
	if !started {
		return false
	}
	if config {
		return !l.repo.MailmapState(ctx).Same(was)
	}
	return was.FilesMoved()
}

// settled waits until the graph has loaded as far as it will for now: to the end, or to
// where the reader pauses ahead of the viewport. It reports false if ctx ended first.
func (l *graphLoader) settled(ctx context.Context) bool {
	l.mu.Lock()
	for l.loading && !l.paused {
		changed := l.changed
		l.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return false
		}
		l.mu.Lock()
	}
	l.mu.Unlock()
	return true
}

// Refs returns the refs matching the displayed graph.
func (l *graphLoader) Refs() *RefIndex {
	l.start()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.refs == nil {
		return newRefIndex(nil)
	}
	return l.refs
}

// Graph returns the graph to display and whether more of it may still load (it is reading
// or paused). The graph may be partial. Asking starts the first load, if nothing has.
func (l *graphLoader) Graph() (*graph.Graph, bool) {
	l.start()
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.current, l.loading
}

// Status reports whether the reader is paused ahead of the viewport, and the estimated
// size of the whole history (0 until known).
func (l *graphLoader) Status() (paused bool, estimate int) {
	l.start()
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.paused, l.estimate
}

// Demand records that the UI wants rows up to row, so a paused reader reads on.
func (l *graphLoader) Demand(row int) {
	l.start()
	l.mu.Lock()
	defer l.mu.Unlock()
	if row > l.want {
		l.want = row
		l.nudgeLocked()
	}
}

// LoadUntil keeps the reader going past the read-ahead limit until found reports true for
// the displayed graph or the history ends, and returns that graph. A background reload
// keeps the old graph displayed, so found may be satisfied by it.
func (l *graphLoader) LoadUntil(ctx context.Context, found func(*graph.Graph) bool) (*graph.Graph, error) {
	g, _, err := l.loadUntil(ctx, found)
	return g, err
}

// loadUntil is LoadUntil, and says as well whether the graph it returns was loaded under a
// filter, as of the moment it was the one being shown.
func (l *graphLoader) loadUntil(ctx context.Context, found func(*graph.Graph) bool) (*graph.Graph, bool, error) {
	l.start()
	l.mu.Lock()
	l.unbounded++
	l.nudgeLocked()
	defer func() {
		l.mu.Lock()
		l.unbounded--
		l.mu.Unlock()
	}()
	for {
		g, loading, changed := l.current, l.loading, l.changed
		if g != nil && (!loading || found(g)) {
			narrowed := l.narrowed
			l.mu.Unlock()
			return g, narrowed, nil
		}
		l.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
		l.mu.Lock()
	}
}

// nudgeLocked wakes a paused reader, if there is one.
func (l *graphLoader) nudgeLocked() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// notifyLocked wakes everything waiting in LoadUntil.
func (l *graphLoader) notifyLocked() {
	close(l.changed)
	l.changed = make(chan struct{})
}

func (l *graphLoader) report(p GraphProgress) {
	if l.onProgress != nil {
		l.onProgress(p)
	}
}

// reload starts loading the graph, cancelling any load in flight. The first load streams
// into the visible graph so the first screen appears quickly; later loads build a fresh
// graph off-screen and swap it in once it reaches the read-ahead limit or ends, so a
// refresh never blanks the view.
func (l *graphLoader) reload() {
	if l.onReload != nil {
		l.onReload()
	}
	l.mu.Lock()
	if l.stopped {
		// A reload asked for as the tab closes: by a change the watcher saw a moment before,
		// or by the refresh that follows coming on screen.
		l.mu.Unlock()
		return
	}
	if l.cancel != nil {
		l.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel
	g := graph.New()
	if l.current == nil {
		l.current = g
		l.narrowed = l.filter != nil
	}
	l.loading = true
	l.started = true
	l.sig = ""
	sigReady := make(chan struct{})
	l.sigReady = sigReady
	show := l.showHidden
	focus := l.focus
	filter := l.filter
	l.looked = 0
	l.times = nil
	// Counted before the lock is let go, so that stop, which takes it to say nothing more
	// loads, never starts waiting with a load about to be added.
	l.wg.Add(1)
	l.mu.Unlock()
	rules := branchkind.New(nil, nil)
	if l.rules != nil {
		rules = l.rules()
	}

	go func() {
		defer l.wg.Done()
		// Where the refs stand, taken before anything below reads them, so what this load
		// shows is at least that new (see refresh). It runs beside the one read that doesn't
		// look at refs, so the first rows don't wait for it.
		sigCh := make(chan string, 1)
		go func() {
			sig, _ := l.repo.RefsState(ctx)
			sigCh <- sig
		}()
		// And where the mailmap stands, before the log below reads it for the same reason.
		names := l.repo.MailmapState(ctx)
		var remotes []string
		if rs, err := l.repo.Remotes(ctx); err == nil {
			for _, r := range rs {
				remotes = append(remotes, r.Name)
			}
		}
		sig := <-sigCh
		l.mu.Lock()
		if ctx.Err() == nil {
			l.sig = sig
			l.mailmap = names
		}
		l.mu.Unlock()
		close(sigReady)

		var focused map[string]bool
		if focus != (Focus{}) {
			var err error
			focused, err = l.focusedRefs(ctx, focus, rules, remotes)
			if ctx.Err() != nil {
				return // superseded or closed; the newer load reports
			}
			if err == nil && len(focused) == 0 {
				// Its branch has gone, so there's nothing left to focus on.
				l.mu.Lock()
				if l.focus == focus {
					l.focus = Focus{}
				}
				l.mu.Unlock()
			}
		}

		// Read refs alongside the log rather than before it: with thousands of refs they can
		// take longer than the first screen of history.
		refsCh := make(chan refResult, 1)
		go func() {
			list, err := l.repo.Refs(ctx)
			if err == nil && l.stackOverrides != nil {
				annotateStacks(list, Stacks(ctx, l.repo, list, rules, l.stackOverrides()))
			}
			idx := newRefIndex(list)
			idx.annotate(rules, remotes, show, focused)
			first := false
			l.mu.Lock()
			if l.refs == nil && ctx.Err() == nil {
				l.refs = idx
				first = true
			}
			l.mu.Unlock()
			if first {
				l.report(GraphProgress{Rows: g.Len(), RefsReady: true})
			}
			refsCh <- refResult{idx, err}
		}()
		var refs *refResult
		awaitRefs := func() refResult {
			if refs == nil {
				r := <-refsCh
				refs = &r
			}
			return *refs
		}

		// Stashes appear as single rows on their base commit. A stash commit's other parents
		// (its index and untracked-files snapshots) are internal: drop those edges and skip
		// the commits, which date order (children before parents) always yields after the stash itself.
		stashIDs, _ := l.repo.StashIDs(ctx)
		stashes := map[string]bool{}
		for _, id := range stashIDs {
			stashes[id] = true
		}
		hidden := map[string]bool{}
		// Under a filter there's one lane and nothing to pin to it.
		if filter == nil {
			pins, kinds := KindTips(ctx, l.repo, rules, remotes, show)
			g.Pin(pins)
			g.SetTipKinds(kinds)
		}
		opts := git.LogOptions{Extra: stashIDs}
		if len(focused) > 0 {
			// Only what the focus reaches: not HEAD, tags or stashes.
			opts = git.LogOptions{}
			for ref := range focused {
				opts.Revs = append(opts.Revs, ref)
			}
			slices.Sort(opts.Revs)
		} else if !show {
			opts.Exclude, opts.Remotes = rules.Hide(), remotes
		}

		// The size of the whole history, for the scrollbar while the reader is paused. It
		// slightly overcounts (stash snapshots are hidden), so the UI calls it approximate.
		var counted atomic.Int64
		go func() {
			n, err := l.repo.CountCommits(ctx, opts)
			if err != nil || ctx.Err() != nil {
				return
			}
			l.mu.Lock()
			if ctx.Err() == nil {
				l.total = n
			}
			l.mu.Unlock()
			// A filter keeps at most what it holds, however long the history is.
			if filter != nil {
				n = min(n, len(filter.Keep))
			}
			counted.Store(int64(n))
			l.mu.Lock()
			shown := l.current == g && l.loading
			if shown {
				l.estimate = n
			}
			paused := l.paused
			l.mu.Unlock()
			if shown {
				l.report(GraphProgress{Rows: g.Len(), Paused: paused, Estimate: n})
			}
		}()

		// swapLocked shows g in place of the graph it is reloading. The caller holds l.mu.
		swapLocked := func(r refResult) {
			l.current = g
			l.narrowed = filter != nil
			l.refs = r.idx
			l.estimate = int(counted.Load())
			l.notifyLocked()
		}

		const every = 100 * time.Millisecond
		last := time.Now()
		// Under a filter a kept commit is added once the next one is known, as its child:
		// held is the one waiting.
		var held *git.LogCommit
		looked := 0
		var times []int64
		err := l.repo.Log(ctx, opts, func(c git.LogCommit) error {
			if hidden[c.ID] {
				return nil
			}
			if stashes[c.ID] && len(c.Parents) > 1 {
				for _, p := range c.Parents[1:] {
					hidden[p] = true
				}
				c.Parents = c.Parents[:1]
			}
			if filter != nil {
				looked++
				times = append(times, c.AuthorTime)
				if looked%1024 == 0 {
					l.mu.Lock()
					l.looked = looked
					l.mu.Unlock()
				}
				if _, keep := filter.Keep[c.ID]; !keep {
					// Passed over. The reader still stops for breath, or a filter that keeps
					// little would read a long history to its end before showing anything.
					if err := ctx.Err(); err != nil {
						return err
					}
					return nil
				}
				kept := c
				before := held
				held = &kept
				if before == nil {
					return nil // the newest kept commit: it waits for the one after it
				}
				before.Parents = []string{kept.ID}
				c = *before
			}
			if err := g.Add(c); err != nil {
				return err
			}
			n := g.Len()
			if time.Since(last) >= every {
				last = time.Now()
				l.mu.Lock()
				shown := l.current == g
				if shown {
					l.notifyLocked()
				}
				est := l.estimate
				l.mu.Unlock()
				if shown {
					l.report(GraphProgress{Rows: n, Estimate: est})
				}
			}
			return l.pauseIfAhead(ctx, g, n, awaitRefs, swapLocked)
		})
		// The last commit kept has nothing after it.
		if err == nil && held != nil {
			held.Parents = nil
			err = g.Add(*held)
		}
		if filter != nil {
			l.mu.Lock()
			if ctx.Err() == nil && err == nil {
				l.looked = looked
				slices.Sort(times)
				l.times = times
			}
			l.mu.Unlock()
		}
		r := awaitRefs()
		if ctx.Err() != nil {
			return // superseded or closed; the newer load reports
		}
		if err == nil {
			err = r.err
		}
		l.mu.Lock()
		l.current = g
		l.narrowed = filter != nil
		l.refs = r.idx
		l.loading = false
		l.paused = false
		l.estimate = g.Len()
		l.cancel = nil
		l.notifyLocked()
		l.mu.Unlock()
		l.report(GraphProgress{Rows: g.Len(), Done: true, Estimate: g.Len(), Err: err})
	}()
}

// pauseIfAhead blocks the reader while g has n rows and is far enough ahead of what the
// UI and any waiters need. An off-screen reload that gets this far is swapped in first.
func (l *graphLoader) pauseIfAhead(ctx context.Context, g *graph.Graph, n int, awaitRefs func() refResult, swapLocked func(refResult)) error {
	for {
		l.mu.Lock()
		if l.unbounded > 0 || n < l.want+l.ahead {
			l.mu.Unlock()
			return nil
		}
		reset := false
		if l.current != g {
			l.mu.Unlock()
			r := awaitRefs() // before taking the lock: the refs goroutine needs it
			l.mu.Lock()
			swapLocked(r)
			reset = true
		}
		l.paused = true
		l.notifyLocked()
		est := l.estimate
		l.mu.Unlock()
		l.report(GraphProgress{Rows: n, Paused: true, Estimate: est, Reset: reset})
		select {
		case <-l.wake:
		case <-ctx.Done():
			return ctx.Err()
		}
		l.mu.Lock()
		l.paused = false
		l.mu.Unlock()
	}
}

// stop cancels any load and waits for it to finish, so no git process is left holding
// files in the repository (on Windows that would block deleting or moving it). Anything
// waiting in LoadUntil is released.
func (l *graphLoader) stop() {
	l.mu.Lock()
	l.stopped = true
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	l.mu.Unlock()
	l.wg.Wait()
	l.mu.Lock()
	l.loading = false
	l.paused = false
	l.notifyLocked()
	l.mu.Unlock()
}
