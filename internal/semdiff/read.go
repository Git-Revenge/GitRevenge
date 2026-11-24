package semdiff

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ts "github.com/odvcencio/gotreesitter"
)

// Budgets for reading code (ADR 0015, measured in #943). Past any of them the file gets the
// line diff, and the result says which.
const (
	// maxCodeBytes is the largest version read as code, checked before parsing.
	maxCodeBytes = 1 << 20
	// QuickRead is how long a version may take to parse before the views get its line diff.
	QuickRead = 150 * time.Millisecond
	// PatientRead is how long a second, background parse may take, for a file that missed
	// QuickRead.
	PatientRead = 3 * time.Second
	// LongRead is how long a parse asked for anyway may take.
	LongRead = time.Minute
	// maxTokens is the most tokens a version may have.
	maxTokens = 200_000
	// parseMemory caps each parse's memory. gotreesitter's default is 512 MB.
	parseMemory = 256 << 20
	// maxErrorShare is the share of a version's bytes that may sit inside parse errors.
	maxErrorShare = 0.25
)

// Fallbacks say why a file in a language read as code got the line diff.
const (
	FallbackSize   = "size"   // a version is over maxCodeBytes
	FallbackTime   = "time"   // a parse ran out of time
	FallbackMemory = "memory" // a parse ran out of memory
	FallbackErrors = "errors" // over maxErrorShare of a version didn't parse
	FallbackTokens = "tokens" // a version has over maxTokens tokens
)

// class is what a token is, for marking it and finding renames.
type class uint8

const (
	clsPunct   class = iota // keywords, operators and brackets
	clsIdent                // a name
	clsNumber               // a number literal
	clsString               // a string or character literal
	clsComment              // a comment
	clsText                 // a word of markup prose, or raw text
	clsSpace                // whitespace in markup content, which counts by its presence
	clsGap                  // text the tree left out, kept so no change goes unseen
)

// tok is a token of a version: a leaf of its syntax tree, or a literal, comment or word read
// whole. Whitespace between tokens isn't a token, except in markup content.
type tok struct {
	from, to  int    // bytes in the file
	key       string // what it means: equal keys are the same token
	class     class
	ignorable bool   // an optional trailing comma, parenthesis or semicolon
	inError   bool   // inside a parse error, or text the tree left out: compared as written
	path      uint64 // the syntax nodes around it, by type; equal keys in different places differ
	open      uint16 // syntax nodes that start here
	close     uint16 // syntax nodes that end here
}

// reading is a version read as code.
type reading struct {
	toks     []tok
	stop     string // a Fallback, when the version couldn't be read
	errBytes int    // bytes inside parse errors
	missing  bool   // the parser filled in something missing
}

// readCode reads both versions in language id, each in its own goroutine. Each parse may
// take timeout, or as long as it likes when timeout is negative. It returns ctx's error if
// ctx ends first.
func readCode(ctx context.Context, id string, before, after string, timeout time.Duration) (ra, rb reading, err error) {
	l := byID(id)
	l.load()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); ra = readVersion(ctx, l, before, timeout) }()
	go func() { defer wg.Done(); rb = readVersion(ctx, l, after, timeout) }()
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return reading{}, reading{}, err
	}
	return ra, rb, nil
}

// readVersion parses src and reads its tokens.
func readVersion(ctx context.Context, l *language, src string, timeout time.Duration) reading {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	var flag uint32
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			atomic.StoreUint32(&flag, 1)
		case <-done:
		}
	}()
	r := &reader{src: src, deadline: deadline, flag: &flag}
	r.read(l, 0, len(src), 0)
	if r.stop == "" && len(r.toks) > maxTokens {
		r.stop = FallbackTokens
	}
	if r.stop == "" {
		r.fillGaps()
	}
	return reading{toks: r.toks, stop: r.stop, errBytes: r.errBytes, missing: r.missing}
}

// reader walks syntax trees into tokens. One file can hold several trees: a Vue file's
// script is JavaScript.
type reader struct {
	src      string
	deadline time.Time
	flag     *uint32
	toks     []tok
	stop     string
	errBytes int
	missing  bool
}

// read parses src[from:to] in language l and walks it. path is the syntax around it.
func (r *reader) read(l *language, from, to int, path uint64) {
	lang := l.load()
	p := ts.NewParser(lang)
	p.SetMemoryBudgetBytes(parseMemory)
	p.SetCancellationFlag(r.flag)
	if !r.deadline.IsZero() {
		left := time.Until(r.deadline)
		if left <= 0 {
			r.stop = FallbackTime
			return
		}
		p.SetTimeoutMicros(uint64(left.Microseconds()) + 1)
	}
	tree, err := p.Parse([]byte(r.src[from:to]))
	if err != nil || tree == nil {
		r.stop = FallbackErrors
		return
	}
	defer tree.Release()
	if tree.ParseStoppedEarly() {
		switch tree.ParseStopReason() {
		case ts.ParseStopTimeout, ts.ParseStopCancelled:
			r.stop = FallbackTime
		case ts.ParseStopMemoryBudget:
			r.stop = FallbackMemory
		default:
			r.stop = FallbackErrors
		}
		return
	}
	w := walker{r: r, l: l, lang: lang, base: from}
	w.walk(tree.RootNode(), path, false, false, false)
}

// walker reads one tree's tokens.
type walker struct {
	r    *reader
	l    *language
	lang *ts.Language
	base int // where the tree's text starts in the file
}

// mix adds a syntax node to a path, by its type's name: a grammar can give one name several
// symbols (CSS's last declaration in a block is one), and those are the same syntax.
func mix(path uint64, name uint64) uint64 {
	const prime = 1099511628211
	return (path ^ name) * prime
}

// walk reads n's tokens. inError is set inside a parse error, keep inside markup whose
// whitespace counts exactly (a <pre>), and optional when n is a parenthesized expression
// whose parentheses don't count.
func (w *walker) walk(n *ts.Node, path uint64, inError, keep, optional bool) {
	r := w.r
	if r.stop != "" {
		return
	}
	if n.IsMissing() {
		r.missing = true
		return
	}
	from, to := w.base+int(n.StartByte()), w.base+int(n.EndByte())
	if n.IsError() {
		if !inError {
			r.errBytes += to - from
		}
		inError = true
	}
	sym := n.Symbol()
	rule := w.l.rule(sym)
	if n.IsError() {
		rule = nodeRule{}
	}
	// A literal's path is where it sits, not its own type, so "a" and `a` can match.
	inner := path
	if n.IsNamed() && !optional && !n.IsError() {
		inner = mix(path, w.l.nameHash(sym))
	}
	kids := n.Children()
	first := len(r.toks)
	defer func() {
		if n.IsNamed() && len(r.toks) > first {
			r.toks[first].open++
			r.toks[len(r.toks)-1].close++
		}
	}()

	switch rule.kind {
	case nodeNumber:
		w.atom(from, to, clsNumber, path, inError, func(s string) (string, bool) { return numKey(w.l.id, s) })
		return
	case nodeString:
		if !w.interpolated(kids) {
			typ := n.Type(w.lang)
			w.atom(from, to, clsString, path, inError, func(s string) (string, bool) { return strKey(w.l.id, typ, s) })
			return
		}
	case nodeComment:
		w.atom(from, to, clsComment, path, inError, func(s string) (string, bool) { return "#" + collapse(s), true })
		return
	case nodeAtom:
		w.atom(from, to, clsText, path, inError, nil)
		return
	case nodeText:
		w.text(from, to, path, inError, keep)
		return
	case nodeScript:
		w.script(n, kids, inner, inError)
		return
	case nodeExpr:
		w.expr(kids, inner, inError)
		return
	}
	if len(kids) == 0 {
		w.leaf(from, to, rule, inner, inError)
		return
	}
	if w.l.rules.markup && keepsSpace(n, w.lang, w.r.src[w.base:]) {
		keep = true
	}
	content := w.l.rules.markup && rule.kind != nodeTag
	for i, k := range kids {
		if content && i > 0 {
			w.space(w.base+int(kids[i-1].EndByte()), w.base+int(k.StartByte()), inner, keep)
		}
		at := len(r.toks)
		redundant := false
		if rule.parens > 0 && w.l.rule(k.Symbol()).kind == nodeParens && !k.IsError() {
			redundant = (rule.parens == 1 || i == lastNamed(kids)) && w.optionalParens(k)
		}
		w.walk(k, inner, inError, keep, redundant)
		if r.stop != "" {
			return
		}
		// An optional trailing comma (or CSS's last semicolon): the last item's, before the
		// closing bracket.
		if last := len(r.toks) - 1; rule.commas && last >= at && (r.toks[last].key == "," || r.toks[last].key == ";") && w.closesNext(kids, i) {
			r.toks[last].ignorable = true
		}
	}
	if optional && len(r.toks)-first >= 2 && r.toks[first].key == "(" && r.toks[len(r.toks)-1].key == ")" {
		r.toks[first].ignorable = true
		r.toks[len(r.toks)-1].ignorable = true
	}
}

// interpolated says whether a string node holds code, such as a template's ${}.
func (w *walker) interpolated(kids []*ts.Node) bool {
	for _, k := range kids {
		if w.l.rule(k.Symbol()).interp {
			return true
		}
	}
	return false
}

// optionalParens says whether parenthesized expression p's parentheses can go: they can
// unless what they hold needs them.
func (w *walker) optionalParens(p *ts.Node) bool {
	kids := p.Children()
	if len(kids) < 3 {
		return false
	}
	named := 0
	for _, k := range kids {
		if k.IsNamed() {
			named++
			if w.l.rule(k.Symbol()).keepParens || w.l.rule(k.Symbol()).kind == nodeComment {
				return false
			}
		}
	}
	return named == 1 && kids[0].ChildCount() == 0 && kids[len(kids)-1].ChildCount() == 0
}

// closesNext says whether the next child after i, comments aside, is a closing bracket.
func (w *walker) closesNext(kids []*ts.Node, i int) bool {
	for j := i + 1; j < len(kids); j++ {
		rule := w.l.rule(kids[j].Symbol())
		if rule.kind == nodeComment {
			continue
		}
		if kids[j].ChildCount() != 0 {
			return false
		}
		switch w.r.src[w.base+int(kids[j].StartByte()) : w.base+int(kids[j].EndByte())] {
		case ")", "]", "}":
			return true
		}
		return false
	}
	return false
}

func lastNamed(kids []*ts.Node) int {
	for i := len(kids) - 1; i >= 0; i-- {
		if kids[i].IsNamed() {
			return i
		}
	}
	return -1
}

// leaf reads a token as written. A ; is optional where the language says so.
func (w *walker) leaf(from, to int, rule nodeRule, path uint64, inError bool) {
	s := w.r.src[from:to]
	if strings.TrimSpace(s) == "" {
		return
	}
	c := clsPunct
	if rule.kind == nodeIdent {
		c = clsIdent
	}
	w.add(tok{from: from, to: to, key: s, class: c, path: path, inError: inError, ignorable: s == ";" && w.l.rules.semis})
}

// atom reads [from, to) as one token, keyed by key, or as written when key can't read it.
func (w *walker) atom(from, to int, c class, path uint64, inError bool, key func(string) (string, bool)) {
	s := w.r.src[from:to]
	k, ok := "", false
	if key != nil && !inError {
		k, ok = key(s)
	}
	if !ok {
		k = "=" + s
	}
	w.add(tok{from: from, to: to, key: k, class: c, path: path, inError: inError})
}

// text reads markup prose as words, with each run of whitespace one space token; keep keeps
// it as written.
func (w *walker) text(from, to int, path uint64, inError, keep bool) {
	s := w.r.src[from:to]
	if keep {
		w.add(tok{from: from, to: to, key: "=" + s, class: clsText, path: path, inError: inError})
		return
	}
	for i := 0; i < len(s); {
		j := i
		if isSpace(s[i]) {
			for j < len(s) && isSpace(s[j]) {
				j++
			}
			w.space(from+i, from+j, path, false)
		} else {
			for j < len(s) && !isSpace(s[j]) {
				j++
			}
			w.add(tok{from: from + i, to: from + j, key: s[i:j], class: clsText, path: path, inError: inError})
		}
		i = j
	}
}

// space adds a space token for whitespace in [from, to) of markup content, if there is any
// and the token before isn't one already.
func (w *walker) space(from, to int, path uint64, keep bool) {
	s := w.r.src[from:to]
	if s == "" || strings.TrimSpace(s) != "" {
		return
	}
	key := " "
	if keep {
		key = "=" + s
	}
	if n := len(w.r.toks); n > 0 && w.r.toks[n-1].class == clsSpace && strings.TrimSpace(w.r.src[w.r.toks[n-1].to:from]) == "" {
		if !keep {
			return
		}
	}
	w.add(tok{from: from, to: to, key: key, class: clsSpace, path: path})
}

// script reads a <script> or <style> element, its raw text in the language its type or
// lang attribute names, or as written when it names none read here.
func (w *walker) script(n *ts.Node, kids []*ts.Node, path uint64, inError bool) {
	tag := n.Type(w.lang)
	sub := ""
	for _, k := range kids {
		switch k.Type(w.lang) {
		case "start_tag":
			sub = scriptLanguage(tag, attrs(k, w.lang, w.r.src[w.base:]))
			w.walk(k, path, inError, false, false)
		case "raw_text":
			from, to := w.base+int(k.StartByte()), w.base+int(k.EndByte())
			if sub == "" || inError {
				w.atom(from, to, clsText, path, inError, nil)
				continue
			}
			w.r.read(byID(sub), from, to, path)
		default:
			w.walk(k, path, inError, false, false)
		}
	}
}

// expr reads a Vue interpolation or directive, whose value is a JavaScript expression.
func (w *walker) expr(kids []*ts.Node, path uint64, inError bool) {
	for _, k := range kids {
		typ := k.Type(w.lang)
		var inner *ts.Node
		switch typ {
		case "raw_text":
			inner = k
		case "quoted_attribute_value":
			for _, v := range k.Children() {
				if v.Type(w.lang) == "attribute_value" {
					inner = v
				}
			}
			if inner == nil {
				w.walk(k, path, inError, false, false)
				continue
			}
			// The quotes stay tokens of their own.
			for _, v := range k.Children() {
				if v != inner {
					w.leaf(w.base+int(v.StartByte()), w.base+int(v.EndByte()), nodeRule{}, path, inError)
					continue
				}
				w.r.read(byID("javascript"), w.base+int(v.StartByte()), w.base+int(v.EndByte()), path)
			}
			continue
		}
		if inner != nil && !inError {
			w.r.read(byID("javascript"), w.base+int(inner.StartByte()), w.base+int(inner.EndByte()), path)
			continue
		}
		w.walk(k, path, inError, false, false)
	}
}

// add appends a token, stopping past the token budget.
func (w *walker) add(t tok) {
	r := w.r
	if len(r.toks) >= maxTokens {
		r.stop = FallbackTokens
		return
	}
	r.toks = append(r.toks, t)
}

// fillGaps adds a token for any text outside every token, other than whitespace, so a change
// there is never missed. Gaps are compared as written.
func (r *reader) fillGaps() {
	var out []tok
	at := 0
	gap := func(from, to int) {
		s := r.src[from:to]
		for i := 0; i < len(s); {
			if isSpace(s[i]) {
				i++
				continue
			}
			j := i
			for j < len(s) && !isSpace(s[j]) {
				j++
			}
			out = append(out, tok{from: from + i, to: from + j, key: "=" + s[i:j], class: clsGap, inError: true})
			i = j
		}
	}
	for _, t := range r.toks {
		if t.from < at {
			// Out of order can't happen with trees; a stray overlap is kept as it is.
			out = append(out, t)
			continue
		}
		gap(at, t.from)
		out = append(out, t)
		at = t.to
	}
	gap(at, len(r.src))
	r.toks = out
}

// attrs reads a markup start tag's attributes, by lower-cased name.
func attrs(tag *ts.Node, lang *ts.Language, src string) map[string]string {
	out := map[string]string{}
	for _, a := range tag.Children() {
		if !strings.HasSuffix(a.Type(lang), "attribute") {
			continue
		}
		name, value := "", ""
		for _, k := range a.Children() {
			switch k.Type(lang) {
			case "attribute_name":
				name = strings.ToLower(src[k.StartByte():k.EndByte()])
			case "attribute_value", "quoted_attribute_value":
				value = strings.Trim(src[k.StartByte():k.EndByte()], `"'`)
			}
		}
		if name != "" {
			out[name] = strings.ToLower(strings.TrimSpace(value))
		}
	}
	return out
}

// scriptLanguage is the language a <script> or <style> element holds, or "" for one not read
// here.
func scriptLanguage(tag string, attr map[string]string) string {
	if tag == "style_element" {
		switch attr["lang"] {
		case "", "css":
			if t := attr["type"]; t == "" || t == "text/css" {
				return "css"
			}
		case "scss":
			return "scss"
		}
		return ""
	}
	switch attr["lang"] {
	case "ts":
		return "typescript"
	case "tsx":
		return "tsx"
	case "", "js", "javascript", "jsx":
	default:
		return ""
	}
	switch attr["type"] {
	case "", "module", "text/javascript", "application/javascript", "text/babel":
		return "javascript"
	case "application/json", "importmap", "application/ld+json", "speculationrules":
		return "json"
	}
	return ""
}

// keepsSpace says whether n is a markup element whose text keeps its whitespace: <pre> and
// <textarea>.
func keepsSpace(n *ts.Node, lang *ts.Language, src string) bool {
	if n.Type(lang) != "element" {
		return false
	}
	kids := n.Children()
	if len(kids) == 0 || kids[0].Type(lang) != "start_tag" {
		return false
	}
	for _, k := range kids[0].Children() {
		if k.Type(lang) == "tag_name" {
			switch strings.ToLower(src[k.StartByte():k.EndByte()]) {
			case "pre", "textarea", "listing", "plaintext":
				return true
			}
			return false
		}
	}
	return false
}

// collapse turns each run of whitespace in s into one space.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// lineCol is where byte off of t is: its line and its UTF-16 column.
func lineCol(t text, off int) (line, col int) {
	lo, hi := 0, t.n()-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if t.start[mid] <= off {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, utf16Len(t.s[t.start[lo]:off])
}
