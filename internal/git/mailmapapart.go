package git

import (
	"slices"
	"strings"
)

// ApartPlan is the .mailmap text with one identity taken away from the person it was
// joined to. As with JoinPlan, nothing is written: the text becomes an edit in the People
// view.
type ApartPlan struct {
	// Text is the mailmap text afterwards.
	Text string `json:"text"`
	// Removed are the lines that joined the identity and are gone, by their line numbers
	// in the text as it was, 1-based. None means nothing in this file joined it.
	Removed []int `json:"removed"`
	// Added are lines written where a removed line was. A line can join several identities
	// at once, such as every name under one address; taking one of them apart mustn't take
	// the others, so each of the others gets a line of its own that names it.
	Added []string `json:"added"`
	// Kept are those others: still joined, now through Added.
	Kept []Identity `json:"kept"`
	// Lost are others that come apart as well, because no line could be written for them,
	// such as an identity with no name to tell it by.
	Lost []Identity `json:"lost"`
}

// PlanApart works out the text that takes who away from the person the .mailmap joins it
// to, and nobody else: the lines that decide who is are removed, and anyone else those
// lines joined is kept joined by a line that names them, put where the removed line was.
// read reads a .mailmap text the way git would (see MailmapReader), and history is every
// identity the history uses.
func PlanApart(text string, read func(text string) *Mailmap, history []HistoryIdentity, who Identity) ApartPlan {
	text = normalizeMailmapText(text)
	plan := ApartPlan{Text: text, Removed: []int{}, Added: []string{}, Kept: []Identity{}, Lost: []Identity{}}
	var lines []string
	if text != "" {
		lines = strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	before := read(text)
	_, by := before.Resolve(who)
	drop := map[int]bool{}
	for _, at := range by {
		if at.Source == MailmapWorkTree && at.Line >= 1 && at.Line <= len(lines) {
			drop[at.Line] = true
		}
	}
	if len(drop) == 0 {
		return plan
	}

	var kept []string
	at := -1 // where the first removed line was, in what's kept
	for i, l := range lines {
		if !drop[i+1] {
			kept = append(kept, l)
			continue
		}
		if at < 0 {
			at = len(kept)
		}
		plan.Removed = append(plan.Removed, i+1)
	}

	// Whoever else reads differently without those lines was joined by them too.
	after := read(joinMailmapLines(kept))
	apart, _ := after.Resolve(who)
	for _, h := range history {
		if h.Identity == who {
			continue
		}
		was, _ := before.Resolve(h.Identity)
		if now, _ := after.Resolve(h.Identity); now == was {
			continue
		}
		// A line that names this identity, so it can't catch the one being taken apart.
		line := was.Name + " <" + was.Email + "> " + h.Name + " <" + h.Email + ">"
		if joinable(was.Name) && joinable(was.Email) && joinable(h.Name) && joinable(h.Email) {
			next := slices.Insert(slices.Clone(kept), at, line)
			m := read(joinMailmapLines(next))
			back, _ := m.Resolve(h.Identity)
			if still, _ := m.Resolve(who); back == was && still == apart {
				kept = next
				at++
				plan.Added = append(plan.Added, line)
				plan.Kept = append(plan.Kept, h.Identity)
				continue
			}
		}
		plan.Lost = append(plan.Lost, h.Identity)
	}
	plan.Text = joinMailmapLines(kept)
	return plan
}
