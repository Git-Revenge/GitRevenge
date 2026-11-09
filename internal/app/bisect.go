package app

import (
	"context"
	"errors"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// BisectMark is one answer given during a bisect, with the commit's subject for showing.
type BisectMark struct {
	Commit  string `json:"commit"`
	Subject string `json:"subject"`
	// Term is "good", "bad" or "skip".
	Term string `json:"term"`
	// Step is 0 for the commits the bisect started from, then 1, 2, … for each answer.
	Step int `json:"step"`
}

// BisectInfo is the bisect in progress in a repository, if any (issue #591). It is read from
// what git keeps in the repository, so it's the same whoever started it.
type BisectInfo struct {
	Running bool `json:"running"`
	// Current is the commit checked out for testing; empty once the answer is known.
	Current string       `json:"current"`
	Marks   []BisectMark `json:"marks"`
	// Steps is how many answers have been given; Remaining how many commits are still in
	// question besides the one under test; StepsLeft about how many more answers that takes.
	Steps     int `json:"steps"`
	Remaining int `json:"remaining"`
	StepsLeft int `json:"stepsLeft"`
	// FirstBad is the commit that introduced the problem, once found. Candidates are the
	// commits it could be when skipped commits make it impossible to say which.
	FirstBad   string   `json:"firstBad"`
	Candidates []string `json:"candidates"`
	// Origin is the branch or commit the bisect started from, where stopping returns to.
	Origin string `json:"origin"`
	// InQuestion lists the commits that could still be the first bad one, newest first, for
	// marking them in the graph. RangeCapped is set when there were too many to list.
	InQuestion  []string `json:"inQuestion"`
	RangeCapped bool     `json:"rangeCapped"`
}

// BisectPlan is what a bisect between two commits would involve, for the start dialog.
type BisectPlan struct {
	// Bad and Good are the commits as full ids. Good is empty when none was given and none
	// could be suggested.
	Bad  string `json:"bad"`
	Good string `json:"good"`
	// BadSubject and GoodSubject are their subject lines.
	BadSubject  string `json:"badSubject"`
	GoodSubject string `json:"goodSubject"`
	// Suggested names the tag Good came from when it was suggested rather than asked for.
	Suggested string `json:"suggested"`
	// Commits is how many lie after Good up to and including Bad; Steps about how many
	// answers finding the first bad one takes.
	Commits int `json:"commits"`
	Steps   int `json:"steps"`
	// First is the commit that would be tested first, and its subject.
	First        string `json:"first"`
	FirstSubject string `json:"firstSubject"`
	// Problem says why these two can't be bisected, in words for the dialog; empty when
	// they can.
	Problem string `json:"problem"`
}

func (a *App) bisectRepo(path string) (*session.Repo, error) {
	m, err := a.sessions()
	if err != nil {
		return nil, err
	}
	r := m.Get(path)
	if r == nil {
		return nil, userError("That repository isn't open.")
	}
	return r, nil
}

// subjects returns the subject line of each commit, by id. Commits it can't read are left out.
func subjects(ctx context.Context, g *git.Repo, ids []string) map[string]string {
	out := map[string]string{}
	if len(ids) == 0 {
		return out
	}
	b, err := g.Read(ctx, append([]string{"log", "--no-walk=unsorted", "--format=%H%x00%s"}, ids...)...)
	if err != nil {
		return out
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if id, subject, ok := strings.Cut(line, "\x00"); ok {
			out[id] = subject
		}
	}
	return out
}

// Bisect returns the bisect in progress in the repository at path, if there is one.
func (a *App) Bisect(path string) (BisectInfo, error) {
	r, err := a.bisectRepo(path)
	if err != nil {
		return BisectInfo{}, err
	}
	ctx := context.Background()
	st, err := r.Git.BisectState(ctx, r.GitDir)
	if err != nil {
		return BisectInfo{}, friendly(err)
	}
	info := BisectInfo{
		Running: st.Running, Current: st.Current, Steps: st.Steps(), Remaining: st.Remaining, StepsLeft: st.StepsLeft,
		FirstBad: st.FirstBad, Candidates: nonNil(st.Candidates), Origin: st.Origin, Marks: []BisectMark{},
		InQuestion: nonNil(st.InQuestion), RangeCapped: st.RangeCapped,
	}
	ids := make([]string, 0, len(st.Marks))
	for _, m := range st.Marks {
		ids = append(ids, m.Commit)
	}
	names := subjects(ctx, r.Git, ids)
	for _, m := range st.Marks {
		// The log can name a commit by an abbreviation or a ref; show what was written.
		subject := names[m.Commit]
		info.Marks = append(info.Marks, BisectMark{Commit: m.Commit, Subject: subject, Term: m.Term, Step: m.Step})
	}
	return info, nil
}

// PlanBisect says what bisecting between bad and good would involve. An empty bad means the
// checked-out commit; an empty good asks for a suggestion, the nearest tag before bad.
func (a *App) PlanBisect(path, bad, good string) (BisectPlan, error) {
	r, err := a.bisectRepo(path)
	if err != nil {
		return BisectPlan{}, err
	}
	ctx := context.Background()
	if strings.TrimSpace(bad) == "" {
		bad = "HEAD"
	}
	var plan BisectPlan
	resolve := func(rev string) string {
		id, err := r.Git.ReadString(ctx, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
		if err != nil {
			return ""
		}
		return id
	}
	if plan.Bad = resolve(bad); plan.Bad == "" {
		plan.Problem = "Git can't find the commit " + bad + "."
		return plan, nil
	}
	if strings.TrimSpace(good) == "" {
		plan.Suggested = r.Git.BisectSuggestGood(ctx, plan.Bad)
		good = plan.Suggested
	}
	if good != "" {
		if plan.Good = resolve(good); plan.Good == "" {
			plan.Problem = "Git can't find the commit " + good + "."
		}
	}
	names := subjects(ctx, r.Git, []string{plan.Bad, plan.Good})
	plan.BadSubject, plan.GoodSubject = names[plan.Bad], names[plan.Good]
	if plan.Good == "" || plan.Problem != "" {
		return plan, nil
	}
	if plan.Good == plan.Bad {
		plan.Problem = "Good and bad are the same commit. Pick an older commit where it still worked."
		return plan, nil
	}
	if _, err := r.Git.Read(ctx, "merge-base", "--is-ancestor", plan.Good, plan.Bad); err != nil {
		plan.Problem = "The good commit isn't in the bad commit's history, so there's nothing between them to search. Pick a good commit that the bad one is built on."
		return plan, nil
	}
	if plan.Commits, plan.Steps, err = r.Git.BisectRange(ctx, plan.Bad, plan.Good); err != nil {
		return plan, friendly(err)
	}
	// The commit git would check out first: the middle of the range.
	if out, err := r.Git.ReadString(ctx, "rev-list", "--bisect", plan.Bad, "^"+plan.Good); err == nil && out != "" {
		plan.First = out
		plan.FirstSubject = subjects(ctx, r.Git, []string{out})[out]
	}
	return plan, nil
}

// StartBisect begins a bisect between a bad commit and a good one in the repository at path.
func (a *App) StartBisect(path, bad, good string) error {
	r, err := a.bisectRepo(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(bad) == "" || strings.TrimSpace(good) == "" {
		return userError("A bisect needs one commit where it's broken and one where it worked.")
	}
	ctx := context.Background()
	if op, _, _ := git.DetectOperation(r.GitDir); op != git.None {
		return userError("Finish or stop the " + string(op) + " in progress first.")
	}
	err = r.Git.BisectStart(ctx, bad, good)
	if errors.Is(err, git.ErrBisectOrder) {
		return userError("The good commit isn't in the bad commit's history, so there's nothing between them to search.")
	}
	return friendly(err)
}

// MarkBisect answers for the commit under test: "good", "bad" or "skip".
func (a *App) MarkBisect(path, term string) error {
	r, err := a.bisectRepo(path)
	if err != nil {
		return err
	}
	if op, _, _ := git.DetectOperation(r.GitDir); op != git.Bisect {
		return userError("No bisect is in progress.")
	}
	switch term {
	case git.BisectGood, git.BisectBad, git.BisectSkip:
	default:
		return userError("Answer good, bad or skip.")
	}
	return friendly(r.Git.BisectMark(context.Background(), r.GitDir, term))
}

// UndoBisect takes back the last answer and checks out the commit it was about again.
func (a *App) UndoBisect(path string) error {
	r, err := a.bisectRepo(path)
	if err != nil {
		return err
	}
	err = r.Git.BisectUndo(context.Background(), r.GitDir)
	if errors.Is(err, git.ErrNothingToUndo) {
		return userError("There's no answer to take back yet.")
	}
	return friendly(err)
}
