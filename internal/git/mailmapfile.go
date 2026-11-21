package git

import "strings"

// This file reads mailmap text the way git does (mailmap.c), without asking git. The
// People view needs that to show what an edit would do before it's saved, and to say
// which line is behind a name. The rules, from git:
//
//   - A line whose first character is # is a comment.
//   - A mapping has one or two "Name <email>" parts, either name optional:
//     "Proper Name <commit@email>", "<proper@email> <commit@email>",
//     "Proper Name <proper@email> <commit@email>" and
//     "Proper Name <proper@email> Commit Name <commit@email>".
//   - The commit's email is what's looked up, and with the last form its name too. Both
//     are matched without regard to case.
//   - A later line for the same commit email replaces an earlier one, field by field:
//     a line that gives only a name leaves the email an earlier line gave.
//   - Nothing is followed twice: the result of one line isn't looked up again.
//
// The tests check its answers against `git check-mailmap`.

// MailmapRef names a line of a mailmap source.
type MailmapRef struct {
	// Source is ".mailmap", "mailmap.blob" or "mailmap.file".
	Source string `json:"source"`
	// Line is 1-based.
	Line int `json:"line"`
}

// Kinds of mailmap line.
const (
	MailmapComment = "comment"
	MailmapBlank   = "blank"
	MailmapMapping = "map"
	// MailmapSkipped is a line git reads past: it has text, and nothing git can use.
	MailmapSkipped = "skipped"
)

// MailmapLine is one line of mailmap text as git reads it.
type MailmapLine struct {
	Line int    `json:"line"`
	Kind string `json:"kind"`
	// For a mapping: what a commit must have (OldEmail, and OldName when the line gives
	// one), and what it's shown as (NewName and NewEmail, either of which may be absent,
	// leaving that part as the commit has it).
	NewName  string `json:"newName,omitempty"`
	NewEmail string `json:"newEmail,omitempty"`
	OldName  string `json:"oldName,omitempty"`
	OldEmail string `json:"oldEmail,omitempty"`
	// hasNewEmail tells an absent new email from an empty one; names can't be empty.
	hasNewEmail bool
	hasOldName  bool
	// Why says what's wrong with a skipped line.
	Why string `json:"why,omitempty"`
}

// ParseMailmap reads mailmap text into its lines.
func ParseMailmap(text string) []MailmapLine {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	rows := strings.Split(text, "\n")
	out := make([]MailmapLine, len(rows))
	for i, row := range rows {
		out[i] = parseMailmapLine(row)
		out[i].Line = i + 1
	}
	return out
}

func parseMailmapLine(row string) MailmapLine {
	if strings.HasPrefix(row, "#") {
		return MailmapLine{Kind: MailmapComment}
	}
	if strings.TrimFunc(row, isCSpace) == "" {
		return MailmapLine{Kind: MailmapBlank}
	}
	name1, email1, rest, ok := mailmapPart(row, false)
	if !ok {
		why := "no address in < >"
		switch {
		case strings.HasPrefix(strings.TrimLeftFunc(row, isCSpace), "#"):
			// Git only takes # in the first column as a comment. With no address the line
			// does nothing either way, so it's shown as the comment it was meant to be.
			return MailmapLine{Kind: MailmapComment}
		case strings.Contains(row, "<>"):
			why = "the address is empty"
		case strings.Contains(row, "<"):
			why = "the address has no closing >"
		}
		return MailmapLine{Kind: MailmapSkipped, Why: why}
	}
	l := MailmapLine{Kind: MailmapMapping}
	name2, email2, _, second := "", "", "", false
	if rest != "" {
		name2, email2, _, second = mailmapPart(rest, true)
	}
	if !second {
		// One part: its address is the commit's, and only the name is given.
		l.NewName, l.OldEmail = name1, email1
		return l
	}
	l.NewName, l.NewEmail, l.hasNewEmail = name1, email1, true
	l.OldName, l.hasOldName, l.OldEmail = name2, name2 != "", email2
	return l
}

// mailmapPart reads one "Name <email>" from the start of s, as git's parse_name_and_email
// does: the first < and the first > after it bound the email, and the name is what's
// before, trimmed. rest is what follows the >.
func mailmapPart(s string, allowEmptyEmail bool) (name, email, rest string, ok bool) {
	left := strings.IndexByte(s, '<')
	if left < 0 {
		return "", "", "", false
	}
	right := strings.IndexByte(s[left+1:], '>')
	if right < 0 {
		return "", "", "", false
	}
	right += left + 1
	if !allowEmptyEmail && left+1 == right {
		return "", "", "", false
	}
	return strings.TrimFunc(s[:left], isCSpace), s[left+1 : right], s[right+1:], true
}

// isCSpace is C's isspace, which is what git trims names with.
func isCSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r'
}

// foldASCII lowers ASCII letters only, as the strcasecmp git matches with does.
func foldASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; 'A' <= c && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if 'A' <= b[j] && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// mailmapTarget is what a commit identity is shown as, and the lines that said so.
type mailmapTarget struct {
	name, email       string
	hasName, hasEmail bool
	nameAt, emailAt   MailmapRef
}

// mailmapItem is everything said about one commit email.
type mailmapItem struct {
	simple mailmapTarget
	// byName holds the lines that also named the commit's name, by that name folded.
	byName map[string]*mailmapTarget
}

// Mailmap is one or more mailmap texts read together, as git reads the work tree's
// .mailmap, then mailmap.blob, then mailmap.file, each replacing what came before.
type Mailmap struct {
	byEmail map[string]*mailmapItem
}

// NewMailmap returns an empty Mailmap.
func NewMailmap() *Mailmap { return &Mailmap{byEmail: map[string]*mailmapItem{}} }

// Add reads text as the next source. Sources are added in the order git reads them.
func (m *Mailmap) Add(source, text string) {
	for _, l := range ParseMailmap(text) {
		if l.Kind != MailmapMapping {
			continue
		}
		at := MailmapRef{Source: source, Line: l.Line}
		key := foldASCII(l.OldEmail)
		item := m.byEmail[key]
		if item == nil {
			item = &mailmapItem{}
			m.byEmail[key] = item
		}
		if !l.hasOldName {
			if l.NewName != "" {
				item.simple.name, item.simple.hasName, item.simple.nameAt = l.NewName, true, at
			}
			if l.hasNewEmail {
				item.simple.email, item.simple.hasEmail, item.simple.emailAt = l.NewEmail, true, at
			}
			continue
		}
		// A line that names the commit's name stands alone: it replaces an earlier line
		// for the same name whole.
		t := &mailmapTarget{}
		if l.NewName != "" {
			t.name, t.hasName, t.nameAt = l.NewName, true, at
		}
		if l.hasNewEmail {
			t.email, t.hasEmail, t.emailAt = l.NewEmail, true, at
		}
		if item.byName == nil {
			item.byName = map[string]*mailmapTarget{}
		}
		item.byName[foldASCII(l.OldName)] = t
	}
}

// Resolve returns who a commit identity is shown as, and the lines that decide it: none
// when no line applies, and two when one line gave the name and another the email.
func (m *Mailmap) Resolve(id Identity) (Identity, []MailmapRef) {
	item := m.byEmail[foldASCII(id.Email)]
	if item == nil {
		return id, nil
	}
	t := &item.simple
	if len(item.byName) > 0 {
		if named := item.byName[foldASCII(id.Name)]; named != nil {
			t = named
		}
	}
	if !t.hasName && !t.hasEmail {
		return id, nil
	}
	out := id
	var by []MailmapRef
	if t.hasEmail {
		out.Email = t.email
		by = append(by, t.emailAt)
	}
	if t.hasName {
		out.Name = t.name
		if !t.hasEmail || t.nameAt != t.emailAt {
			by = append(by, t.nameAt)
		}
	}
	return out, by
}

// live returns the lines that still decide something: a line every part of which a later
// line replaced isn't among them.
func (m *Mailmap) live() map[MailmapRef]bool {
	out := map[MailmapRef]bool{}
	mark := func(t *mailmapTarget) {
		if t.hasName {
			out[t.nameAt] = true
		}
		if t.hasEmail {
			out[t.emailAt] = true
		}
	}
	for _, item := range m.byEmail {
		mark(&item.simple)
		for _, t := range item.byName {
			mark(t)
		}
	}
	return out
}

// MailmapNote says what one line of a mailmap source does to a history.
type MailmapNote struct {
	MailmapLine
	// Identities is how many of the history's identities the line changes, and Commits and
	// CoAuthored how many commits they wrote and are credited on.
	Identities int `json:"identities"`
	Commits    int `json:"commits"`
	CoAuthored int `json:"coAuthored"`
	// Replaced means a later line, in this source or one read after it, says something
	// else for everything this line said.
	Replaced bool `json:"replaced"`
}

// Notes says what each line of one source does, given the identities a history uses.
// source must be one that was added.
func (m *Mailmap) Notes(source, text string, history []HistoryIdentity) []MailmapNote {
	lines := ParseMailmap(text)
	notes := make([]MailmapNote, len(lines))
	for i, l := range lines {
		notes[i].MailmapLine = l
	}
	live := m.live()
	for i := range notes {
		if notes[i].Kind == MailmapMapping {
			notes[i].Replaced = !live[MailmapRef{Source: source, Line: notes[i].Line}]
		}
	}
	for _, h := range history {
		shown, by := m.Resolve(h.Identity)
		if shown == h.Identity {
			continue // a line that maps someone to themselves joins nothing
		}
		for _, at := range by {
			if at.Source != source || at.Line < 1 || at.Line > len(notes) {
				continue
			}
			n := &notes[at.Line-1]
			n.Identities++
			n.Commits += h.Authored
			n.CoAuthored += h.CoAuthored
		}
	}
	return notes
}
