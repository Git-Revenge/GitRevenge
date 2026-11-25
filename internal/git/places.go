package git

import (
	"context"
	"strings"
)

// ConflictText is one conflicted place in a file written with conflict markers: what each
// side has there, and the original when the markers were written in diff3 style.
type ConflictText struct {
	Ours    string
	Base    string
	Theirs  string
	HasBase bool
}

// ConflictTexts splits text written with conflict markers into its conflicted places, in
// order. Markers git didn't write, such as a lone ======= line, are left as text.
func ConflictTexts(text string) []ConflictText {
	const (
		outside = iota
		inOurs
		inBase
		inTheirs
	)
	var out []ConflictText
	state := outside
	var cur ConflictText
	var b strings.Builder
	for line := range strings.SplitAfterSeq(text, "\n") {
		bare := strings.TrimRight(line, "\r\n")
		switch {
		case state == outside && isMarker(bare, '<'):
			state, cur = inOurs, ConflictText{}
			b.Reset()
		case state == inOurs && isMarker(bare, '|'):
			cur.Ours, cur.HasBase, state = b.String(), true, inBase
			b.Reset()
		case (state == inOurs || state == inBase) && bare == "=======":
			if state == inOurs {
				cur.Ours = b.String()
			} else {
				cur.Base = b.String()
			}
			state = inTheirs
			b.Reset()
		case state == inTheirs && isMarker(bare, '>'):
			cur.Theirs, state = b.String(), outside
			out = append(out, cur)
		case state != outside:
			b.WriteString(line)
		}
	}
	return out
}

// SameMerge reports whether two texts written with conflict markers are the same merge, their
// places perhaps split differently. git's default conflict style joins places a line or two
// apart, and diff3's never does (#974), so a file on disk and its diff3 re-merge can have
// different places. Taking every place's ours, and then every place's theirs, gives the same
// text from both unless something in one was resolved or edited. The conflict tool's sourceOf
// does the same.
func SameMerge(a, b string) bool {
	aOurs, aTheirs, aOK := taking(a)
	bOurs, bTheirs, bOK := taking(b)
	return aOK && bOK && aOurs == bOurs && aTheirs == bTheirs
}

// taking is text with every conflicted place replaced by its ours, and by its theirs. ok is
// false when a place has no end, which ConflictTexts would drop.
func taking(text string) (ours, theirs string, ok bool) {
	const (
		outside = iota
		inOurs
		inBase
		inTheirs
	)
	state := outside
	var o, t strings.Builder
	for line := range strings.SplitAfterSeq(text, "\n") {
		bare := strings.TrimRight(line, "\r\n")
		switch {
		case state == outside && isMarker(bare, '<'):
			state = inOurs
		case state == inOurs && isMarker(bare, '|'):
			state = inBase
		case (state == inOurs || state == inBase) && bare == "=======":
			state = inTheirs
		case state == inTheirs && isMarker(bare, '>'):
			state = outside
		case state == outside:
			o.WriteString(line)
			t.WriteString(line)
		case state == inOurs:
			o.WriteString(line)
		case state == inTheirs:
			t.WriteString(line)
		}
	}
	return o.String(), t.String(), state == outside
}

// isMarker reports whether line is a conflict marker made of c: seven of them, then the end
// of the line or a space and a label.
func isMarker(line string, c byte) bool {
	if len(line) < 7 || (len(line) > 7 && line[7] != ' ') {
		return false
	}
	for i := range 7 {
		if line[i] != c {
			return false
		}
	}
	return true
}

// ChangedOnBothSides returns which of paths both sides of the operation in progress
// changed: the files git had to merge, and so the only ones a merge driver ran on.
func (r *Repo) ChangedOnBothSides(ctx context.Context, gitDir string, paths []string) map[string]bool {
	out := map[string]bool{}
	ours, base, theirs := r.conflictRevs(ctx, gitDir)
	if base == "" || theirs == "" || len(paths) == 0 {
		return out
	}
	want := make(map[string]bool, len(paths))
	for _, p := range paths {
		want[p] = true
	}
	// Every changed name is listed and filtered here: there may be too many paths to pass.
	changed := func(to string) map[string]bool {
		set := map[string]bool{}
		b, err := r.Read(ctx, "diff", "--name-only", "-z", "--no-renames", base, to)
		if err != nil {
			return set
		}
		for p := range strings.SplitSeq(string(b), "\x00") {
			if want[p] {
				set[p] = true
			}
		}
		return set
	}
	a, b := changed(ours), changed(theirs)
	for p := range a {
		if b[p] {
			out[p] = true
		}
	}
	return out
}
