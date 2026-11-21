package git

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// JoinPlan is the mailmap lines that make several identities read as one person, and the
// text with them in place. Nothing is written: the People view shows the lines, and the
// text becomes an edit there, saved like any other.
type JoinPlan struct {
	// Lines are the new lines, in the order they were added. None means the identities
	// already read as that person.
	Lines []string `json:"lines"`
	// At are their line numbers in Text, 1-based, in the same order.
	At []int `json:"at"`
	// Text is the mailmap text with the lines in place. Nothing else in it has moved.
	Text string `json:"text"`
	// Named are the identities whose line had to give the commit's name as well as its
	// address, because the address alone wouldn't do: someone who isn't being joined uses
	// it too, or another mailmap already has a line for it. Every other line leaves the
	// old name out: the address is enough, and a mailmap is plain text that people keep
	// after a name change.
	Named []Identity `json:"named"`
	// Blocked are identities no line here can join: a mailmap read after this one, from
	// mailmap.blob or mailmap.file, names that very name and address.
	Blocked []Identity `json:"blocked"`
}

// ErrJoinNeedsIdentity means the person to show wasn't given a usable name and address.
var ErrJoinNeedsIdentity = errors.New("a name and an address to show are needed, without < > or line breaks")

// MailmapReader returns a function that reads a .mailmap text together with the other
// sources git reads, as ReadMailmap does, without asking git about them again each time.
func (r *Repo) MailmapReader(ctx context.Context) (func(text string) *Mailmap, error) {
	sources, err := r.MailmapSources(ctx)
	if err != nil {
		return nil, err
	}
	type saved struct{ source, text string }
	var others []saved
	for _, s := range sources {
		if s.Source == MailmapWorkTree {
			continue
		}
		text, _, err := r.readMailmapSource(ctx, s)
		if err != nil {
			return nil, err
		}
		others = append(others, saved{s.Source, text})
	}
	return func(text string) *Mailmap {
		m := NewMailmap()
		m.Add(MailmapWorkTree, text)
		for _, o := range others {
			m.Add(o.source, o.text)
		}
		return m
	}, nil
}

// PlanJoin works out the lines that make members read as shown, and where they go in text.
// read reads a .mailmap text the way git would, with any other sources (see MailmapReader).
// history is every identity the history uses: a line must not change anyone who isn't a
// member.
//
// Each line is the shortest that does the job. An identity with another address gets
// "Shown Name <shown@email> <commit@email>", and one with the shown address and another
// name gets "Shown Name <commit@email>": neither spells the old name. Only when someone
// else shares the commit's address does the line name the commit's name too.
//
// A line goes beside the person's other lines; else, in a file kept in order, where its
// order puts it; else at the end. No existing line moves or changes.
func PlanJoin(text string, read func(text string) *Mailmap, history []HistoryIdentity, members []Identity, shown Identity) (JoinPlan, error) {
	if !joinable(shown.Name) || !joinable(shown.Email) {
		return JoinPlan{}, ErrJoinNeedsIdentity
	}
	text = normalizeMailmapText(text)
	var lines []string
	if text != "" {
		lines = strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	member := map[Identity]bool{}
	for _, m := range members {
		member[m] = true
	}
	// How everyone outside the join reads now: that must still hold afterwards.
	before := read(text)
	type reading struct{ who, as Identity }
	var others []reading
	for _, h := range history {
		if !member[h.Identity] {
			as, _ := before.Resolve(h.Identity)
			others = append(others, reading{h.Identity, as})
		}
	}
	keeps := func(m *Mailmap) bool {
		for _, o := range others {
			if as, _ := m.Resolve(o.who); as != o.as {
				return false
			}
		}
		return true
	}

	plan := JoinPlan{Lines: []string{}, At: []int{}, Named: []Identity{}, Blocked: []Identity{}}
	for _, r := range members {
		if as, _ := read(joinMailmapLines(lines)).Resolve(r); as == shown {
			continue
		}
		candidates := []string{shown.Name + " <" + r.Email + ">"}
		if r.Email != shown.Email {
			candidates[0] = shown.Name + " <" + shown.Email + "> <" + r.Email + ">"
		}
		if r.Name != "" {
			candidates = append(candidates, shown.Name+" <"+shown.Email+"> "+r.Name+" <"+r.Email+">")
		}
		placed := false
		for i, line := range candidates {
			if !joinable(r.Email) || (i == 1 && !joinable(r.Name)) || slices.Contains(lines, line) {
				continue
			}
			next, at := insertMailmapLine(lines, line, shown)
			m := read(joinMailmapLines(next))
			if as, _ := m.Resolve(r); as != shown || !keeps(m) {
				continue
			}
			lines = next
			for k := range plan.At {
				if plan.At[k] >= at+1 {
					plan.At[k]++
				}
			}
			plan.Lines = append(plan.Lines, line)
			plan.At = append(plan.At, at+1)
			if i == 1 {
				plan.Named = append(plan.Named, r)
			}
			placed = true
			break
		}
		if !placed {
			plan.Blocked = append(plan.Blocked, r)
		}
	}
	plan.Text = joinMailmapLines(lines)
	return plan, nil
}

// joinable reports whether s can be written into a mailmap line as a name or an address.
func joinable(s string) bool {
	return strings.TrimSpace(s) != "" && !strings.ContainsAny(s, "<>\r\n")
}

func joinMailmapLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// insertMailmapLine puts line into lines and says where (0-based): after the last line
// that already maps someone to shown; else, when the file keeps its mapping lines in
// order, where that order puts it; else at the end.
func insertMailmapLine(lines []string, line string, shown Identity) ([]string, int) {
	parsed := ParseMailmap(joinMailmapLines(lines))
	at := -1
	var mappings []int
	for i, l := range parsed {
		if l.Kind != MailmapMapping {
			continue
		}
		mappings = append(mappings, i)
		sameEmail := l.hasNewEmail && foldASCII(l.NewEmail) == foldASCII(shown.Email)
		nameOnly := !l.hasNewEmail && l.NewName == shown.Name && foldASCII(l.OldEmail) == foldASCII(shown.Email)
		if sameEmail || nameOnly {
			at = i + 1
		}
	}
	if at < 0 && mailmapInOrder(lines, mappings) {
		at = mappings[len(mappings)-1] + 1
		for _, i := range mappings {
			if strings.ToLower(lines[i]) > strings.ToLower(line) {
				at = i
				break
			}
		}
	}
	if at < 0 {
		at = len(lines)
	}
	return slices.Insert(slices.Clone(lines), at, line), at
}

// mailmapInOrder reports whether a file keeps its mapping lines sorted, as the Linux
// kernel's asks to be. A long file that's sorted by hand has a few lines out of place, so
// one in twenty may be.
func mailmapInOrder(lines []string, mappings []int) bool {
	if len(mappings) < 3 {
		return false
	}
	out := 0
	for k := 1; k < len(mappings); k++ {
		if strings.ToLower(lines[mappings[k-1]]) > strings.ToLower(lines[mappings[k]]) {
			out++
		}
	}
	return out*20 <= len(mappings)-1
}

// apartKey holds, in the repository's own git config, the suggestions someone answered
// "different people" to, so they aren't offered again. Each value names one: "name:" and a
// name, or "address:" and an address, in lower case.
const apartKey = "gitrevenge.apart"

// KeptApart returns the suggestions marked as different people in this repository.
func (r *Repo) KeptApart(ctx context.Context) []string {
	out, _ := r.ReadString(ctx, "config", "--local", "--get-all", apartKey) // exit 1: none
	var keys []string
	for line := range strings.Lines(out) {
		if k := strings.TrimSpace(line); k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

// KeepApart remembers that a suggestion is of different people, or with apart false
// forgets it, so it's offered again.
func (r *Repo) KeepApart(ctx context.Context, key string, apart bool) error {
	if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "\r\n") {
		return errors.New("no suggestion was named")
	}
	has := slices.Contains(r.KeptApart(ctx), key)
	switch {
	case apart && !has:
		_, err := r.Write(ctx, Cmd{Args: []string{"config", "--local", "--add", apartKey, key}})
		return err
	case !apart && has:
		_, err := r.Write(ctx, Cmd{Args: []string{"config", "--local", "--fixed-value", "--unset-all", apartKey, key}})
		return err
	}
	return nil
}
