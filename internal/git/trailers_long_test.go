package git

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// inTime runs fn, and panics if it takes longer than limit: the test's own cleanup would
// otherwise wait behind it.
func inTime(limit time.Duration, fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		panic(fmt.Sprintf("still reading after %s", limit))
	}
}

// A long message is read in time that grows with its length, not with its square (#893):
// a trailer carried on over many lines, a note carried on over many, and a message of many
// short trailer paragraphs. Each of these took minutes when every line or paragraph copied
// everything before it. The trailer index reads every commit in the background, and closing
// a tab waits for the commit being read, so one such commit held the app up.
//
// It takes a quarter of a second here, and minutes the old way. It isn't run in parallel,
// and is given a minute: on CI's macOS runner, under the race detector and beside the
// package's other tests, it once took more than 20 seconds.
func TestLongMessagesAreReadInLinearTime(t *testing.T) {
	const lines = 200000

	var carried strings.Builder
	carried.WriteString("subject\n\nbody\n\nKey: v\n")
	for range lines {
		carried.WriteString("  more\n")
	}
	var note strings.Builder
	note.WriteString("subject\n\nbody\n\nSigned-off-by: Ada <ada@example.com>\n[a note]\n")
	for range lines {
		note.WriteString("  more\n")
	}
	var paras strings.Builder
	paras.WriteString("subject\n\nbody\n")
	for i := range lines / 2 {
		fmt.Fprintf(&paras, "\nK%d: v\n", i)
	}

	inTime(time.Minute, func() {
		got := MessageTrailers(carried.String())
		if want := "v" + strings.Repeat(" more", lines); len(got) != 1 || got[0].Value != want {
			t.Errorf("a trailer carried on over %d lines: %d trailers", lines, len(got))
		}
		// So many lines that aren't trailers make the paragraph not one of trailers.
		if got := MessageTrailers(note.String()); len(got) != 0 {
			t.Errorf("a note carried on over %d lines: %d trailers, want none", lines, len(got))
		}
		got = MessageTrailers(paras.String())
		if len(got) != lines/2 || got[0].Key != "K0" || got[len(got)-1].Key != fmt.Sprintf("K%d", lines/2-1) {
			t.Errorf("%d paragraphs of one trailer each: %d trailers, in the wrong order or short", lines/2, len(got))
		}
	})

	// The joining is the same as it was a line at a time.
	rest, got := SplitTrailers("Fixes: the gauge\n  that drifted\n\tat dusk\n[note one]\n  and more\nSigned-off-by: Ada <ada@example.com>")
	if len(got) != 2 || got[0].Value != "the gauge that drifted at dusk" || rest != "[note one]\n  and more" {
		t.Errorf("SplitTrailers = %q, %+v", rest, got)
	}
}
