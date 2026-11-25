package perf

import (
	"context"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// TestFindInFiles times Find in files (#979) on the bench repository: the first line, the
// whole search, and a search with too much to show, which has to stop at its limit rather
// than read on through every file.
func TestFindInFiles(t *testing.T) {
	r := benchRepo(t)
	ctx := context.Background()
	for _, c := range []struct {
		name string
		o    git.GrepOptions
	}{
		{"working tree", git.GrepOptions{Text: "kmalloc_array("}},
		{"matching case", git.GrepOptions{Text: "kmalloc_array(", MatchCase: true}},
		{"regex", git.GrepOptions{Text: `kmalloc_array\(.*GFP_ATOMIC`, Regex: true}},
		{"a common word", git.GrepOptions{Text: "TODO", MatchCase: true}},
		{"everywhere", git.GrepOptions{Text: "the", MatchCase: true}},
		{"at HEAD~1000", git.GrepOptions{Text: "kmalloc_array("}},
	} {
		if c.name == "at HEAD~1000" {
			c.o.Rev = "HEAD~1000"
		}
		start := time.Now()
		var first time.Duration
		sum, err := r.Grep(ctx, c.o, git.GrepLimit, func(git.GrepLine) {
			if first == 0 {
				first = time.Since(start)
			}
		})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		all := time.Since(start)
		t.Logf("%-14s first %6s  all %7s  %5d lines in %4d files  truncated %v", c.name,
			first.Round(time.Millisecond), all.Round(time.Millisecond), sum.Lines, sum.Files, sum.Truncated)
		if c.name == "everywhere" && !sum.Truncated {
			t.Errorf("a search for %q wasn't cut at %d lines", c.o.Text, git.GrepLimit)
		}
	}
}
