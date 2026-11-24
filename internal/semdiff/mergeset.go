package semdiff

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Merging a list of imports as a set (#947), as Spork and mergiraf merge unordered members:
// the order of imports means nothing, so two sides adding different ones in the same spot
// don't conflict. Only a place whose every line, in all three versions, is an import (or
// blank) is merged this way, and only when the token merge couldn't combine it.

// importLines recognises one import per line, by language id. The value is the regexp for a
// line and, when set, the group that names what's imported, so the same module imported
// twice in two ways is still a clash.
var importLines = map[string]struct {
	line *regexp.Regexp
	key  int
}{
	"go":         {regexp.MustCompile(`^\s*(?:import\s+)?(?:[\w.]+\s+)?("[^"]+")\s*(?://.*)?$`), 1},
	"javascript": {regexp.MustCompile(`^\s*(?:import|export)\s(?:.+\sfrom\s+)?(['"][^'"]+['"])\s*(?:with\s*\{[^}]*\}\s*)?;?\s*$`), 0},
	"typescript": {regexp.MustCompile(`^\s*(?:import|export)\s(?:.+\sfrom\s+)?(['"][^'"]+['"])\s*(?:with\s*\{[^}]*\}\s*)?;?\s*$`), 0},
	"tsx":        {regexp.MustCompile(`^\s*(?:import|export)\s(?:.+\sfrom\s+)?(['"][^'"]+['"])\s*(?:with\s*\{[^}]*\}\s*)?;?\s*$`), 0},
	"python":     {regexp.MustCompile(`^\s*(?:import\s+[\w.]+(?:\s+as\s+\w+)?(?:\s*,\s*[\w.]+(?:\s+as\s+\w+)?)*|from\s+[\w.]+\s+import\s+[\w*][\w\s,*]*)\s*(?:#.*)?$`), 0},
	"java":       {regexp.MustCompile(`^\s*import\s+(?:static\s+)?([\w.]+(?:\.\*)?)\s*;\s*$`), 1},
	"kotlin":     {regexp.MustCompile(`^\s*import\s+([\w.]+(?:\.\*)?)(?:\s+as\s+\w+)?\s*$`), 0},
	"scala":      {regexp.MustCompile(`^\s*import\s+[\w.]+(?:\.(?:\*|_|\{[^}]*\}))?\s*$`), 0},
	"groovy":     {regexp.MustCompile(`^\s*import\s+(?:static\s+)?([\w.]+(?:\.\*)?)\s*;?\s*$`), 1},
	"csharp":     {regexp.MustCompile(`^\s*(?:global\s+)?using\s+(?:static\s+)?(?:\w+\s*=\s*)?([\w.]+)\s*;\s*$`), 0},
	"rust":       {regexp.MustCompile(`^\s*(?:pub(?:\([\w:]+\))?\s+)?use\s+[^;{}]+(?:\{[^}]*\})?;\s*$`), 0},
	"c":          {regexp.MustCompile(`^\s*#\s*include\s*([<"][^>"]+[>"])\s*$`), 1},
	"cpp":        {regexp.MustCompile(`^\s*#\s*include\s*([<"][^>"]+[>"])\s*$`), 1},
	"objc":       {regexp.MustCompile(`^\s*#\s*(?:include|import)\s*([<"][^>"]+[>"])\s*$`), 1},
	"php":        {regexp.MustCompile(`^\s*use\s+(?:function\s+|const\s+)?([\w\\]+)(?:\s+as\s+\w+)?\s*;\s*$`), 0},
	"swift":      {regexp.MustCompile(`^\s*(?:@\w+\s+)?import\s+(\w+)\s*$`), 1},
	"dart":       {regexp.MustCompile(`^\s*(?:import|export)\s+('[^']+'|"[^"]+")[^;]*;\s*$`), 0},
	"ruby":       {regexp.MustCompile(`^\s*require(?:_relative)?\s*\(?\s*(['"][^'"]+['"])\s*\)?\s*$`), 1},
	"haskell":    {regexp.MustCompile(`^\s*import\s+(?:qualified\s+)?([\w.]+)\b.*$`), 0},
	"elixir":     {regexp.MustCompile(`^\s*(?:alias|import|require|use)\s+[\w.]+(?:,.*)?$`), 0},
}

// importLine is a line of an import list, with what it imports; key is "" for a blank line.
type importLine struct{ text, key string }

// mergeSet merges a place of imports as a set. ok is false when it isn't one, or the sides
// import one module in two different ways.
func mergeSet(id, base, ours, theirs string) (Suggestion, bool) {
	rule, known := importLines[id]
	if !known {
		return Suggestion{}, false
	}
	read := func(s string) ([]importLine, bool) {
		var out []importLine
		for l := range strings.SplitAfterSeq(s, "\n") {
			if l == "" {
				continue
			}
			bare := strings.TrimRight(l, "\r\n")
			if strings.TrimSpace(bare) == "" {
				out = append(out, importLine{text: l})
				continue
			}
			m := rule.line.FindStringSubmatch(bare)
			if m == nil {
				return nil, false
			}
			k := strings.Join(strings.Fields(bare), " ")
			if rule.key > 0 {
				k = m[rule.key]
			}
			out = append(out, importLine{text: l, key: k})
		}
		return out, true
	}
	b, okb := read(base)
	o, oko := read(ours)
	t, okt := read(theirs)
	if !okb || !oko || !okt {
		return Suggestion{}, false
	}
	whole := func(ls []importLine) map[string]string {
		m := map[string]string{}
		for _, l := range ls {
			if l.key != "" {
				m[l.key] = strings.TrimSpace(l.text)
			}
		}
		return m
	}
	inBase, inOurs, inTheirs := whole(b), whole(o), whole(t)
	// One module imported differently by the two sides, or changed by one and dropped by the
	// other, is a real conflict.
	for k, ol := range inOurs {
		if tl, ok := inTheirs[k]; ok && tl != ol && inBase[k] != ol && inBase[k] != tl {
			return Suggestion{}, false
		}
	}
	for k, was := range inBase {
		ol, ko := inOurs[k]
		tl, kt := inTheirs[k]
		if (ko && !kt && ol != was) || (kt && !ko && tl != was) {
			return Suggestion{}, false
		}
	}

	// Ours, less what theirs removed, then what theirs added that ours lacks.
	var out []importLine
	var side []string
	var oursAdded, theirsAdded []string
	for _, l := range o {
		if l.key != "" {
			if _, was := inBase[l.key]; was {
				if _, kept := inTheirs[l.key]; !kept {
					continue
				}
				// Theirs changed it (an alias, say) and ours didn't: theirs' version.
				if tl := inTheirs[l.key]; tl != inBase[l.key] && strings.TrimSpace(l.text) == inBase[l.key] {
					l.text = strings.Replace(l.text, strings.TrimSpace(l.text), tl, 1)
					out, side = append(out, l), append(side, "theirs")
					continue
				}
			} else {
				oursAdded = append(oursAdded, strings.TrimSpace(l.text))
			}
		}
		out, side = append(out, l), append(side, "")
		if l.key != "" && inBase[l.key] == "" {
			side[len(side)-1] = "ours"
		}
	}
	indent := ""
	for _, l := range o {
		if l.key != "" {
			indent = l.text[:len(l.text)-len(strings.TrimLeft(l.text, " \t"))]
			break
		}
	}
	lineEnd := "\n"
	if strings.Contains(ours, "\r\n") {
		lineEnd = "\r\n"
	}
	for _, l := range t {
		if l.key == "" {
			continue
		}
		if _, was := inBase[l.key]; was {
			continue
		}
		if _, have := inOurs[l.key]; have {
			continue
		}
		theirsAdded = append(theirsAdded, strings.TrimSpace(l.text))
		at := len(out)
		// After the last import, not after trailing blank lines.
		for at > 0 && out[at-1].key == "" {
			at--
		}
		nl := importLine{text: indent + strings.TrimSpace(l.text) + lineEnd, key: l.key}
		out = slices.Insert(out, at, nl)
		side = slices.Insert(side, at, "theirs")
	}
	if len(oursAdded) == 0 && len(theirsAdded) == 0 {
		return Suggestion{}, false
	}
	// Lists kept in order on both sides stay in order.
	if sorted(o) && sorted(t) && !slices.ContainsFunc(out, func(l importLine) bool { return l.key == "" }) {
		idx := make([]int, len(out))
		for i := range idx {
			idx[i] = i
		}
		slices.SortStableFunc(idx, func(x, y int) int {
			return strings.Compare(strings.TrimSpace(out[x].text), strings.TrimSpace(out[y].text))
		})
		o2, s2 := make([]importLine, len(out)), make([]string, len(out))
		for i, k := range idx {
			o2[i], s2[i] = out[k], side[k]
		}
		out, side = o2, s2
	}

	s := Suggestion{OK: true, Origins: []Origin{}}
	var sb strings.Builder
	at := 0
	for i, l := range out {
		n := utf16Len(l.text)
		if side[i] != "" {
			body := strings.TrimRight(l.text, "\r\n")
			lead := len(body) - len(strings.TrimLeft(body, " \t"))
			s.Origins = append(s.Origins, Origin{From: at + lead, To: at + utf16Len(body), Side: side[i]})
		}
		sb.WriteString(l.text)
		at += n
	}
	s.Text = sb.String()
	var did []string
	if len(oursAdded) > 0 {
		did = append(did, "ours added "+listOf(oursAdded))
	}
	if len(theirsAdded) > 0 {
		did = append(did, "theirs added "+listOf(theirsAdded))
	}
	s.Reason = "imports merge as a set: " + strings.Join(did, "; ")
	return s, true
}

// sorted reports whether a list's imports are in order.
func sorted(ls []importLine) bool {
	prev := ""
	for _, l := range ls {
		if l.key == "" {
			continue
		}
		cur := strings.TrimSpace(l.text)
		if cur < prev {
			return false
		}
		prev = cur
	}
	return true
}

// listOf names a few imports for a sentence.
func listOf(lines []string) string {
	if len(lines) == 1 {
		return quoted([]string{lines[0]})
	}
	return quoted([]string{lines[0]}) + " and " + strconv.Itoa(len(lines)-1) + " more"
}
