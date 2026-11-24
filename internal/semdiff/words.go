package semdiff

import (
	"unicode"
	"unicode/utf8"

	"znkr.io/diff"
)

// Limits on comparing a block word by word: past them the block is marked by whole lines,
// as it was before.
const (
	maxWordLines  = 500
	maxWordTokens = 20_000
)

// minAlike is how much of a block's text has to survive, as a share of its letters, digits
// and punctuation on both sides, for its words to be marked. Below it the lines were
// rewritten, and marking every word of them says nothing the whole-line colour doesn't.
const minAlike = 0.4

// token is a word, a run of spaces, or one other character, within one line. from and to
// are byte offsets into the line.
type token struct {
	line, from, to int
	s              string
	space          bool
}

// markWords sets blk.Old and blk.New to the words that changed between its old and new
// lines, leaving out moved lines.
func markWords(a, b text, blk *Block, moved movedLines) {
	if blk.OldStart == blk.OldEnd || blk.NewStart == blk.NewEnd {
		return
	}
	if blk.OldEnd-blk.OldStart > maxWordLines || blk.NewEnd-blk.NewStart > maxWordLines {
		return
	}
	ta := tokens(a, blk.OldStart, blk.OldEnd, moved.before)
	tb := tokens(b, blk.NewStart, blk.NewEnd, moved.after)
	if len(ta) == 0 || len(tb) == 0 || len(ta)+len(tb) > maxWordTokens {
		return
	}
	sa := make([]string, len(ta))
	for i, t := range ta {
		sa[i] = t.s
	}
	sb := make([]string, len(tb))
	for i, t := range tb {
		sb[i] = t.s
	}
	edits := diff.Edits(sa, sb)
	changedA := make([]bool, len(ta))
	changedB := make([]bool, len(tb))
	kept, total := 0, 0
	for _, e := range edits {
		switch e.Op {
		case diff.Match:
			if !ta[e.PosX].space {
				kept += 2 * len(e.X)
			}
		case diff.Delete:
			changedA[e.PosX] = true
		case diff.Insert:
			changedB[e.PosY] = true
		}
	}
	for _, t := range ta {
		if !t.space {
			total += len(t.s)
		}
	}
	for _, t := range tb {
		if !t.space {
			total += len(t.s)
		}
	}
	if total == 0 || float64(kept)/float64(total) < minAlike {
		return
	}
	blk.Old = spans(a, ta, changedA)
	blk.New = spans(b, tb, changedB)
}

// tokens splits lines [from, to) of t into tokens, leaving out the lines in skip and line
// endings.
func tokens(t text, from, to int, skip map[int]bool) []token {
	var out []token
	for i := from; i < to; i++ {
		if skip[i] {
			continue
		}
		body := t.body(i)
		for p := 0; p < len(body); {
			r, size := utf8.DecodeRuneInString(body[p:])
			end := p + size
			switch {
			case isWord(r):
				for end < len(body) {
					r2, s2 := utf8.DecodeRuneInString(body[end:])
					if !isWord(r2) {
						break
					}
					end += s2
				}
			case r == ' ' || r == '\t':
				for end < len(body) && (body[end] == ' ' || body[end] == '\t') {
					end++
				}
			}
			out = append(out, token{line: i, from: p, to: end, s: body[p:end], space: r == ' ' || r == '\t'})
			p = end
		}
	}
	return out
}

func isWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// spans joins the changed tokens into one span per run in a line. A space between two changed
// tokens joins them, so "a b" becoming "c d" is one change rather than two.
func spans(t text, toks []token, changed []bool) []Span {
	var out []Span
	for i := 0; i < len(toks); {
		if !changed[i] {
			i++
			continue
		}
		j := i + 1
		for j < len(toks) && toks[j].line == toks[i].line {
			if changed[j] {
				j++
				continue
			}
			if toks[j].space && j+1 < len(toks) && changed[j+1] && toks[j+1].line == toks[i].line {
				j += 2
				continue
			}
			break
		}
		line := t.body(toks[i].line)
		out = append(out, Span{Line: toks[i].line, From: utf16Len(line[:toks[i].from]), To: utf16Len(line[:toks[j-1].to])})
		i = j
	}
	return out
}

// utf16Len is the length of s in UTF-16 code units.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
