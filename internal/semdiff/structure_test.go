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

// Lines heavy enough to be a move by themselves still say what they went into (#971).
func TestLongLinesWrappedInATryMoveIntoIt(t *testing.T) {
	before := "async function load() {\n" +
		"  const res = await fetch('/api/checks')\n" +
		"  const checks = await res.json()\n" +
		"  results.replaceChildren(...checks.map(checkItem))\n" +
		"}\n"
	after := "async function load() {\n" +
		"  try {\n" +
		"    const res = await fetch('/api/checks')\n" +
		"    const checks = await res.json()\n" +
		"    results.replaceChildren(...checks.map(checkItem))\n" +
		"  } catch {\n" +
		"    results.replaceChildren()\n" +
		"  }\n" +
		"}\n"
	res := readDiff(t, "javascript", before, after)
	if len(res.Moves) != 1 {
		t.Fatalf("moves %+v, want one", res.Moves)
	}
	if mv := res.Moves[0]; mv.OldStart != 1 || mv.OldEnd != 4 || mv.NewStart != 2 || mv.NewEnd != 5 || !mv.Reindented || mv.Into != "try statement" {
		t.Fatalf("move %+v", mv)
	}
}

// Wrapped with a line edited on the way, it's one move that changed, into the try.
func TestWrappedAndChangedMovesIntoIt(t *testing.T) {
	before := "async function load() {\n" +
		"  const res = await fetch('/api/checks')\n" +
		"  const checks = await res.json()\n" +
		"  checks.sort((a, b) => b.ok - a.ok)\n" +
		"  results.replaceChildren(...checks.map(checkItem))\n" +
		"  document.querySelector('#updated').textContent = age(checks[0].at)\n" +
		"}\n"
	after := "async function load() {\n" +
		"  try {\n" +
		"    const res = await fetch('/api/checks')\n" +
		"    const checks = await res.json()\n" +
		"    checks.sort((a, b) => a.ok - b.ok)\n" +
		"    results.replaceChildren(...checks.map(checkItem))\n" +
		"    document.querySelector('#updated').textContent = age(checks[0].at)\n" +
		"  } catch {\n" +
		"    results.replaceChildren()\n" +
		"  }\n" +
		"}\n"
	res := readDiff(t, "javascript", before, after)
	if len(res.Moves) != 1 {
		t.Fatalf("moves %+v, want one", res.Moves)
	}
	mv := res.Moves[0]
	if mv.OldStart != 1 || mv.OldEnd != 6 || mv.NewStart != 2 || mv.NewEnd != 7 || !mv.Changed || mv.Into != "try statement" {
		t.Fatalf("move %+v", mv)
	}
	// Its own diff: the sort's line changed; the rest was indented, which is formatting.
	var edited []Block
	for _, e := range mv.Edits {
		if e.Kind != KindFormat {
			edited = append(edited, e)
		}
	}
	if len(edited) != 1 || edited[0].OldStart != 2 || edited[0].NewStart != 2 {
		t.Fatalf("edits %+v", mv.Edits)
	}
}

func TestWrappedInEachLanguage(t *testing.T) {
	for _, c := range []struct{ id, before, after, into string }{
		{"bash", "mkdir -p logs\npython3 check_stations.py >> logs/nightly.log\necho checked every station >> logs/nightly.log\n",
			"mkdir -p logs\nif curl -fsS https://feed.example/health; then\n  python3 check_stations.py >> logs/nightly.log\n  echo checked every station >> logs/nightly.log\nfi\n", "if statement"},
		{"python", "def main(stations):\n    for station in stations:\n        reading = latest_reading(station)\n        record(station, problems(reading))\n",
			"def main(stations):\n    for station in stations:\n        try:\n            reading = latest_reading(station)\n            record(station, problems(reading))\n        except OSError:\n            record(station, 'no answer')\n", "try statement"},
	} {
		res := readDiff(t, c.id, c.before, c.after)
		if len(res.Moves) != 1 || res.Moves[0].Into != c.into {
			t.Errorf("%s: moves %+v, want one into %s", c.id, res.Moves, c.into)
		}
	}
}

// A move that took its lines out of structure doesn't say it moved into anything, and nor
// does one from elsewhere.
func TestMovesThatWerentWrappedSayNothingOfStructure(t *testing.T) {
	for _, c := range []struct{ name, before, after string }{
		{"unwrapped",
			"async function load() {\n  try {\n    const res = await fetch('/api/checks')\n    const checks = await res.json()\n  } catch {\n    return\n  }\n}\n",
			"async function load() {\n  const res = await fetch('/api/checks')\n  const checks = await res.json()\n}\n"},
		{"from elsewhere",
			"function a() {\n  const reading = latestReading(station)\n  record(station, problems(reading))\n}\n\nfunction b() {\n  return 1\n}\n",
			"function a() {\n}\n\nfunction b() {\n  if (ready) {\n    const reading = latestReading(station)\n    record(station, problems(reading))\n  }\n  return 1\n}\n"},
	} {
		res := readDiff(t, "javascript", c.before, c.after)
		for _, mv := range res.Moves {
			if mv.Into != "" {
				t.Errorf("%s: move %+v says it went into %s", c.name, mv, mv.Into)
			}
		}
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
	ta, tb := buildTree(v, r.nodes, nil), buildTree(v, r.nodes, nil)
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
