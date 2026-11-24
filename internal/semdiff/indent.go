// Copyright 2025 Florian Zenker (flo@znkr.io)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Adapted for Git Revenge from znkr.io/diff v1.1.4, internal/indentheuristic, which can't be
// imported from outside its module. Changes: it works on this package's text and on plain
// []bool vectors; the start-of-file penalty applies when no line with text comes before the
// split, and the penalty for a line indented more than the one before is added rather than
// assigned, both as git does.

package semdiff

import "cmp"

// slideChanges moves each run of changed lines, where an equal line lets it slide, to where
// git's diff would put it: merged with neighbouring runs, level with a change on the other
// side when it can be, and otherwise where the indentation says a reader expects a change to
// start and end. This is Michael Haggerty's indent heuristic
// (https://github.com/mhagger/diff-slider-tools), which git applies by default.
//
// changedA and changedB mark the changed lines of a and b, with one extra false entry at the
// end of each.
func slideChanges(a, b text, changedA, changedB []bool) {
	slideSide(a, changedA, changedB) // deletions
	slideSide(b, changedB, changedA) // insertions
}

// Never move a run more than this many lines.
const maxSliding = 100

// Indentation past this is clamped, so a line of only spaces can't overflow an int.
const maxIndent = 200

// At most this many blank lines in a row are counted.
const maxBlanks = 20

// The weights, from diff-slider-tools.
const (
	startOfFilePenalty              = 1   // no line with text before the split
	endOfFilePenalty                = 21  // no line with text after the split
	totalBlankWeight                = -30 // for each blank line around the split
	postBlankWeight                 = 6   // for each blank line after the split
	relativeIndentPenalty           = -4  // indented more than the line before
	relativeIndentWithBlankPenalty  = 10  // the same, with blank lines between
	relativeOutdentPenalty          = 24  // indented less than the line before
	relativeOutdentWithBlankPenalty = 17
	relativeDedentPenalty           = 23 // less than the line before, but not the line after
	relativeDedentWithBlankPenalty  = 17
	indentWeight                    = 60
)

func slideSide(t text, r, ro []bool) {
	s, so := newScanner(t, r), newScanner(text{}, ro)
	for s.nextGroup() {
		if !so.nextGroup() {
			panic("semdiff: the sides' runs are out of step")
		}
		if s.groupLen() == 0 {
			if so.groupLen() == 0 {
				// Lines unchanged on both sides: skip them together.
				n := min(nextSet(s.r, s.end+1)-s.end, nextSet(so.r, so.end+1)-so.end) - 1
				s.start, s.end = s.start+n, s.end+n
				so.start, so.end = so.start+n, so.end+n
			}
			continue
		}

		matchingEnd := -1 // where the run lines up with a change on the other side
		minEnd := s.end   // the highest it can go
		length := 0
		for length != s.groupLen() {
			length = s.groupLen()
			matchingEnd = -1
			// Slide up as far as it goes, joining any run it meets.
			for s.slideUp() {
				if !so.prevGroup() {
					panic("semdiff: the sides' runs are out of step")
				}
			}
			minEnd = s.end
			if so.groupLen() > 0 {
				matchingEnd = s.end
			}
			// Then down as far as it goes.
			for s.slideDown() {
				if !so.nextGroup() {
					panic("semdiff: the sides' runs are out of step")
				}
				if so.groupLen() > 0 {
					matchingEnd = s.end
				}
			}
		}

		switch {
		case minEnd == s.end:
			// It can't move.
		case matchingEnd != -1:
			// Line it up with the change on the other side.
			for so.groupLen() == 0 {
				if !s.slideUp() || !so.prevGroup() {
					panic("semdiff: lost the matching run")
				}
			}
		default:
			// It's at its lowest; try each place above for the best split.
			best := -1
			var bestScore shiftScore
			for shift := max(minEnd, s.end-length-1, s.end-maxSliding); shift <= s.end; shift++ {
				var score shiftScore
				score.add(measureSplit(t, shift))
				score.add(measureSplit(t, shift-length))
				if best == -1 || score.cmp(bestScore) <= 0 {
					best, bestScore = shift, score
				}
			}
			for s.end > best {
				if !s.slideUp() || !so.prevGroup() {
					panic("semdiff: lost the best place")
				}
			}
		}
	}
	if so.nextGroup() {
		panic("semdiff: the sides' runs are out of step")
	}
}

// nextSet is the first changed line at or after i, or len(r).
func nextSet(r []bool, i int) int {
	for ; i < len(r); i++ {
		if r[i] {
			return i
		}
	}
	return len(r)
}

// nextClear is the first unchanged line at or after i, or len(r).
func nextClear(r []bool, i int) int {
	for ; i < len(r); i++ {
		if !r[i] {
			return i
		}
	}
	return len(r)
}

// prevClear is the last unchanged line at or before i, or -1.
func prevClear(r []bool, i int) int {
	for ; i >= 0; i-- {
		if !r[i] {
			return i
		}
	}
	return -1
}

// scanner walks the runs of changed lines on one side. A run is [start, end); between two
// runs of changed lines there's an empty run at each unchanged line.
type scanner struct {
	start, end int
	t          text
	r          []bool
}

func newScanner(t text, r []bool) *scanner { return &scanner{start: -1, end: -1, t: t, r: r} }

func (s *scanner) groupLen() int { return s.end - s.start }

func (s *scanner) nextGroup() bool {
	if s.end == len(s.r)-1 {
		return false
	}
	s.start, s.end = s.end+1, s.end+1
	s.end = nextClear(s.r, s.end)
	return true
}

func (s *scanner) prevGroup() bool {
	if s.start == 0 {
		return false
	}
	s.end = s.start - 1
	s.start = prevClear(s.r, s.end-1) + 1
	return true
}

// slideDown moves the run down a line if the line after it equals its first, joining a run it
// then touches.
func (s *scanner) slideDown() bool {
	if s.end < len(s.r)-1 && s.t.line(s.start) == s.t.line(s.end) {
		s.r[s.start], s.r[s.end] = false, true
		s.start++
		s.end = nextClear(s.r, s.end+1)
		return true
	}
	return false
}

// slideUp moves the run up a line if the line before it equals its last, joining a run it
// then touches.
func (s *scanner) slideUp() bool {
	if s.start > 0 && s.t.line(s.start-1) == s.t.line(s.end-1) {
		s.r[s.start-1], s.r[s.end-1] = true, false
		s.end--
		s.start = prevClear(s.r, s.start-2) + 1
		return true
	}
	return false
}

type measure struct {
	endOfFile  bool
	indent     int
	preBlank   int
	preIndent  int
	postBlank  int
	postIndent int
}

// measureSplit describes the lines around a split before line i.
func measureSplit(t text, i int) measure {
	m := measure{indent: -1, preIndent: -1, postIndent: -1}
	if i >= t.n() {
		m.endOfFile = true
	} else {
		m.indent = indentOf(t.line(i))
	}
	for k := i - 1; k >= 0; k-- {
		if m.preIndent = indentOf(t.line(k)); m.preIndent != -1 {
			break
		}
		if m.preBlank++; m.preBlank == maxBlanks {
			m.preIndent = 0
			break
		}
	}
	for k := i + 1; k < t.n(); k++ {
		if m.postIndent = indentOf(t.line(k)); m.postIndent != -1 {
			break
		}
		if m.postBlank++; m.postBlank == maxBlanks {
			m.postIndent = 0
			break
		}
	}
	return m
}

// indentOf is a line's indentation in columns, tabs to the next multiple of 8, or -1 for a
// line of only white space.
func indentOf(line string) int {
	n := 0
	for i := range len(line) {
		switch line[i] {
		case ' ':
			n++
		case '\t':
			n += 8 - n%8
		case '\n', '\v', '\r', '\f':
		default:
			return n
		}
		if n >= maxIndent {
			return maxIndent
		}
	}
	return -1
}

type shiftScore struct {
	effectiveIndent int // smaller is better
	penalty         int // smaller is better
}

func (s *shiftScore) add(m measure) {
	if m.preIndent == -1 && m.preBlank == 0 {
		s.penalty += startOfFilePenalty
	}
	if m.endOfFile {
		s.penalty += endOfFilePenalty
	}
	postBlank := 0
	if m.indent == -1 {
		postBlank = 1 + m.postBlank
	}
	totalBlank := m.preBlank + postBlank
	s.penalty += totalBlankWeight * totalBlank
	s.penalty += postBlankWeight * postBlank

	indent := m.indent
	if indent == -1 {
		indent = m.postIndent
	}
	s.effectiveIndent += indent

	switch {
	case indent == -1 || m.preIndent == -1:
	case indent > m.preIndent:
		if totalBlank != 0 {
			s.penalty += relativeIndentWithBlankPenalty
		} else {
			s.penalty += relativeIndentPenalty
		}
	case indent == m.preIndent:
	default:
		// Indented less than the line before: the end of a block, or the start of another
		// (an else) if the next line is indented more.
		if m.postIndent != -1 && m.postIndent > indent {
			if totalBlank != 0 {
				s.penalty += relativeOutdentWithBlankPenalty
			} else {
				s.penalty += relativeOutdentPenalty
			}
		} else {
			if totalBlank != 0 {
				s.penalty += relativeDedentWithBlankPenalty
			} else {
				s.penalty += relativeDedentPenalty
			}
		}
	}
}

func (s *shiftScore) cmp(t shiftScore) int {
	return indentWeight*cmp.Compare(s.effectiveIndent, t.effectiveIndent) + s.penalty - t.penalty
}

// compact slides blocks as slideChanges does, and returns them again as blocks.
func compact(a, b text, blocks []Block) []Block {
	if len(blocks) == 0 {
		return blocks
	}
	ra := make([]bool, a.n()+1)
	rb := make([]bool, b.n()+1)
	for _, blk := range blocks {
		for i := blk.OldStart; i < blk.OldEnd; i++ {
			ra[i] = true
		}
		for j := blk.NewStart; j < blk.NewEnd; j++ {
			rb[j] = true
		}
	}
	slideChanges(a, b, ra, rb)
	// Unchanged lines pair up in order, so walking both sides together finds the blocks.
	var out []Block
	i, j := 0, 0
	for i < a.n() || j < b.n() {
		if i < a.n() && j < b.n() && !ra[i] && !rb[j] {
			i++
			j++
			continue
		}
		blk := Block{OldStart: i, NewStart: j}
		for i < a.n() && ra[i] {
			i++
		}
		for j < b.n() && rb[j] {
			j++
		}
		if i == blk.OldStart && j == blk.NewStart {
			panic("semdiff: the sides' unchanged lines don't pair up")
		}
		blk.OldEnd, blk.NewEnd = i, j
		out = append(out, blk)
	}
	return out
}
