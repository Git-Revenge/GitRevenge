package git

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// grepAll runs a search to its end and returns every line it found.
func grepAll(t *testing.T, repo *Repo, o GrepOptions, limit int) ([]GrepLine, GrepSummary) {
	t.Helper()
	var got []GrepLine
	sum, err := repo.Grep(context.Background(), o, limit, func(l GrepLine) { got = append(got, l) })
	if err != nil {
		t.Fatalf("Grep(%+v): %v", o, err)
	}
	return got, sum
}

// windRepo is a repository where km/h turns up in a few places, as it does in the demo.
func windRepo(t *testing.T) (*gittest.Repo, *Repo) {
	t.Helper()
	fx := gittest.New(t)
	fx.WriteFile("a b.txt", "speed in km/h, or KM/H\nno match\r\nlater km/h\r\n")
	fx.WriteFile("dir/ü.txt", "café 🌊 km/h\n")
	fx.WriteFile("bin.dat", "bin\x00km/h\n")
	fx.Commit("one")
	return fx, OpenRepo(newTestRunner(t), fx.Dir)
}

func TestGrepFindsEveryLineAndMarksEveryMatch(t *testing.T) {
	t.Parallel()
	_, repo := windRepo(t)
	got, sum := grepAll(t, repo, GrepOptions{Text: "km/h"}, 0)
	want := []GrepLine{
		// Case doesn't count unless asked to; a carriage return isn't part of the line.
		{File: "a b.txt", Line: 1, Text: "speed in km/h, or KM/H", Matches: []GrepSpan{{9, 13}, {18, 22}}},
		{File: "a b.txt", Line: 3, Text: "later km/h", Matches: []GrepSpan{{6, 10}}},
		// UTF-16 units, as the window counts: the wave is two.
		{File: "dir/ü.txt", Line: 1, Text: "café 🌊 km/h", Matches: []GrepSpan{{8, 12}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines =\n%+v\nwant\n%+v", got, want)
	}
	if sum != (GrepSummary{Lines: 3, Files: 2}) {
		t.Fatalf("summary = %+v; the binary file is skipped", sum)
	}
}

func TestGrepOptions(t *testing.T) {
	t.Parallel()
	fx, repo := windRepo(t)
	fx.WriteFile("words.txt", "km kmh xkm km-x\nkmh only\nwind_kmh and wind_knots\n")
	fx.Commit("words")
	fx.WriteFile("old.txt", "km/h, once\n")
	fx.Commit("old")
	fx.WriteFile("old.txt", "knots now\n")
	fx.Commit("knots")
	fx.WriteFile("new.txt", "untracked km/h\n")

	files := func(lines []GrepLine) []string {
		var out []string
		for _, l := range lines {
			out = append(out, l.File+":"+l.Text)
		}
		return out
	}

	got, _ := grepAll(t, repo, GrepOptions{Text: "KM/H", MatchCase: true}, 0)
	if len(got) != 1 || !reflect.DeepEqual(got[0].Matches, []GrepSpan{{18, 22}}) {
		t.Errorf("match case = %+v", got)
	}

	got, _ = grepAll(t, repo, GrepOptions{Text: "km", WholeWord: true, Paths: []string{"words.txt"}}, 0)
	if len(got) != 1 || !reflect.DeepEqual(got[0].Matches, []GrepSpan{{0, 2}, {11, 13}}) {
		t.Errorf("whole word = %+v; km alone, and before a dash, but never in kmh or xkm", got)
	}

	got, _ = grepAll(t, repo, GrepOptions{Text: "wind_(kmh|knots)", Regex: true}, 0)
	if len(got) != 1 || !reflect.DeepEqual(got[0].Matches, []GrepSpan{{0, 8}, {13, 23}}) {
		t.Errorf("regex = %+v", got)
	}

	got, _ = grepAll(t, repo, GrepOptions{Text: "km/h"}, 0)
	if strings.Contains(strings.Join(files(got), "\n"), "new.txt") {
		t.Errorf("an untracked file was searched unasked: %v", files(got))
	}
	got, _ = grepAll(t, repo, GrepOptions{Text: "km/h", Untracked: true}, 0)
	if !strings.Contains(strings.Join(files(got), "\n"), "new.txt:untracked km/h") {
		t.Errorf("untracked = %v", files(got))
	}

	got, _ = grepAll(t, repo, GrepOptions{Text: "km/h", Rev: "HEAD~1"}, 0)
	if !strings.Contains(strings.Join(files(got), "\n"), "old.txt:km/h, once") {
		t.Errorf("at HEAD~1 = %v; the file is named without the revision", files(got))
	}
	got, _ = grepAll(t, repo, GrepOptions{Text: "km/h", Rev: "HEAD", Untracked: true}, 0)
	if strings.Contains(strings.Join(files(got), "\n"), "new.txt") || strings.Contains(strings.Join(files(got), "\n"), "old.txt") {
		t.Errorf("at HEAD = %v; a commit has no untracked files, and old.txt says knots", files(got))
	}

	got, _ = grepAll(t, repo, GrepOptions{Text: "km/h", Paths: []string{" dir/ ", ""}}, 0)
	if !reflect.DeepEqual(files(got), []string{"dir/ü.txt:café 🌊 km/h"}) {
		t.Errorf("in dir/ = %v", files(got))
	}
	got, _ = grepAll(t, repo, GrepOptions{Text: "km/h", Paths: []string{"!*.txt"}}, 0)
	if len(got) != 0 {
		t.Errorf("leaving out *.txt = %v", files(got))
	}
	got, _ = grepAll(t, repo, GrepOptions{Text: "km/h", Paths: []string{"*.txt", "!a b.txt"}}, 0)
	if !reflect.DeepEqual(files(got), []string{"dir/ü.txt:café 🌊 km/h"}) {
		t.Errorf("*.txt but a b.txt = %v", files(got))
	}
}

func TestGrepStopsAtTheLimitAndSaysSo(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.WriteFile("many.txt", strings.Repeat("a km/h line\n", 25))
	fx.WriteFile("more.txt", "km/h\n")
	fx.Commit("many")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	got, sum := grepAll(t, repo, GrepOptions{Text: "km/h"}, 10)
	if len(got) != 10 || sum != (GrepSummary{Lines: 10, Files: 1, Truncated: true}) {
		t.Fatalf("%d lines, summary %+v", len(got), sum)
	}
	got, sum = grepAll(t, repo, GrepOptions{Text: "km/h"}, 26)
	if len(got) != 26 || sum.Truncated {
		t.Fatalf("with room for all: %d lines, summary %+v", len(got), sum)
	}
}

func TestGrepCutsALongLineAroundItsFirstMatch(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.WriteFile("min.js", strings.Repeat("a", 1000)+"km/h"+strings.Repeat("é", 1000)+"\n")
	fx.Commit("minified")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	got, _ := grepAll(t, repo, GrepOptions{Text: "km/h"}, 0)
	if len(got) != 1 {
		t.Fatalf("lines = %+v", got)
	}
	l := got[0]
	if !l.CutStart || !l.CutEnd || len(l.Text) > grepCutAt+4 || len(l.Matches) != 1 {
		t.Fatalf("cut line = %+v", l)
	}
	if got := string([]rune(l.Text)[l.Matches[0].From:l.Matches[0].To]); got != "km/h" {
		t.Fatalf("the mark covers %q", got)
	}
	if !strings.HasPrefix(l.Text, "aaa") || !strings.HasSuffix(l.Text, "é") {
		t.Fatalf("cut mid-character, or in the wrong place: %q…%q", l.Text[:3], l.Text[len(l.Text)-3:])
	}
}

func TestGrepReadsLinesThatArentUTF8(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.WriteFile("latin1.txt", "caf\xe9 km/h\n")
	fx.Commit("latin-1")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	got, _ := grepAll(t, repo, GrepOptions{Text: "km/h"}, 0)
	want := []GrepLine{{File: "latin1.txt", Line: 1, Text: "caf� km/h", Matches: []GrepSpan{{5, 9}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %+v", got)
	}
}

func TestGrepNothingFoundIsNoError(t *testing.T) {
	t.Parallel()
	_, repo := windRepo(t)
	got, sum := grepAll(t, repo, GrepOptions{Text: "nothing like it"}, 0)
	if len(got) != 0 || sum != (GrepSummary{}) {
		t.Fatalf("lines %+v, summary %+v", got, sum)
	}
}

func TestGrepSaysWhatsWrong(t *testing.T) {
	t.Parallel()
	_, repo := windRepo(t)
	ctx := context.Background()
	none := func(GrepLine) {}
	for text, reason := range map[string]string{
		"kmalloc_array(": "a ( is never closed",
		"[abc":           "a [ is never closed",
	} {
		_, err := repo.Grep(ctx, GrepOptions{Text: text, Regex: true}, 0, none)
		var pe *PatternError
		if !errors.As(err, &pe) || pe.Reason != reason {
			t.Errorf("%q: err = %v, want %q", text, err, reason)
		}
		// As written, it's no regular expression at all.
		if _, err := repo.Grep(ctx, GrepOptions{Text: text}, 0, none); err != nil {
			t.Errorf("%q as written: %v", text, err)
		}
	}
	for _, rev := range []string{"no-such-branch", "-x"} {
		if _, err := repo.Grep(ctx, GrepOptions{Text: "km/h", Rev: rev}, 0, none); !errors.Is(err, ErrUnknownRevision) {
			t.Errorf("rev %q: err = %v", rev, err)
		}
	}
	for _, text := range []string{"", "two\nlines"} {
		if _, err := repo.Grep(ctx, GrepOptions{Text: text}, 0, none); !errors.Is(err, ErrFindText) {
			t.Errorf("text %q: err = %v", text, err)
		}
	}
}

func TestGrepCountCountsLinesAndFiles(t *testing.T) {
	t.Parallel()
	_, repo := windRepo(t)
	sum, err := repo.GrepCount(context.Background(), GrepOptions{Text: "km/h"})
	if err != nil || sum != (GrepSummary{Lines: 3, Files: 2}) {
		t.Fatalf("count = %+v, %v", sum, err)
	}
	sum, err = repo.GrepCount(context.Background(), GrepOptions{Text: "nothing like it"})
	if err != nil || sum != (GrepSummary{}) {
		t.Fatalf("count of nothing = %+v, %v", sum, err)
	}
}

func TestGrepStopsWhenCancelled(t *testing.T) {
	t.Parallel()
	_, repo := windRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.Grep(ctx, GrepOptions{Text: "km/h"}, 0, func(GrepLine) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestGrepInASparseCheckoutSearchesWhatsCheckedOut(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.WriteFile("in/x.txt", "km/h\n")
	fx.WriteFile("out/y.txt", "km/h\n")
	fx.Commit("two folders")
	fx.Git("sparse-checkout", "set", "in")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	names := func(lines []GrepLine) []string {
		var out []string
		for _, l := range lines {
			out = append(out, l.File)
		}
		return out
	}
	got, _ := grepAll(t, repo, GrepOptions{Text: "km/h"}, 0)
	if !reflect.DeepEqual(names(got), []string{"in/x.txt"}) {
		t.Errorf("working tree = %v; files outside the checkout aren't there to search", names(got))
	}
	got, _ = grepAll(t, repo, GrepOptions{Text: "km/h", Rev: "HEAD"}, 0)
	if !reflect.DeepEqual(names(got), []string{"in/x.txt", "out/y.txt"}) {
		t.Errorf("HEAD = %v; a commit has every file", names(got))
	}
}

// partialClone is a clone of a two-commit repository that downloaded only the files HEAD
// checks out: old.txt as it was at HEAD~1 was never fetched.
func partialClone(t *testing.T) string {
	t.Helper()
	src := gittest.New(t)
	src.CommitFile("old.txt", "km/h once\n", "one")
	src.CommitFile("old.txt", "knots now\n", "two")
	src.Git("config", "uploadpack.allowFilter", "true")
	dest := filepath.Join(t.TempDir(), "partial")
	src.Git("clone", "-q", "--filter=blob:none", "file://"+filepath.ToSlash(src.Dir), dest)
	return dest
}

// stillMissing reports whether old.txt at HEAD~1 is still not in the clone.
func stillMissing(t *testing.T, repo *Repo) bool {
	t.Helper()
	_, err := repo.readCmd(context.Background(), Cmd{Args: []string{"cat-file", "-e", "HEAD~1:old.txt"}, Env: []string{"GIT_NO_LAZY_FETCH=1"}})
	return err != nil
}

func TestGrepInAPartialCloneDownloadsNothing(t *testing.T) {
	t.Parallel()
	dest := partialClone(t)
	runner := newTestRunner(t)
	if runner.version().Less(noLazyFetch) {
		t.Skip("git older than 2.44: TestGrepWithAnOlderGitRefusesToDownload covers it")
	}
	repo := OpenRepo(runner, dest)
	if _, err := repo.Grep(context.Background(), GrepOptions{Text: "km/h", Rev: "HEAD~1"}, 0, func(GrepLine) {}); !errors.Is(err, ErrNotDownloaded) {
		t.Fatalf("err = %v", err)
	}
	if !stillMissing(t, repo) {
		t.Fatal("the search downloaded old.txt")
	}
	// The working tree's files are there, and searched as anywhere else.
	got, _ := grepAll(t, repo, GrepOptions{Text: "knots"}, 0)
	if len(got) != 1 || got[0].File != "old.txt" {
		t.Fatalf("working tree = %+v", got)
	}
}

func TestGrepWithAnOlderGitRefusesToDownload(t *testing.T) {
	t.Parallel()
	dest := partialClone(t)
	runner := newTestRunner(t)
	// A git that can't be told not to fetch.
	runner.verOnce.Do(func() { runner.ver = Version{2, 43, 0} })
	repo := OpenRepo(runner, dest)
	if _, err := repo.Grep(context.Background(), GrepOptions{Text: "km/h", Rev: "HEAD~1"}, 0, func(GrepLine) {}); !errors.Is(err, ErrLazyFetch) {
		t.Fatalf("err = %v", err)
	}
	if !stillMissing(t, repo) {
		t.Fatal("the search downloaded old.txt")
	}
	if _, err := repo.GrepCount(context.Background(), GrepOptions{Text: "knots"}); err != nil {
		t.Fatalf("the working tree can still be searched: %v", err)
	}
}
