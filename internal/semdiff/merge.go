package semdiff

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"znkr.io/diff"
)

// Merging a conflicted place token by token (ADR 0015, #946). git's line merge decides what
// conflicts; inside each conflicted place, the original and the two sides are split into
// tokens and merged as diff3 merges lines. Where the sides changed different tokens, the
// result is a suggestion for the conflict tool to pre-fill, never written on its own.

// Suggestion is a conflicted place's three versions merged by token.
type Suggestion struct {
	// OK is set when the sides changed different tokens, so Text combines them.
	OK   bool   `json:"ok"`
	Text string `json:"text"`
	// Reason says what each side did ("ours added ctx, theirs added step"), or why they
	// can't be combined ("both sides changed maxHours").
	Reason string `json:"reason"`
	// Origins are where each side's changes are in Text.
	Origins []Origin `json:"origins"`
	// Ours and Theirs are what each side changed against the original, as stretches of
	// that side's own text, combined or not.
	Ours   []Stretch `json:"ours"`
	Theirs []Stretch `json:"theirs"`
}

// Stretch is [From, To) of a text, in UTF-16 units.
type Stretch struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Origin is a stretch of a suggestion's text from one side: [From, To) in UTF-16 units.
type Origin struct {
	From int    `json:"from"`
	To   int    `json:"to"`
	Side string `json:"side"` // "ours", "theirs", or "both" when they made the same change
}

// MergePlace merges one conflicted place, given its original and both sides, in the
// language with id. It reads each version's tokens; whitespace between them is a token too,
// so the result is exact text.
func MergePlace(ctx context.Context, id, base, ours, theirs string) Suggestion {
	b, o, t := pieces(ctx, id, base), pieces(ctx, id, ours), pieces(ctx, id, theirs)
	s := merge3(b, o, t)
	// Imports added by both sides in one spot clash as tokens, but their order means
	// nothing: they merge as a set.
	if !s.OK {
		if set, ok := mergeSet(id, base, ours, theirs); ok {
			s = set
		}
	}
	s.Ours, s.Theirs = changed(o, matches(b, o)), changed(t, matches(b, t))
	return s
}

// changed is where side differs from the original: its pieces nothing in the original
// matches, joined across the whitespace between them. m is matches(base, side).
func changed(side []string, m []int) []Stretch {
	kept := make([]bool, len(side))
	for _, j := range m {
		if j >= 0 {
			kept[j] = true
		}
	}
	out := []Stretch{}
	at := 0
	joinable := false // nothing but spaces since the last stretch ended
	for i, p := range side {
		n := utf16Len(p)
		blank := strings.TrimSpace(p) == ""
		switch {
		case !kept[i] && !blank:
			if k := len(out) - 1; k >= 0 && joinable {
				out[k].To = at + n
			} else {
				out = append(out, Stretch{From: at, To: at + n})
			}
			joinable = true
		case blank && !strings.Contains(p, "\n"):
			// Spaces on a line between two changed pieces join them into one stretch.
		default:
			joinable = false
		}
		at += n
	}
	return out
}

// pieces splits s into tokens and the whitespace between them, which joined give s back.
// The language's own tokens are used when s reads; a fragment that doesn't, such as half a
// function, still splits into words, runs of whitespace and single other characters.
func pieces(ctx context.Context, id, s string) []string {
	if l := byID(id); l != nil && s != "" {
		l.load()
		r := readVersion(ctx, l, s, QuickRead, false)
		if r.stop == "" {
			var out []string
			at := 0
			for _, tk := range r.toks {
				if tk.from < at {
					continue
				}
				if tk.from > at {
					out = append(out, s[at:tk.from])
				}
				out = append(out, s[tk.from:tk.to])
				at = tk.to
			}
			if at < len(s) {
				out = append(out, s[at:])
			}
			return out
		}
	}
	var out []string
	for _, w := range words(s) {
		out = append(out, s[w[0]:w[1]])
	}
	return out
}

// merge3 merges ours and theirs against base, as diff3 does: between the tokens all three
// keep, a stretch only one side changed takes that side's version, one both changed the same
// way takes it, and one they changed differently can't be merged.
func merge3(base, ours, theirs []string) Suggestion {
	mo, mt := matches(base, ours), matches(base, theirs)
	var b strings.Builder
	var origins []Origin
	var oursDid, theirsDid []string
	at := 0 // UTF-16 length of what's written
	write := func(parts []string, side string) {
		text := strings.Join(parts, "")
		if text == "" {
			return
		}
		n := utf16Len(text)
		if side != "" {
			if k := len(origins) - 1; k >= 0 && origins[k].Side == side && origins[k].To == at {
				origins[k].To += n
			} else {
				origins = append(origins, Origin{From: at, To: at + n, Side: side})
			}
		}
		b.WriteString(text)
		at += n
	}
	i, j, k := 0, 0, 0
	for {
		// The next token all three keep, and where it is on each side.
		s := i
		for s < len(base) && (mo[s] < 0 || mt[s] < 0) {
			s++
		}
		oj, tk := len(ours), len(theirs)
		if s < len(base) {
			oj, tk = mo[s], mt[s]
		}
		bc, oc, tc := base[i:s], ours[j:oj], theirs[k:tk]
		switch {
		case equal(oc, bc):
			write(tc, side(tc, bc, "theirs"))
			if !equal(tc, bc) {
				theirsDid = append(theirsDid, describe(bc, tc))
			}
		case equal(tc, bc):
			write(oc, "ours")
			oursDid = append(oursDid, describe(bc, oc))
		case equal(oc, tc):
			write(oc, "both")
		default:
			return Suggestion{Reason: conflictReason(bc), Origins: []Origin{}}
		}
		if s == len(base) {
			break
		}
		write(base[s:s+1], "")
		i, j, k = s+1, oj+1, tk+1
	}
	return Suggestion{OK: true, Text: b.String(), Reason: mergeReason(oursDid, theirsDid), Origins: origins}
}

// side is who a stretch came from: name, unless it's the original.
func side(got, base []string, name string) string {
	if equal(got, base) {
		return ""
	}
	return name
}

// matches pairs each base token with the side's token it matches, or -1.
func matches(base, side []string) []int {
	m := make([]int, len(base))
	for i := range m {
		m[i] = -1
	}
	for _, e := range diff.Edits(base, side) {
		if e.Op == diff.Match {
			m[e.PosX] = e.PosY
		}
	}
	return m
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// describe says what one side did to a stretch of the original: added, removed or changed
// what.
func describe(base, got []string) string {
	was, now := quoted(base), quoted(got)
	switch {
	case was == "" && now == "":
		return "changed the spacing"
	case was == "":
		return "added " + now
	case now == "":
		return "removed " + was
	}
	return "changed " + was + " to " + now
}

// quoted is a stretch's text for a sentence: trimmed, in backticks, and shortened when long.
func quoted(parts []string) string {
	s := strings.Join(strings.Fields(strings.Join(parts, "")), " ")
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > 32 {
		r := []rune(s)
		s = string(r[:30]) + "…"
	}
	return "`" + s + "`"
}

// mergeReason says what each side did, for a suggestion's heading.
func mergeReason(ours, theirs []string) string {
	say := func(who string, did []string) string {
		if len(did) == 0 {
			return ""
		}
		s := who + " " + did[0]
		if len(did) > 1 {
			s += fmt.Sprintf(" and made %d more changes", len(did)-1)
		}
		return s
	}
	o, t := say("ours", ours), say("theirs", theirs)
	switch {
	case o != "" && t != "":
		return o + "; " + t
	case o != "":
		return o
	}
	return t
}

// conflictReason says why a place can't be combined.
func conflictReason(base []string) string {
	if w := quoted(base); w != "" {
		return "both sides changed " + w
	}
	return "both sides added something in the same spot"
}
