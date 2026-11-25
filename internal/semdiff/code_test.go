package semdiff

import (
	"context"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

// readDiff compares two versions read as code in language id, and checks the blocks still
// rebuild the new text, as staging needs.
func readDiff(t *testing.T, id, before, after string) Result {
	t.Helper()
	res, err := Diff(context.Background(), before, after, Options{Language: id, Moves: true, ReadTime: -1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Engine != EngineCode {
		t.Fatalf("not read as code: fallback %q", res.Fallback)
	}
	checkBlocks(t, before, after, res.Blocks)
	return res
}

// kinds lists the blocks' kinds, with "change" for KindChange.
func kinds(res Result) []string {
	var out []string
	for _, b := range res.Blocks {
		k := b.Kind
		if k == KindChange {
			k = "change"
		}
		out = append(out, k)
	}
	return out
}

// marked lists a side's marked text as kind:text, with "change" for KindChange.
func marked(s string, spans func(Block) []Span, res Result) []string {
	t := split(s)
	var out []string
	for _, b := range res.Blocks {
		for _, sp := range spans(b) {
			line := []rune(strings.TrimRight(t.line(sp.Line), "\r\n"))
			k := sp.Kind
			if k == KindChange {
				k = "change"
			}
			// The tests' text is in the Basic Multilingual Plane, so UTF-16 counts runes.
			out = append(out, k+":"+string(line[sp.From:sp.To]))
		}
	}
	return out
}

func oldMarks(s string, res Result) []string {
	return marked(s, func(b Block) []Span { return b.Old }, res)
}

func newMarks(s string, res Result) []string {
	return marked(s, func(b Block) []Span { return b.New }, res)
}

func same(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%s: got %q, want %q", what, got, want)
	}
}

func TestANumberWrittenAnotherWayIsFormatting(t *testing.T) {
	before := "package p\n\nconst maxHours = 0x30\n\nvar x = 1\n"
	after := "package p\n\nconst maxHours = 48\n\nvar x = 1\n"
	res := readDiff(t, "go", before, after)
	same(t, "kinds", kinds(res), "format")
	if res.Blocks[0].Note != "same value" {
		t.Fatalf("note %q", res.Blocks[0].Note)
	}
	same(t, "marks", newMarks(after, res), "format:48")
}

func TestADifferentNumberIsAChange(t *testing.T) {
	before := "package p\n\nconst maxHours = 48\n"
	after := "package p\n\nconst maxHours = 72\n"
	res := readDiff(t, "go", before, after)
	same(t, "kinds", kinds(res), "change")
	same(t, "old marks", oldMarks(before, res), "change:48")
	same(t, "new marks", newMarks(after, res), "change:72")
}

func TestRealignedFieldsAreFormatting(t *testing.T) {
	before := "package p\n\ntype Reading struct {\n\tAt time.Time\n\tHeight float64\n}\n"
	after := "package p\n\ntype Reading struct {\n\tAt     time.Time\n\tHeight float64\n}\n"
	res := readDiff(t, "go", before, after)
	same(t, "kinds", kinds(res), "format")
	if res.Blocks[0].Note != "spacing" {
		t.Fatalf("note %q", res.Blocks[0].Note)
	}
}

func TestLineBreaksAndATrailingCommaAreFormatting(t *testing.T) {
	before := "package p\n\nfunc f() {\n\tg(a, b)\n}\n"
	after := "package p\n\nfunc f() {\n\tg(\n\t\ta,\n\t\tb,\n\t)\n}\n"
	res := readDiff(t, "go", before, after)
	same(t, "kinds", kinds(res), "format")
	same(t, "new marks", newMarks(after, res), "format:,")
}

func TestOptionalParenthesesAreFormatting(t *testing.T) {
	before := "package p\n\nfunc f(a, b int) int {\n\tif (a > b) {\n\t\treturn (a + b)\n\t}\n\treturn 0\n}\n"
	after := "package p\n\nfunc f(a, b int) int {\n\tif a > b {\n\t\treturn a + b\n\t}\n\treturn 0\n}\n"
	res := readDiff(t, "go", before, after)
	same(t, "kinds", kinds(res), "format")
	if res.Blocks[0].Note != "parentheses" {
		t.Fatalf("note %q", res.Blocks[0].Note)
	}
}

func TestParenthesesThatMatterAreAChange(t *testing.T) {
	// (a + b) * c isn't a + b * c.
	before := "package p\n\nvar x = (a + b) * c\n"
	after := "package p\n\nvar x = a + b * c\n"
	res := readDiff(t, "go", before, after)
	same(t, "kinds", kinds(res), "change")
}

func TestFormattingStandsApartFromAChangeNextToIt(t *testing.T) {
	before := "package p\n\nconst a = 0x10\nconst b = 1\n"
	after := "package p\n\nconst a = 16\nconst b = 2\n"
	res := readDiff(t, "go", before, after)
	same(t, "kinds", kinds(res), "format", "change")
}

func TestARenameIsFoundWhereverItHappened(t *testing.T) {
	before := "package p\n\nfunc f(n int) []int {\n\tout := make([]int, 0, n)\n\tout = append(out, n)\n\treturn out\n}\n"
	after := "package p\n\nfunc f(n int) []int {\n\treadings := make([]int, 0, n)\n\treadings = append(readings, n)\n\treturn readings\n}\n"
	res := readDiff(t, "go", before, after)
	if len(res.Renames) != 1 || res.Renames[0] != (Rename{From: "out", To: "readings", Count: 4}) {
		t.Fatalf("renames %+v", res.Renames)
	}
	same(t, "kinds", kinds(res), "rename")
	if res.Blocks[0].Note != "renamed from out" {
		t.Fatalf("note %q", res.Blocks[0].Note)
	}
	same(t, "marks", newMarks(after, res), "rename:readings", "rename:readings", "rename:readings", "rename:readings")
}

func TestEachRenameNumbersItsPlaces(t *testing.T) {
	// Two renames side by side on a line stay apart, each with its own number.
	before := "package p\n\nfunc f(a, b int) int {\n\tx := a + b\n\ty := a * b\n\treturn x - y + a + b\n}\n"
	after := "package p\n\nfunc f(lo, hi int) int {\n\tx := lo + hi\n\ty := lo * hi\n\treturn x - y + lo + hi\n}\n"
	res := readDiff(t, "go", before, after)
	if len(res.Renames) != 2 {
		t.Fatalf("renames %+v", res.Renames)
	}
	lines := strings.Split(after, "\n")
	got := map[string][]int{}
	for _, b := range res.Blocks {
		for _, s := range b.New {
			if s.Kind == KindRename {
				r := []rune(lines[s.Line])
				got[string(r[s.From:s.To])] = append(got[string(r[s.From:s.To])], s.Rename)
			}
		}
	}
	for i, rn := range res.Renames {
		if n := got[rn.To]; len(n) != rn.Count || slices.ContainsFunc(n, func(g int) bool { return g != i+1 }) {
			t.Errorf("%s: numbered %v, want %d places numbered %d", rn.To, n, rn.Count, i+1)
		}
	}
}

func TestANameChangedInOnePlaceIsAChange(t *testing.T) {
	before := "package p\n\nfunc f() int { return a }\n"
	after := "package p\n\nfunc f() int { return b }\n"
	res := readDiff(t, "go", before, after)
	same(t, "kinds", kinds(res), "change")
	if len(res.Renames) != 0 {
		t.Fatalf("renames %+v", res.Renames)
	}
}

func TestAChangedStringIsMarkedByWord(t *testing.T) {
	before := "package p\n\nvar e = fmt.Errorf(\"at most %d hours\", n)\n"
	after := "package p\n\nvar e = fmt.Errorf(\"at most %d readings\", n)\n"
	res := readDiff(t, "go", before, after)
	same(t, "old marks", oldMarks(before, res), "change:hours")
	same(t, "new marks", newMarks(after, res), "change:readings")
}

func TestAnAddedArgumentIsMarkedWithItsComma(t *testing.T) {
	before := "package p\n\nfunc Forecast(from time.Time, n int) {}\n"
	after := "package p\n\nfunc Forecast(from time.Time, n int, step time.Duration) {}\n"
	res := readDiff(t, "go", before, after)
	same(t, "new marks", newMarks(after, res), "change:, step time.Duration")
}

func TestIndentingPythonIntoABlockIsAChange(t *testing.T) {
	before := "if x:\n    a()\nb()\n"
	after := "if x:\n    a()\n    b()\n"
	res := readDiff(t, "python", before, after)
	same(t, "kinds", kinds(res), "change")
}

func TestAOneItemTupleKeepsItsComma(t *testing.T) {
	res := readDiff(t, "python", "t = (1,)\n", "t = (1)\n")
	same(t, "kinds", kinds(res), "change")
}

func TestPythonQuotesAreFormatting(t *testing.T) {
	res := readDiff(t, "python", "s = 'a\\tb'\n", "s = \"a\\x09b\"\n")
	same(t, "kinds", kinds(res), "format")
	res = readDiff(t, "python", "s = r'\\n'\n", "s = '\\n'\n")
	same(t, "kinds", kinds(res), "change")
	res = readDiff(t, "python", "s = b'a'\n", "s = 'a'\n")
	same(t, "kinds", kinds(res), "change")
}

func TestJavaScriptSemicolonsAndQuotesAreFormatting(t *testing.T) {
	res := readDiff(t, "javascript", "const a = 'x';\nf(a);\n", "const a = \"x\"\nf(a)\n")
	same(t, "kinds", kinds(res), "format")
}

func TestASemicolonThatChangesTheCodeIsAChange(t *testing.T) {
	// Without the semicolon, (b) calls a's result.
	res := readDiff(t, "javascript", "let x = a;\n(b)\n", "let x = a\n(b)\n")
	same(t, "kinds", kinds(res), "change")
}

func TestJavaScriptNumbersAreOneType(t *testing.T) {
	res := readDiff(t, "javascript", "const a = 1.0\n", "const a = 0x1\n")
	same(t, "kinds", kinds(res), "format")
	res = readDiff(t, "javascript", "const a = 010\n", "const a = 8\n")
	same(t, "kinds", kinds(res), "change")
}

func TestACSSDescendantIsNotACompound(t *testing.T) {
	res := readDiff(t, "css", ".a .b { color: red }\n", ".a.b { color: red }\n")
	same(t, "kinds", kinds(res), "change")
	res = readDiff(t, "css", ".a { margin: 0.5em }\n", ".a {\n  margin: .5em;\n}\n")
	same(t, "kinds", kinds(res), "format")
}

func TestReformattedJSONIsFormatting(t *testing.T) {
	res := readDiff(t, "json", "{\"a\": [1, 2], \"b\": 1.50}\n", "{\n  \"a\": [\n    1,\n    2\n  ],\n  \"b\": 1.5\n}\n")
	same(t, "kinds", kinds(res), "format")
	res = readDiff(t, "json", "{\"a\": 1}\n", "{\"a\": 1.0}\n")
	same(t, "kinds", kinds(res), "change")
}

func TestHTMLWhitespaceCountsByItsPresence(t *testing.T) {
	res := readDiff(t, "html", "<p>Some text\nwrapped here</p>\n", "<p>Some\n   text wrapped here</p>\n")
	same(t, "kinds", kinds(res), "format")
	res = readDiff(t, "html", "<p><b>a</b> b</p>\n", "<p><b>a</b>b</p>\n")
	same(t, "kinds", kinds(res), "change")
	res = readDiff(t, "html", "<pre>a b</pre>\n", "<pre>a  b</pre>\n")
	same(t, "kinds", kinds(res), "change")
}

func TestVueScriptsAreReadAsTheirLanguage(t *testing.T) {
	before := "<template>\n  <p>{{ msg }}</p>\n</template>\n\n<script setup lang=\"ts\">\nconst msg = 'hi'\n</script>\n"
	after := "<template>\n  <p>{{msg}}</p>\n</template>\n\n<script setup lang=\"ts\">\nconst msg = \"hi\";\n</script>\n"
	res := readDiff(t, "vue", before, after)
	same(t, "kinds", kinds(res), "format", "format")
}

func TestLuaKeepsItsParentheses(t *testing.T) {
	// (f()) keeps only f's first result.
	res := readDiff(t, "lua", "return (f())\n", "return f()\n")
	same(t, "kinds", kinds(res), "change")
}

func TestTheLanguagesReadTheirLiterals(t *testing.T) {
	for _, c := range []struct {
		id, before, after string
		format            bool
	}{
		{"go", "var s = \"a\\tb\"\n", "var s = `a\tb`\n", true},
		{"go", "var n = 017\n", "var n = 0o17\n", true},
		{"rust", "const N: u32 = 1_000u32;\n", "const N: u32 = 1000u32;\n", true},
		{"rust", "const N: u32 = 1000u32;\n", "const N: u32 = 1000;\n", false},
		{"rust", "let s = \"a\\x41\";\n", "let s = r\"aA\";\n", true},
		{"csharp", "class K { long l = 10L; }\n", "class K { long l = 10l; }\n", true},
		{"csharp", "class K { string s = \"a\\\"b\"; }\n", "class K { string s = @\"a\"\"b\"; }\n", true},
		{"csharp", "class K { decimal d = 1.50m; }\n", "class K { decimal d = 1.5m; }\n", false},
		{"java", "class K { int n = 017; }\n", "class K { int n = 15; }\n", true},
		{"java", "class K { String s = \"\\101\"; }\n", "class K { String s = \"A\"; }\n", true},
		{"lua", "local s = 'a\\65'\n", "local s = [[aA]]\n", true},
		{"lua", "local n = 1\n", "local n = 1.0\n", false},
		{"typescript", "const s = `plain`;\n", "const s = 'plain';\n", true},
		{"typescript", "const s = `${a}`;\n", "const s = 'a';\n", false},
		{"scss", ".a { content: \"\\41\"; }\n", ".a { content: 'A'; }\n", true},
	} {
		res := readDiff(t, c.id, c.before, c.after)
		got := len(res.Blocks) == 1 && res.Blocks[0].Kind == KindFormat
		if got != c.format {
			t.Errorf("%s %q → %q: formatting only %v, want %v (%+v)", c.id, c.before, c.after, got, c.format, res.Blocks)
		}
	}
}

func TestCodeBlocksRebuildTheNewText(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 45))
	for range 60 {
		before := codeText(r, 10+r.IntN(40))
		after := codeMutate(r, before)
		for _, src := range [][2]string{{before, after}, {"package p\n" + before, "package p\n" + after}} {
			res, err := Diff(context.Background(), src[0], src[1], Options{Language: "go", Moves: true})
			if err != nil {
				t.Fatal(err)
			}
			checkBlocks(t, src[0], src[1], res.Blocks)
			for _, mv := range res.Moves {
				if !inBlocks(res.Blocks, mv) {
					t.Fatalf("move %+v lies outside the blocks %+v", mv, res.Blocks)
				}
			}
		}
	}
}

func inBlocks(blocks []Block, mv Move) bool {
	for i := mv.OldStart; i < mv.OldEnd; i++ {
		ok := false
		for _, b := range blocks {
			ok = ok || i >= b.OldStart && i < b.OldEnd
		}
		if !ok {
			return false
		}
	}
	for j := mv.NewStart; j < mv.NewEnd; j++ {
		ok := false
		for _, b := range blocks {
			ok = ok || j >= b.NewStart && j < b.NewEnd
		}
		if !ok {
			return false
		}
	}
	return true
}

func TestFallsBackToLines(t *testing.T) {
	big := "package p\n\n" + strings.Repeat("var x = 1\n", maxCodeBytes/10)
	slow := "package p\n\n" + strings.Repeat("func f() { x := []int{1, 2, 3}; _ = x }\n", 2000)
	for _, c := range []struct {
		name, before, after string
		opts                Options
		want                string
	}{
		{"size", big, big + "var y = 2\n", Options{Language: "go"}, FallbackSize},
		{"time", slow, slow + "var y = 2\n", Options{Language: "go", ReadTime: time.Microsecond}, FallbackTime},
		// No clock for errors: under the default limit a busy runner can run out of time first.
		{"errors", "package p\nvar x = 1\n", "}}}} ))) {{{{ ]]]] ;;;; ,,,,\nvar x = 2\n", Options{Language: "go", ReadTime: -1}, FallbackErrors},
	} {
		res, err := Diff(context.Background(), c.before, c.after, c.opts)
		if err != nil {
			t.Fatal(err)
		}
		if res.Engine != EngineLines || res.Fallback != c.want || res.Language != "Go" {
			t.Errorf("%s: engine %q fallback %q language %q", c.name, res.Engine, res.Fallback, res.Language)
		}
		checkBlocks(t, c.before, c.after, res.Blocks)
	}
}

func TestASmallParseErrorIsReadAroundAndNoted(t *testing.T) {
	before := "package p\n\nfunc f() {\n\tx := 1\n\ty := 2\n\tz := 3\n\tw := 4\n}\n"
	after := "package p\n\nfunc f() {\n\tx := 1\n\ty := 2 +\n\tz := 3\n\tw := 5\n}\n"
	res := readDiff(t, "go", before, after)
	if !res.Unparsed {
		t.Fatal("the parse error isn't noted")
	}
	for _, b := range res.Blocks {
		if b.Kind == KindFormat {
			t.Fatalf("part of a parse error is called formatting: %+v", b)
		}
	}
}

func TestLanguageOfReadsAttributesFirst(t *testing.T) {
	for _, c := range []struct{ file, linguist, driver, want string }{
		{"a/b.go", "", "", "go"},
		{"a/b.TSX", "", "", "tsx"},
		{"a/b.txt", "Go", "", "go"},
		{"a/b.go", "Fortran", "", ""},
		{"a/b.inc", "", "csharp", "csharp"},
		{"a/b.go", "", "weird", "go"},
		{"LICENSE", "", "", ""},
		{"a/app.csproj", "", "", "xml"},
	} {
		if got := LanguageOf(c.file, c.linguist, c.driver); got != c.want {
			t.Errorf("LanguageOf(%q, %q, %q) = %q, want %q", c.file, c.linguist, c.driver, got, c.want)
		}
	}
}
