package semdiff

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// apply rebuilds after from before and the blocks, as staging does.
func apply(before, after string, blocks []Block) string {
	a, b := split(before), split(after)
	var out strings.Builder
	i := 0
	for _, blk := range blocks {
		out.WriteString(a.span(i, blk.OldStart))
		out.WriteString(b.span(blk.NewStart, blk.NewEnd))
		i = blk.OldEnd
	}
	out.WriteString(a.span(i, a.n()))
	return out.String()
}

// checkBlocks fails unless the blocks are in order, don't overlap, and turn before into
// after. Read as code, blocks of different kinds can touch.
func checkBlocks(t *testing.T, before, after string, blocks []Block) {
	t.Helper()
	for k, blk := range blocks {
		if blk.OldStart > blk.OldEnd || blk.NewStart > blk.NewEnd || blk.OldStart == blk.OldEnd && blk.NewStart == blk.NewEnd {
			t.Fatalf("block %d is malformed: %+v", k, blk)
		}
		if k > 0 {
			prev := blocks[k-1]
			if blk.OldStart < prev.OldEnd || blk.NewStart < prev.NewEnd {
				t.Fatalf("blocks %d and %d overlap: %+v, %+v", k-1, k, prev, blk)
			}
			if blk.OldStart == prev.OldEnd && blk.NewStart == prev.NewEnd && blk.Kind == prev.Kind {
				t.Fatalf("blocks %d and %d of one kind touch: %+v, %+v", k-1, k, prev, blk)
			}
			// What lies between them is unchanged, so it's as long on both sides.
			if blk.OldStart-prev.OldEnd != blk.NewStart-prev.NewEnd {
				t.Fatalf("blocks %d and %d leave unequal gaps: %+v, %+v", k-1, k, prev, blk)
			}
		}
	}
	if got := apply(before, after, blocks); got != after {
		t.Fatalf("applying the blocks gives\n%q\nwant\n%q", got, after)
	}
}

func TestSplitMatchesTheViews(t *testing.T) {
	// The frontend splits with /[^\n]*\n|[^\n]+$/g.
	for s, want := range map[string][]string{
		"":          nil,
		"a":         {"a"},
		"a\n":       {"a\n"},
		"a\r\nb":    {"a\r\n", "b"},
		"\n\n":      {"\n", "\n"},
		"a\nb\n\nc": {"a\n", "b\n", "\n", "c"},
	} {
		tx := split(s)
		var got []string
		for i := range tx.n() {
			got = append(got, tx.line(i))
		}
		if !slices.Equal(got, want) {
			t.Errorf("split(%q) = %q, want %q", s, got, want)
		}
	}
}

// randomText makes a text from a small vocabulary, so that lines repeat as they do in code.
func randomText(r *rand.Rand, lines int) string {
	vocab := []string{"{", "}", "", "return nil", "x := 1", "if err != nil {", "func f() {", "// note", "y++", "\tz := x"}
	var b strings.Builder
	for range lines {
		b.WriteString(vocab[r.IntN(len(vocab))])
		if r.IntN(10) == 0 {
			b.WriteString("\r\n")
		} else {
			b.WriteString("\n")
		}
	}
	if r.IntN(3) == 0 {
		b.WriteString("no newline")
	}
	return b.String()
}

func mutate(r *rand.Rand, s string) string {
	lines := strings.SplitAfter(s, "\n")
	for range r.IntN(8) {
		at := r.IntN(len(lines) + 1)
		switch r.IntN(3) {
		case 0:
			lines = slices.Insert(lines, at, fmt.Sprintf("added %d\n", r.IntN(100)))
		case 1:
			if at < len(lines) {
				lines = slices.Delete(lines, at, at+1)
			}
		case 2:
			if at < len(lines) {
				lines[at] = "changed " + lines[at]
			}
		}
	}
	return strings.Join(lines, "")
}

func TestEveryAlgorithmRebuildsTheNewText(t *testing.T) {
	r := rand.New(rand.NewPCG(944, 1))
	for n := range 400 {
		before := randomText(r, r.IntN(60))
		after := mutate(r, before)
		if n%7 == 0 {
			after = randomText(r, r.IntN(60))
		}
		for _, alg := range Algorithms {
			res, err := Diff(context.Background(), before, after, Options{Algorithm: alg, Moves: true})
			if err != nil {
				t.Fatal(err)
			}
			checkBlocks(t, before, after, res.Blocks)
			for _, mv := range res.Moves {
				if mv.OldEnd-mv.OldStart != mv.NewEnd-mv.NewStart || mv.OldStart == mv.OldEnd {
					t.Fatalf("%s: malformed move %+v", alg, mv)
				}
			}
		}
	}
}

func TestSameTextHasNoBlocks(t *testing.T) {
	for _, alg := range Algorithms {
		res, err := Diff(context.Background(), "a\nb\n", "a\nb\n", Options{Algorithm: alg, Moves: true})
		if err != nil || len(res.Blocks) != 0 || res.Added != 0 || res.Removed != 0 {
			t.Errorf("%s: %+v, %v", alg, res, err)
		}
		// The views read lists: null broke them, for histogram, after staging a whole file.
		js, _ := json.Marshal(res)
		if !strings.Contains(string(js), `"blocks":[]`) || !strings.Contains(string(js), `"moves":[]`) {
			t.Errorf("%s: %s, want empty lists", alg, js)
		}
	}
}

func TestCountsAddedAndRemovedLines(t *testing.T) {
	res, err := Diff(context.Background(), "a\nb\nc\n", "a\nB\nc\nd\n", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 2 || res.Removed != 1 || res.Algorithm != Myers {
		t.Errorf("got +%d -%d by %s, want +2 -1 by myers", res.Added, res.Removed, res.Algorithm)
	}
}

func TestMarksTheWordThatChanged(t *testing.T) {
	res, err := Diff(context.Background(), "func forecast(at time.Time) {\n", "func forecast(when time.Time) {\n", Options{})
	if err != nil {
		t.Fatal(err)
	}
	blk := res.Blocks[0]
	if want := []Span{{Line: 0, From: 14, To: 16}}; !slices.Equal(blk.Old, want) {
		t.Errorf("old marks %+v, want %+v", blk.Old, want)
	}
	if want := []Span{{Line: 0, From: 14, To: 18}}; !slices.Equal(blk.New, want) {
		t.Errorf("new marks %+v, want %+v", blk.New, want)
	}
}

func TestJoinsChangedWordsAcrossASpace(t *testing.T) {
	res, _ := Diff(context.Background(), "say calm seas today\n", "say rough water today\n", Options{})
	if want := []Span{{Line: 0, From: 4, To: 15}}; !slices.Equal(res.Blocks[0].New, want) {
		t.Errorf("new marks %+v, want %+v", res.Blocks[0].New, want)
	}
}

func TestMarksCountUTF16(t *testing.T) {
	// "🌊" is two UTF-16 code units, and four bytes.
	res, _ := Diff(context.Background(), "tide 🌊 high\n", "tide 🌊 low\n", Options{})
	if want := []Span{{Line: 0, From: 8, To: 11}}; !slices.Equal(res.Blocks[0].New, want) {
		t.Errorf("new marks %+v, want %+v", res.Blocks[0].New, want)
	}
}

func TestARewrittenLineIsNotMarkedWordByWord(t *testing.T) {
	res, _ := Diff(context.Background(), "the harbour master keeps a log\n", "return errors.New(\"no tide table\")\n", Options{})
	if blk := res.Blocks[0]; blk.Old != nil || blk.New != nil {
		t.Errorf("marked a rewrite: %+v", blk)
	}
}

func TestEndingsAreNotMarked(t *testing.T) {
	res, _ := Diff(context.Background(), "a = 1\r\n", "a = 2\r\n", Options{})
	if want := []Span{{Line: 0, From: 4, To: 5}}; !slices.Equal(res.Blocks[0].New, want) {
		t.Errorf("new marks %+v, want %+v", res.Blocks[0].New, want)
	}
}

const forecast = `func forecast(station string) (Tide, error) {
	readings, err := fetchReadings(station)
	if err != nil {
		return Tide{}, err
	}
	return predict(readings), nil
}
`

// helper is short, so a diff of moving it past forecast deletes and adds helper, not forecast.
const helper = `func harbour() string {
	return "Tidewater harbour master"
}
`

func TestFindsAMovedFunction(t *testing.T) {
	before := "package tides\n\n" + forecast + "\n" + helper
	after := "package tides\n\n" + helper + "\n" + forecast
	res, err := Diff(context.Background(), before, after, Options{Moves: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Moves) != 1 {
		t.Fatalf("moves %+v, want one", res.Moves)
	}
	mv := res.Moves[0]
	a, b := split(before), split(after)
	if mv.Reindented || a.span(mv.OldStart, mv.OldEnd) != helper || b.span(mv.NewStart, mv.NewEnd) != helper {
		t.Errorf("move %+v covers %q to %q, want the helper, same indentation", mv, a.span(mv.OldStart, mv.OldEnd), b.span(mv.NewStart, mv.NewEnd))
	}
	// Moved lines aren't marked word by word as well.
	for _, blk := range res.Blocks {
		for _, s := range append(slices.Clone(blk.Old), blk.New...) {
			t.Errorf("marked %+v in a block that only moved", s)
		}
	}
}

func TestAReindentedMoveIsAMove(t *testing.T) {
	indented := "\t" + strings.ReplaceAll(strings.TrimSuffix(forecast, "\n"), "\n", "\n\t") + "\n"
	before := forecast + "\nfunc outer() {\n}\n"
	after := "func outer() {\n" + indented + "}\n"
	res, _ := Diff(context.Background(), before, after, Options{Moves: true})
	moved := 0
	for _, mv := range res.Moves {
		if !mv.Reindented {
			t.Errorf("move %+v isn't marked reindented", mv)
		}
		moved += mv.OldEnd - mv.OldStart
	}
	// Indented, the function's braces read the same as other braces, and the line diff
	// matches them there; the rest of it moves, in two runs around one of them. Joining moves
	// across lines the diff matched is #947's.
	if moved < 5 {
		t.Errorf("moves %+v cover %d lines, want at least 5 of the function's 7", res.Moves, moved)
	}
}

func TestBracesAreNotAMove(t *testing.T) {
	before := "a()\n}\n}\nb()\n"
	after := "b()\n}\n}\nc()\n"
	res, _ := Diff(context.Background(), before, after, Options{Algorithm: Minimal, Moves: true})
	if len(res.Moves) != 0 {
		t.Errorf("moves %+v, want none", res.Moves)
	}
}

func TestMovesAreOnlyFoundWhenAsked(t *testing.T) {
	before := forecast + "\nfunc other() {}\n"
	after := "func other() {}\n\n" + forecast
	res, _ := Diff(context.Background(), before, after, Options{})
	if len(res.Moves) != 0 {
		t.Errorf("moves %+v, want none", res.Moves)
	}
}

func TestStopsWhenAsked(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Diff(ctx, forecast, "x\n"+forecast, Options{Moves: true}); err == nil {
		t.Error("a cancelled diff finished")
	}
}

func TestALongFileFallsBackToMyers(t *testing.T) {
	long := strings.Repeat("x\n", maxAnchoredLines+1)
	res, err := Diff(context.Background(), long, long+"y\n", Options{Algorithm: Histogram})
	if err != nil || res.Algorithm != Myers {
		t.Errorf("algorithm %s, %v; want myers", res.Algorithm, err)
	}
}

func TestCacheReturnsTheSameResult(t *testing.T) {
	c := NewCache(2)
	first, _ := c.Diff(context.Background(), "a\n", "b\n", Options{})
	again, _ := c.Diff(context.Background(), "a\n", "b\n", Options{})
	if len(first.Blocks) != 1 || len(again.Blocks) != 1 {
		t.Fatalf("got %+v and %+v", first, again)
	}
	_, _ = c.Diff(context.Background(), "c\n", "d\n", Options{})
	_, _ = c.Diff(context.Background(), "e\n", "f\n", Options{})
	if c.order.Len() != 2 || len(c.entries) != 2 {
		t.Errorf("cache holds %d (%d), want 2", c.order.Len(), len(c.entries))
	}
}

// place is a block's lines alone, which compare.
type place struct{ a0, a1, b0, b1 int }

var hunkHeader = regexp.MustCompile(`(?m)^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// gitBlocks runs git diff -U0 with the given algorithm and reads its hunks as blocks.
func gitBlocks(t *testing.T, before, after, algorithm string) []place {
	t.Helper()
	dir := t.TempDir()
	pa, pb := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.WriteFile(pa, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pb, []byte(after), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-c", "core.autocrlf=false", "diff", "--no-index", "--no-color", "-U0", "--diff-algorithm="+algorithm, pa, pb)
	out, err := cmd.Output()
	if err != nil && cmd.ProcessState.ExitCode() != 1 {
		t.Fatalf("git diff: %v", err)
	}
	var blocks []place
	for _, m := range hunkHeader.FindAllStringSubmatch(string(out), -1) {
		num := func(s string, dflt int) int {
			if s == "" {
				return dflt
			}
			n, _ := strconv.Atoi(s)
			return n
		}
		as, ac, bs, bc := num(m[1], 0), num(m[2], 1), num(m[3], 0), num(m[4], 1)
		// git numbers from 1 and names the line before an empty side.
		if ac > 0 {
			as--
		}
		if bc > 0 {
			bs--
		}
		blocks = append(blocks, place{as, as + ac, bs, bs + bc})
	}
	return blocks
}

func TestAnchoredAlgorithmsAgreeWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	cases := map[string][2]string{
		// The case patience diff is known for: Myers matches the braces and blank lines of
		// two different functions.
		"functions": {
			"#include <stdio.h>\n\n// Frobs foo heartily\nint frobnitz(int foo)\n{\n    int i;\n    for(i = 0; i < 10; i++)\n    {\n        printf(\"Your answer is: \");\n        printf(\"%d\\n\", foo);\n    }\n}\n\nint fact(int n)\n{\n    if(n > 1)\n    {\n        return fact(n-1) * n;\n    }\n    return 1;\n}\n\nint main(int argc, char **argv)\n{\n    frobnitz(fact(10));\n}\n",
			"#include <stdio.h>\n\nint fib(int n)\n{\n    if(n > 2)\n    {\n        return fib(n-1) + fib(n-2);\n    }\n    return 1;\n}\n\n// Frobs foo heartily\nint frobnitz(int foo)\n{\n    int i;\n    for(i = 0; i < 10; i++)\n    {\n        printf(\"%d\\n\", foo);\n    }\n}\n\nint main(int argc, char **argv)\n{\n    frobnitz(fib(10));\n}\n",
		},
		// git doesn't trim the common start and end before anchoring, so here it keeps
		// harbour() and has forecast() leave and come back.
		"swap": {
			"package tides\n\nfunc harbour() string {\n\treturn \"Tidewater harbour master\"\n}\n\nfunc forecast(at time.Time) Tide {\n\treadings := fetch(at)\n\treturn predict(readings)\n}\n\n// end\n",
			"package tides\n\nfunc forecast(when time.Time) Tide {\n\treadings := fetch(when)\n\tlog.Printf(\"forecast for %v\", when)\n\treturn predict(readings)\n}\n\nfunc harbour() string {\n\treturn \"Tidewater harbour master\"\n}\n\n// end\n",
		},
		"reorder": {"a\nb\nc\nd\ne\nf\n", "d\ne\nf\na\nb\nc\n"},
		"repeats": {"x\ny\nx\ny\nx\nz\n", "x\nz\nx\ny\nx\ny\n"},
		"append":  {"one\ntwo\n", "one\ntwo\nthree\n"},
		"empty":   {"", "a\nb\n"},
		"gone":    {"a\nb\n", ""},
	}
	for name, c := range cases {
		for _, alg := range []string{Patience, Histogram} {
			res, err := Diff(context.Background(), c[0], c[1], Options{Algorithm: alg})
			if err != nil {
				t.Fatal(err)
			}
			want := gitBlocks(t, c[0], c[1], alg)
			got := make([]place, len(res.Blocks))
			for i, blk := range res.Blocks {
				got[i] = place{blk.OldStart, blk.OldEnd, blk.NewStart, blk.NewEnd}
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s by %s:\n got %+v\nwant %+v", name, alg, got, want)
			}
		}
	}
}

// codeText and codeMutate make code-like text with the repeats real code has: braces, blank
// lines, returns.
func codeText(r *rand.Rand, n int) string {
	vocab := []string{"}", "", "\treturn nil", "\tif err != nil {", "\t\treturn err", "\t}", "func f() error {", "func g() error {", "\tx := 1", "\ty := 2", "// note", "\tfor i := range xs {", "\t\tsum += i", "\tlog.Println(x)", "package p", "import \"fmt\""}
	var b strings.Builder
	for range n {
		b.WriteString(vocab[r.IntN(len(vocab))])
		b.WriteString("\n")
	}
	return b.String()
}

func codeMutate(r *rand.Rand, s string) string {
	lines := strings.SplitAfter(s, "\n")
	lines = lines[:len(lines)-1]
	for range 1 + r.IntN(6) {
		at := r.IntN(len(lines) + 1)
		switch r.IntN(4) {
		case 0:
			n := 1 + r.IntN(4)
			block := strings.SplitAfter(codeText(r, n), "\n")
			lines = slices.Insert(lines, at, block[:n]...)
		case 1:
			if at < len(lines) {
				lines = slices.Delete(lines, at, min(len(lines), at+1+r.IntN(3)))
			}
		case 2:
			if at < len(lines) {
				lines[at] = "\tchanged()\n"
			}
		case 3:
			// Move a run of lines.
			if len(lines) > 6 {
				from := r.IntN(len(lines) - 3)
				run := slices.Clone(lines[from : from+3])
				lines = slices.Delete(lines, from, from+3)
				to := r.IntN(len(lines) + 1)
				lines = slices.Insert(lines, to, run...)
			}
		}
	}
	return strings.Join(lines, "")
}

// TestEveryAlgorithmAgreesWithGit compares each algorithm with git on random edits of
// code-like text. Histogram, the one people most often set, gives exactly git's hunks.
// Myers and Minimal here are znkr.io/diff's, not git's, and patience hands stretches with no
// unique line to Myers, so those can break ties differently: they must change as many lines
// as git does, but may pick other ones.
func TestEveryAlgorithmAgreesWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	t.Parallel()
	size := func(ps []place) int {
		n := 0
		for _, p := range ps {
			n += p.a1 - p.a0 + p.b1 - p.b0
		}
		return n
	}
	r := rand.New(rand.NewPCG(1, 944))
	for _, alg := range Algorithms {
		for c := range 25 {
			before := codeText(r, 10+r.IntN(40))
			after := codeMutate(r, before)
			res, err := Diff(context.Background(), before, after, Options{Algorithm: alg})
			if err != nil {
				t.Fatal(err)
			}
			got := make([]place, len(res.Blocks))
			for i, blk := range res.Blocks {
				got[i] = place{blk.OldStart, blk.OldEnd, blk.NewStart, blk.NewEnd}
			}
			want := gitBlocks(t, before, after, alg)
			if alg == Histogram && !slices.Equal(got, want) || size(got) != size(want) {
				t.Errorf("%s, case %d:\n got %v\nwant %v\nbefore:\n%s\nafter:\n%s", alg, c, got, want, before, after)
			}
		}
	}
}
