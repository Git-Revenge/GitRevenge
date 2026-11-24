package semdiff

import (
	"context"
	"strings"
	"testing"
)

func TestReorderedArgumentsMove(t *testing.T) {
	before := "package p\n\nfunc f() {\n\tsend(station, reading)\n}\n"
	after := "package p\n\nfunc f() {\n\tsend(reading, station)\n}\n"
	res := readDiff(t, "go", before, after)
	old, nw := oldMarks(before, res), newMarks(after, res)
	// The diff keeps one argument where it was; the other moved across it.
	if !strings.Contains(strings.Join(old, "|"), "move:") || !strings.Contains(strings.Join(nw, "|"), "move:") {
		t.Fatalf("old %q, new %q: want a move on each side", old, nw)
	}
	for _, m := range append(old, nw...) {
		if strings.HasPrefix(m, "change:") {
			t.Errorf("marked %q as a change", m)
		}
	}
}

func TestCodeWrappedInAnIfMovedIntoIt(t *testing.T) {
	before := "package p\n\nfunc f() {\n\tload()\n\tsave()\n}\n"
	after := "package p\n\nfunc f() {\n\tif ready() {\n\t\tload()\n\t\tsave()\n\t}\n}\n"
	res := readDiff(t, "go", before, after)
	var nested []Move
	for _, mv := range res.Moves {
		if mv.Into != "" {
			nested = append(nested, mv)
		}
	}
	if len(nested) != 1 {
		t.Fatalf("moves %+v, want one into new structure", res.Moves)
	}
	mv := nested[0]
	if mv.OldStart != 3 || mv.OldEnd != 5 || mv.NewStart != 4 || mv.NewEnd != 6 || !mv.Reindented || mv.Into != "if statement" {
		t.Fatalf("move %+v", mv)
	}
}

func TestTreesAreOnlyMatchedWhenMovesAreWanted(t *testing.T) {
	before := "package p\n\nfunc f() {\n\tload()\n}\n"
	after := "package p\n\nfunc f() {\n\tif ready() {\n\t\tload()\n\t}\n}\n"
	res, err := Diff(context.Background(), before, after, Options{Language: "go", ReadTime: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Moves) != 0 {
		t.Fatalf("moves %+v without asking", res.Moves)
	}
}

func TestMatchingIdenticalTreesMatchesEveryToken(t *testing.T) {
	src := "package p\n\nfunc f(a, b int) int {\n\tif a > b {\n\t\treturn a\n\t}\n\treturn b\n}\n"
	l := byID("go")
	l.load()
	r := readVersion(context.Background(), l, src, -1, true)
	v := newVersion(split(src), r.toks, nil)
	ta, tb := buildTree(v, r.nodes), buildTree(v, r.nodes)
	pa, _, ok := matchTrees(context.Background(), ta, tb, len(r.toks), len(r.toks))
	if !ok {
		t.Fatal("ran out of budget")
	}
	for i, p := range pa {
		if !r.toks[i].ignorable && r.toks[i].class != clsSpace && p != int32(i) {
			t.Fatalf("token %d (%q) matched %d", i, src[r.toks[i].from:r.toks[i].to], p)
		}
	}
}
