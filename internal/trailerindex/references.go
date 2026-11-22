package trailerindex

import (
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// References returns the tickets one commit names, in the order written and each once: the
// ones `subject` finds in its subject line, then the ones its trailers name, read by the
// same rules a Reading reads them by. A ticket the Smart filters panel counts is then the
// same ticket wherever work is grouped by ticket, as in Insights' Shipped.
//
// It reads one message and needs no index, so whoever calls it reads a range of commits for
// itself and doesn't wait for a history's trailers to be read.
//
// A trailer names tickets when its whole value is tickets: "#12", "tide/api#12", "TIDE-12",
// the address of an issue, several with commas between, or what a smart link matches whole.
// Which key it's under doesn't decide that: "Bug: #12" names a ticket and "Fixes: 656c296
// (\"… #2\")" doesn't. A repository's rules do have their say: a key it holds back names
// none, a key it says holds something other than tickets names none, and a key it says
// holds tickets names one for each of its values, whatever they look like. A nil subject
// finds nothing in the subject.
func References(message string, rules Rules, subject func(text string) []string) []string {
	refs := []string{}
	add := func(ref string) {
		if ref = strings.TrimSpace(ref); ref != "" && !slices.Contains(refs, ref) {
			refs = append(refs, ref)
		}
	}
	message = strings.TrimSpace(message)
	if subject != nil {
		head, _, _ := strings.Cut(message, "\n")
		for _, ref := range subject(head) {
			add(ref)
		}
	}
	for _, t := range git.MessageTrailers(message) {
		id := joinedID(KeyID(t.Key), rules.Join)
		if rules.Show[id] == ShowHeld {
			continue
		}
		said := rules.Kinds[id]
		if said != "" && said != KindTicket {
			continue
		}
		tickets := ticketsIn(t.Value, rules)
		if tickets == nil && said == KindTicket {
			// The repository says these are tickets: each is one, as written.
			for part := range strings.SplitSeq(t.Value, ",") {
				add(part)
			}
			continue
		}
		for _, ticket := range tickets {
			add(ticket.Text)
		}
	}
	return refs
}
