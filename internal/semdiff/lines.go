package semdiff

import "strings"

// text is a file split into lines, each keeping its line ending, as the views split it: the
// lines joined give the text back, and a last line with no ending is a line.
type text struct {
	s     string
	start []int // line i is s[start[i]:start[i+1]]
}

func split(s string) text {
	t := text{s: s, start: make([]int, 0, strings.Count(s, "\n")+2)}
	t.start = append(t.start, 0)
	for i := 0; i < len(s); {
		j := strings.IndexByte(s[i:], '\n')
		if j < 0 {
			i = len(s)
		} else {
			i += j + 1
		}
		t.start = append(t.start, i)
	}
	return t
}

// n is the number of lines.
func (t text) n() int { return len(t.start) - 1 }

// line returns line i with its ending.
func (t text) line(i int) string { return t.s[t.start[i]:t.start[i+1]] }

// body returns line i without its ending.
func (t text) body(i int) string {
	l := t.line(i)
	l = strings.TrimSuffix(l, "\n")
	return strings.TrimSuffix(l, "\r")
}

// span returns lines [from, to) as one string.
func (t text) span(from, to int) string { return t.s[t.start[from]:t.start[to]] }
