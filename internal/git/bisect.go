package git

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"strings"
)

// Bisect terms: what a commit was found to be.
const (
	BisectGood = "good"
	BisectBad  = "bad"
	BisectSkip = "skip"
)

// BisectMark is one answer given during a bisect.
type BisectMark struct {
	Commit string
	// Term is BisectGood, BisectBad or BisectSkip.
	Term string
	// Step is 0 for the commits the bisect started from, then 1, 2, … for each answer.
	Step int
}

// BisectState is a bisect as git records it. Everything here is read from the state git
// keeps in the repository, so it's the same whoever started the bisect, and survives the
// app being closed.
type BisectState struct {
	Running bool
	// Current is the commit checked out for testing; empty once the answer is known.
	Current string
	Marks   []BisectMark
	// Remaining is how many commits are still in question besides the one under test, and
	// StepsLeft about how many more answers that takes.
	Remaining int
	StepsLeft int
	// FirstBad is the commit that introduced the problem, once found.
	FirstBad string
	// Candidates are the commits it could be when skipped commits make it impossible to say
	// which: the answer is one of these.
	Candidates []string
	// Origin is where the bisect was started from, a branch name or a commit, and where
	// stopping returns to.
	Origin string
	// InQuestion lists the commits that could still be the first bad one, newest first, up to
	// BisectRangeCap of them; RangeCapped says the list was cut short.
	InQuestion  []string
	RangeCapped bool
}

// BisectRangeCap is the most commits BisectState lists as still in question.
const BisectRangeCap = 5000

// Steps returns how many answers have been given since the bisect started.
func (s BisectState) Steps() int {
	n := 0
	for _, m := range s.Marks {
		n = max(n, m.Step)
	}
	return n
}

// bisectCommand is one command from the bisect log: its verb and its arguments.
type bisectCommand struct {
	verb string
	args []string
}

// parseBisectLog reads git's BISECT_LOG: the commands that were run, and the conclusions
// git wrote as comments.
func parseBisectLog(log string) (cmds []bisectCommand, firstBad string, candidates []string) {
	for line := range strings.SplitSeq(log, "\n") {
		line = strings.TrimSpace(line)
		// Git writes its conclusions as comments, with the term quoted since 2.39:
		// "# first 'bad' commit: [id] subject", "# possible first 'bad' commit: [id] subject".
		switch {
		case strings.HasPrefix(line, "# first ") && strings.Contains(line, " commit: ["):
			firstBad = bracketed(line)
		case strings.HasPrefix(line, "# possible first ") && strings.Contains(line, " commit: ["):
			candidates = append(candidates, bracketed(line))
		case strings.HasPrefix(line, "git bisect "):
			f := strings.Fields(strings.TrimPrefix(line, "git bisect "))
			if len(f) == 0 {
				continue
			}
			c := bisectCommand{verb: f[0]}
			for _, a := range f[1:] {
				c.args = append(c.args, strings.Trim(a, `'"`))
			}
			cmds = append(cmds, c)
		}
	}
	return cmds, firstBad, candidates
}

// bracketed returns what's inside the first [ ] of a log comment: a commit id.
func bracketed(line string) string {
	i, j := strings.Index(line, "["), strings.Index(line, "]")
	if i < 0 || j < i {
		return ""
	}
	return line[i+1 : j]
}

// marksFrom turns the log's commands into marks. The commits a bisect starts from are step
// 0; each answer after both a good and a bad commit are known is a step.
func marksFrom(cmds []bisectCommand) []BisectMark {
	var marks []BisectMark
	haveGood, haveBad := false, false
	step := 0
	add := func(term string, commits []string) {
		started := haveGood && haveBad
		if started {
			step++
		}
		for _, c := range commits {
			m := BisectMark{Commit: c, Term: term}
			if started {
				m.Step = step
			}
			marks = append(marks, m)
		}
		haveGood = haveGood || term == BisectGood
		haveBad = haveBad || term == BisectBad
	}
	for _, c := range cmds {
		switch c.verb {
		case "start":
			// `git bisect start <bad> <good>... [-- paths]`: the first is bad, the rest good.
			var revs []string
			for _, a := range c.args {
				if a == "--" {
					break
				}
				if !strings.HasPrefix(a, "-") {
					revs = append(revs, a)
				}
			}
			if len(revs) > 0 {
				add(BisectBad, revs[:1])
			}
			if len(revs) > 1 {
				add(BisectGood, revs[1:])
			}
		case BisectGood, BisectBad, BisectSkip, "old", "new":
			term := c.verb
			if term == "old" {
				term = BisectGood
			}
			if term == "new" {
				term = BisectBad
			}
			if len(c.args) > 0 {
				add(term, c.args)
			}
		}
	}
	return marks
}

// BisectState reads the bisect in progress in the repository, if there is one.
func (r *Repo) BisectState(ctx context.Context, gitDir string) (BisectState, error) {
	log, err := os.ReadFile(filepath.Join(gitDir, "BISECT_LOG"))
	if errors.Is(err, os.ErrNotExist) {
		return BisectState{}, nil
	}
	if err != nil {
		return BisectState{}, err
	}
	cmds, firstBad, candidates := parseBisectLog(string(log))
	st := BisectState{Running: true, Marks: marksFrom(cmds), FirstBad: firstBad, Candidates: candidates}
	if b, err := os.ReadFile(filepath.Join(gitDir, "BISECT_START")); err == nil {
		st.Origin = strings.TrimSpace(string(b))
	}
	if st.FirstBad != "" || len(st.Candidates) > 0 {
		return st, nil
	}
	if head, err := r.ReadString(ctx, "rev-parse", "HEAD"); err == nil {
		st.Current = head
	}
	// What's still in question: everything the latest bad commit has that no good commit
	// does. That set includes the bad commit itself, which may yet be the answer.
	bad, err := r.ReadString(ctx, "rev-parse", "--verify", "--quiet", "refs/bisect/bad")
	if err != nil || bad == "" {
		return st, nil
	}
	args := []string{"rev-list", "--count", bad}
	goods, err := r.ReadString(ctx, "for-each-ref", "--format=%(objectname)", "refs/bisect/good-*")
	if err != nil {
		return st, nil
	}
	if strings.TrimSpace(goods) == "" {
		return st, nil
	}
	for g := range strings.FieldsSeq(goods) {
		args = append(args, "^"+g)
	}
	var n int
	if out, err := r.ReadString(ctx, args...); err == nil {
		_, _ = fmt.Sscan(out, &n)
	}
	st.Remaining, st.StepsLeft = bisectLeft(n)
	list := append([]string{"rev-list", fmt.Sprintf("--max-count=%d", BisectRangeCap+1)}, args[2:]...)
	if out, err := r.ReadString(ctx, list...); err == nil {
		st.InQuestion = strings.Fields(out)
		if len(st.InQuestion) > BisectRangeCap {
			st.InQuestion, st.RangeCapped = st.InQuestion[:BisectRangeCap], true
		}
	}
	return st, nil
}

// bisectLeft says, for n commits that could be the first bad one, how many are left to test
// besides the one checked out and about how many answers that takes: the way git puts it in
// "N revisions left to test after this (roughly M steps)".
func bisectLeft(n int) (remaining, steps int) {
	if n <= 1 {
		return 0, 0
	}
	remaining = n/2 - 1 + n%2
	if n == 2 {
		remaining = 0
	}
	return max(remaining, 0), bits.Len(uint(n - 1))
}

// BisectRange says how many commits lie between a good and a bad commit, and about how many
// answers a bisect of them takes.
func (r *Repo) BisectRange(ctx context.Context, bad, good string) (commits, steps int, err error) {
	out, err := r.ReadString(ctx, "rev-list", "--count", good+".."+bad)
	if err != nil {
		return 0, 0, err
	}
	if _, err := fmt.Sscan(out, &commits); err != nil {
		return 0, 0, err
	}
	if commits <= 1 {
		return commits, 0, nil
	}
	return commits, bits.Len(uint(commits - 1)), nil
}

// ErrBisectOrder means the good commit isn't in the bad commit's history, so there is no
// range between them to search.
var ErrBisectOrder = errors.New("the good commit isn't in the bad commit's history")

// BisectStart begins a bisect between a bad commit and a good one, and checks out the first
// commit to test. If it can't get going, the repository is put back as it was.
func (r *Repo) BisectStart(ctx context.Context, bad, good string) error {
	if _, err := r.Read(ctx, "merge-base", "--is-ancestor", good, bad); err != nil {
		var gerr *Error
		if errors.As(err, &gerr) && gerr.ExitCode == 1 {
			return ErrBisectOrder
		}
		return err
	}
	steps := [][]string{{"bisect", "start"}, {"bisect", "bad", bad}, {"bisect", "good", good}}
	for i, args := range steps {
		if _, err := r.Write(ctx, Cmd{Args: args}); err != nil {
			if i > 0 {
				_, _ = r.Write(ctx, Cmd{Args: []string{"bisect", "reset"}})
			}
			return err
		}
	}
	return nil
}

// BisectMark answers for the commit checked out: BisectGood, BisectBad or BisectSkip. When
// a skip leaves only skipped commits to test, git stops with an error after recording the
// commits it could be; that is an answer, not a failure, and is reported as such.
func (r *Repo) BisectMark(ctx context.Context, gitDir, term string) error {
	switch term {
	case BisectGood, BisectBad, BisectSkip:
	default:
		return fmt.Errorf("unknown bisect answer %q", term)
	}
	_, err := r.Write(ctx, Cmd{Args: []string{"bisect", term}})
	if err != nil {
		if log, rerr := os.ReadFile(filepath.Join(gitDir, "BISECT_LOG")); rerr == nil {
			if _, _, candidates := parseBisectLog(string(log)); len(candidates) > 0 {
				return nil
			}
		}
	}
	return err
}

// ErrNothingToUndo means no answer has been given since the bisect started.
var ErrNothingToUndo = errors.New("no answer has been given yet")

// BisectUndo takes back the last answer, by replaying the bisect without it, and checks out
// the commit that answer was about again.
func (r *Repo) BisectUndo(ctx context.Context, gitDir string) error {
	log, err := os.ReadFile(filepath.Join(gitDir, "BISECT_LOG"))
	if err != nil {
		return err
	}
	cmds, _, _ := parseBisectLog(string(log))
	marks := marksFrom(cmds)
	if len(marks) == 0 || marks[len(marks)-1].Step == 0 {
		return ErrNothingToUndo
	}
	// Keep every command line but the last answer.
	var lines []string
	for line := range strings.SplitSeq(string(log), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "git bisect ") {
			lines = append(lines, line)
		}
	}
	lines = lines[:len(lines)-1]
	f, err := os.CreateTemp("", "gitrevenge-bisect-*.log")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if _, err := r.Write(ctx, Cmd{Args: []string{"bisect", "reset"}}); err != nil {
		return err
	}
	_, err = r.Write(ctx, Cmd{Args: []string{"bisect", "replay", f.Name()}})
	return err
}

// BisectSuggestGood proposes a commit to call good when bisecting from bad: the nearest tag
// before it. It returns the tag's name, or "" when there isn't one.
func (r *Repo) BisectSuggestGood(ctx context.Context, bad string) string {
	// A tag on the bad commit itself is no use: start from its parent.
	tag, err := r.ReadString(ctx, "describe", "--tags", "--abbrev=0", bad+"^")
	if err != nil {
		return ""
	}
	return tag
}
