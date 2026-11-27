package git

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Two versions of a branch compared with git range-diff (#1038): each old commit paired with
// its new one, and what became of it.

// Kinds of pair, from range-diff's marks and what its diff-of-diffs holds.
const (
	PairSame    = "same"    // =: the same change and message
	PairChanged = "changed" // !: the change itself differs
	PairAround  = "around"  // !: only lines around the change differ, as something before it changed
	PairMessage = "message" // !: only the message (or its author) differs
	PairDropped = "dropped" // <: only in the old version
	PairFolded  = "folded"  // <: a fixup!, squash! or amend! whose target changed
	PairNew     = "new"     // >: only in the new version
)

// RangeSide is one version of a branch: the commits from Base to Tip, as Base..Tip.
type RangeSide struct {
	Base string `json:"base"`
	Tip  string `json:"tip"`
}

func (s RangeSide) spec() string { return s.Base + ".." + s.Tip }

// RangePair is one row: an old commit and the new one it became, or either alone.
type RangePair struct {
	// Old and New are the commits' places in each version, from 1, or 0 when it has none.
	Old int `json:"old"`
	New int `json:"new"`
	// OldID and NewID are the commits' full ids, or "".
	OldID string `json:"oldId"`
	NewID string `json:"newId"`
	// Subject is the new commit's, or the old one's when it has no new one. OldSubject is the
	// old one's when it differs.
	Subject    string `json:"subject"`
	OldSubject string `json:"oldSubject,omitempty"`
	Kind       string `json:"kind"`
	// Into is, for a folded commit, the new place of the commit it went into.
	Into int `json:"into,omitempty"`
	// Moved says its place among the commits in both versions changed.
	Moved bool `json:"moved,omitempty"`
	// Text is range-diff's own lines for the pair: its header, then its diff-of-diffs.
	Text string `json:"text"`
}

// RangeCount counts a side's commits as range-diff reads them, merges left out, and the merges.
func (r *Repo) RangeCount(ctx context.Context, side RangeSide) (commits, merges int, err error) {
	if err := CheckRevs(side.Base, side.Tip); err != nil {
		return 0, 0, err
	}
	n, err := r.ReadString(ctx, "rev-list", "--count", "--no-merges", side.spec())
	if err != nil {
		return 0, 0, err
	}
	m, err := r.ReadString(ctx, "rev-list", "--count", "--merges", side.spec())
	if err != nil {
		return 0, 0, err
	}
	commits, _ = strconv.Atoi(n)
	merges, _ = strconv.Atoi(m)
	return commits, merges, nil
}

// RangeDiff pairs old's commits with new's with git range-diff. Loose matches commits that
// were reworked more (a creation factor of 100, git's own is 60). It returns the pairs and
// range-diff's whole text.
func (r *Repo) RangeDiff(ctx context.Context, old, newer RangeSide, loose bool) ([]RangePair, string, error) {
	if err := CheckRevs(old.Base, old.Tip, newer.Base, newer.Tip); err != nil {
		return nil, "", err
	}
	args := []string{"range-diff", "--no-color"}
	if loose {
		args = append(args, "--creation-factor=100")
	}
	out, err := r.readCmd(ctx, Cmd{Args: append(args, old.spec(), newer.spec()), Env: noLazyFetch})
	if err != nil {
		return nil, "", err
	}
	text := string(out)
	pairs := ParseRangeDiff(text)
	// range-diff names commits by short ids: each side's list gives their full ones.
	oldIDs, err := r.rangeIDs(ctx, old)
	if err != nil {
		return nil, "", err
	}
	newIDs, err := r.rangeIDs(ctx, newer)
	if err != nil {
		return nil, "", err
	}
	for i := range pairs {
		p := &pairs[i]
		var oldSubject, newSubject string
		p.OldID, oldSubject = fullID(oldIDs, p.OldID)
		p.NewID, newSubject = fullID(newIDs, p.NewID)
		// range-diff names a pair by its old subject: a reworded commit reads by its new one.
		if newSubject != "" {
			p.Subject = newSubject
			if oldSubject != "" && oldSubject != newSubject {
				p.OldSubject = oldSubject
			}
		}
	}
	return pairs, text, nil
}

// rangeCommit is a commit of a side: its full id and its subject.
type rangeCommit struct{ id, subject string }

func (r *Repo) rangeIDs(ctx context.Context, side RangeSide) ([]rangeCommit, error) {
	out, err := r.ReadString(ctx, "log", "--no-merges", "--format=%H%x00%s", side.spec(), "--")
	if err != nil || out == "" {
		return nil, err
	}
	var commits []rangeCommit
	for line := range strings.SplitSeq(out, "\n") {
		id, subject, _ := strings.Cut(line, "\x00")
		commits = append(commits, rangeCommit{id, subject})
	}
	return commits, nil
}

func fullID(commits []rangeCommit, short string) (id, subject string) {
	if short == "" {
		return "", ""
	}
	for _, c := range commits {
		if strings.HasPrefix(c.id, short) {
			return c.id, c.subject
		}
	}
	return short, ""
}

// pairHeader is a pair's first line: "1:  1637a6b ! 1:  f6df5ac Subject", with - for a side
// the commit isn't in.
var pairHeader = regexp.MustCompile(`^\s*(\d+|-+):\s+([0-9a-f]+|-+)\s+([=!<>])\s+(\d+|-+):\s+([0-9a-f]+|-+)\s?(.*)$`)

// ParseRangeDiff reads range-diff's --no-color output into pairs, in its order, each with its
// kind. Ids are as range-diff prints them, short.
func ParseRangeDiff(text string) []RangePair {
	var pairs []RangePair
	var body []string
	flush := func() {
		if len(pairs) == 0 {
			return
		}
		p := &pairs[len(pairs)-1]
		if len(body) > 0 {
			p.Text += "\n" + strings.Join(body, "\n")
		}
		if p.Kind == PairChanged {
			p.Kind = changeKind(body)
		}
		body = nil
	}
	for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
		m := pairHeader.FindStringSubmatch(line)
		if m == nil {
			body = append(body, line)
			continue
		}
		flush()
		p := RangePair{Old: place(m[1]), New: place(m[4]), OldID: id(m[2]), NewID: id(m[5]), Subject: m[6], Text: line}
		switch m[3] {
		case "=":
			p.Kind = PairSame
		case "!":
			p.Kind = PairChanged
		case "<":
			p.Kind = PairDropped
		case ">":
			p.Kind = PairNew
		}
		pairs = append(pairs, p)
	}
	flush()
	markFolded(pairs)
	markMoved(pairs)
	return pairs
}

func place(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func id(s string) string {
	if strings.Trim(s, "-") == "" {
		return ""
	}
	return s
}

// changeKind reads a changed pair's diff-of-diffs. Each line is indented four spaces, then an
// outer mark (this version against the old: space, + or -), then the commit's own diff line
// with its inner mark. "@@ Commit message", "@@ Metadata" and "@@ <file>" start sections. A
// pair whose differing lines are all in the message or metadata changed only its message; one
// whose differing lines in files are all context (or hunk headers, which move with them)
// changed only around it.
func changeKind(body []string) string {
	section := ""
	code, around, message := false, false, false
	for _, raw := range body {
		line, ok := strings.CutPrefix(raw, "    ")
		if !ok || line == "" {
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			section = strings.TrimPrefix(line, "@@ ")
			continue
		}
		outer, inner := line[0], line[1:]
		// Inside a section, " ## Commit message ##" and " ## src/cache.js ##" say which part
		// follows: range-diff starts a section where the first difference is, not where the
		// part does.
		if header, ok := strings.CutPrefix(strings.TrimLeft(inner, " "), "## "); ok && outer == ' ' {
			section = strings.TrimSuffix(header, " ##")
			continue
		}
		if outer != '+' && outer != '-' {
			continue
		}
		switch {
		case section == "Commit message" || section == "Metadata":
			message = true
		case inner == "" || inner[0] == ' ' || strings.HasPrefix(inner, "@@"):
			around = true
		default:
			code = true
		}
	}
	switch {
	case code:
		return PairChanged
	case message:
		return PairMessage
	case around:
		return PairAround
	}
	return PairChanged
}

// markFolded turns a dropped fixup!, squash! or amend! into folded when the commit it names
// is paired with a new one that changed.
func markFolded(pairs []RangePair) {
	for i, p := range pairs {
		if p.Kind != PairDropped {
			continue
		}
		target := ""
		for _, prefix := range []string{"fixup! ", "squash! ", "amend! "} {
			if t, ok := strings.CutPrefix(p.Subject, prefix); ok {
				target = t
				break
			}
		}
		if target == "" {
			continue
		}
		for _, q := range pairs {
			if q.Subject == target && q.New > 0 && q.Old > 0 && q.Kind != PairSame {
				pairs[i].Kind = PairFolded
				pairs[i].Into = q.New
				break
			}
		}
	}
}

// markMoved marks the pairs in both versions that moved: those outside the longest run that
// kept its order. One commit moved to the end is one moved, not every commit it passed.
func markMoved(pairs []RangePair) {
	var both []int
	for i, p := range pairs {
		if p.Old > 0 && p.New > 0 {
			both = append(both, i)
		}
	}
	slices.SortFunc(both, func(a, b int) int { return pairs[a].Old - pairs[b].Old })
	// The longest increasing run of new places, in old order (n is at most a few hundred).
	best := make([]int, len(both))
	prev := make([]int, len(both))
	end := -1
	for i := range both {
		best[i], prev[i] = 1, -1
		for j := range i {
			if pairs[both[j]].New < pairs[both[i]].New && best[j]+1 > best[i] {
				best[i], prev[i] = best[j]+1, j
			}
		}
		if end < 0 || best[i] > best[end] {
			end = i
		}
	}
	kept := map[int]bool{}
	for i := end; i >= 0; i = prev[i] {
		kept[both[i]] = true
	}
	for _, i := range both {
		if !kept[i] {
			pairs[i].Moved = true
		}
	}
}
