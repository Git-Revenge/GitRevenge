package semdiff

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the corpus's expected results")

// TestCorpus reads a fixed corpus of file pairs as code, one or more for every language, and
// compares the results with the ones recorded beside them. It guards gotreesitter upgrades: a
// new version that reads any of these files differently shows up here as a changed result.
// Run with -update to record new results, then read the difference before committing it.
//
// The pairs are named name.before.ext and name.after.ext, and the result is name.txt. Some
// are this repository's own files at two commits; the rest are written to try each
// language's rules.
func TestCorpus(t *testing.T) {
	befores, err := filepath.Glob(filepath.Join("testdata", "corpus", "*.before.*"))
	if err != nil || len(befores) == 0 {
		t.Fatalf("no corpus: %v", err)
	}
	seen := map[string]bool{}
	for _, before := range befores {
		name := strings.SplitN(filepath.Base(before), ".", 2)[0]
		after := strings.Replace(before, ".before.", ".after.", 1)
		// The language comes from the pair's name without .after: go.after.mod is go.mod.
		id := LanguageOf(strings.Replace(filepath.Base(after), ".after.", ".", 1), "", "")
		if id == "" {
			t.Fatalf("%s isn't in a language read as code", after)
		}
		seen[id] = true
		t.Run(name, func(t *testing.T) {
			a, err := os.ReadFile(before)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(after)
			if err != nil {
				t.Fatal(err)
			}
			// No time limit, so a busy machine can't change the result.
			res, err := Diff(context.Background(), string(a), string(b), Options{Algorithm: Histogram, Moves: true, Language: id, ReadTime: -1})
			if err != nil {
				t.Fatal(err)
			}
			checkBlocks(t, string(a), string(b), res.Blocks)
			got := render(string(a), string(b), res)
			want := filepath.Join("testdata", "corpus", name+".txt")
			if *update {
				if err := os.WriteFile(want, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			w, err := os.ReadFile(want)
			if err != nil {
				t.Fatalf("no recorded result; run with -update: %v", err)
			}
			if string(w) != got {
				t.Errorf("%s reads differently now. Got:\n%s", name, got)
			}
		})
	}
	for _, l := range languages {
		if !seen[l.id] && !l.hidden && l.id != "tsx" && l.id != "javascript" {
			t.Errorf("the corpus has no %s pair", l.name)
		}
	}
}

// render writes a result as text: each block with its kind and note, and its lines with the
// marks in brackets, [ ] for a change, {f: } for formatting and {r: } for a rename.
func render(before, after string, res Result) string {
	a, b := split(before), split(after)
	var out strings.Builder
	fmt.Fprintf(&out, "engine %s, language %s", res.Engine, res.Language)
	if res.Fallback != "" {
		fmt.Fprintf(&out, ", fallback %s", res.Fallback)
	}
	if res.Unparsed {
		out.WriteString(", part unparsed")
	}
	out.WriteString("\n")
	for _, r := range res.Renames {
		fmt.Fprintf(&out, "rename %s -> %s, %d places\n", r.From, r.To, r.Count)
	}
	for _, m := range res.Moves {
		fmt.Fprintf(&out, "move %d-%d -> %d-%d\n", m.OldStart+1, m.OldEnd, m.NewStart+1, m.NewEnd)
	}
	for _, blk := range res.Blocks {
		kind := blk.Kind
		if kind == KindChange {
			kind = "change"
		}
		fmt.Fprintf(&out, "\n@@ -%d,%d +%d,%d @@ %s", blk.OldStart+1, blk.OldEnd-blk.OldStart, blk.NewStart+1, blk.NewEnd-blk.NewStart, kind)
		if blk.Note != "" {
			fmt.Fprintf(&out, ": %s", blk.Note)
		}
		out.WriteString("\n")
		for i := blk.OldStart; i < blk.OldEnd; i++ {
			fmt.Fprintf(&out, "-%s\n", show(a.body(i), i, blk.Old))
		}
		for j := blk.NewStart; j < blk.NewEnd; j++ {
			fmt.Fprintf(&out, "+%s\n", show(b.body(j), j, blk.New))
		}
	}
	return out.String()
}

// show writes a line with its marks in brackets.
func show(body string, line int, spans []Span) string {
	u := []rune(body)
	// Spans count UTF-16 units; walk the runes counting them.
	var out strings.Builder
	at := 0
	for i := 0; i <= len(u); i++ {
		for _, s := range spans {
			if s.Line == line && s.To == at && s.To > s.From {
				out.WriteString("]")
			}
		}
		for _, s := range spans {
			if s.Line == line && s.From == at && s.To > s.From {
				switch s.Kind {
				case KindFormat:
					out.WriteString("{f:")
				case KindRename:
					out.WriteString("{r:")
				default:
					out.WriteString("[")
				}
			}
		}
		if i < len(u) {
			out.WriteRune(u[i])
			at++
			if u[i] >= 0x10000 {
				at++
			}
		}
	}
	return out.String()
}
