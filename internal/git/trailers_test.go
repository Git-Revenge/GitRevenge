package git

import (
	"context"
	"reflect"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestSplitTrailers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		body     string
		rest     string
		trailers []Trailer
	}{
		{
			name:     "after a paragraph",
			body:     "Explains the change.\n\nCloses #12\nCo-Authored-By: Ada <ada@example.com>",
			rest:     "Explains the change.",
			trailers: []Trailer{{"Closes", "#12"}, {"Co-Authored-By", "Ada <ada@example.com>"}},
		},
		{
			name:     "the whole body",
			body:     "Signed-off-by: Linus <torvalds@example.org>\n",
			rest:     "",
			trailers: []Trailer{{"Signed-off-by", "Linus <torvalds@example.org>"}},
		},
		{
			name:     "continuation lines and CRLF",
			body:     "Body.\r\n\r\nNote: a long value\r\n  that wraps\r\nAcked-by: Bo <bo@example.com>",
			rest:     "Body.",
			trailers: []Trailer{{"Note", "a long value that wraps"}, {"Acked-by", "Bo <bo@example.com>"}},
		},
		{
			name:     "a closing paragraph before the trailers, as this repo writes them",
			body:     "Explains it.\n\nCloses #171\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
			rest:     "Explains it.",
			trailers: []Trailer{{"Closes", "#171"}, {"Co-Authored-By", "Claude <noreply@anthropic.com>"}},
		},
		{
			name:     "several closing references",
			body:     "fixes #1, #2",
			rest:     "",
			trailers: []Trailer{{"fixes", "#1, #2"}},
		},
		{
			name: "a last paragraph that isn't all trailers",
			body: "Intro.\n\nSee: the docs\nand then some prose",
			rest: "Intro.\n\nSee: the docs\nand then some prose",
		},
		{
			name: "a closing keyword inside prose",
			body: "This closes #4 by accident.",
			rest: "This closes #4 by accident.",
		},
		{
			name: "a URL on its own line",
			body: "Intro.\n\nhttps://example.com/issue/1",
			rest: "Intro.\n\nhttps://example.com/issue/1",
		},
		{
			name: "trailer-like text before the last paragraph",
			body: "Fixes: something\n\nThen prose at the end.",
			rest: "Fixes: something\n\nThen prose at the end.",
		},
		{
			// The Linux kernel's way: a note about who changed what, between the sign-offs.
			name: "a note among the trailers",
			body: "Explains it.\n\n[akpm@example.org: shorten the locked section]\nLink: https://lore.example/r/1\nSigned-off-by: Baolin <baolin@example.com>\nAcked-by: Linus <torvalds@example.org>",
			rest: "Explains it.\n\n[akpm@example.org: shorten the locked section]",
			trailers: []Trailer{
				{"Link", "https://lore.example/r/1"},
				{"Signed-off-by", "Baolin <baolin@example.com>"},
				{"Acked-by", "Linus <torvalds@example.org>"},
			},
		},
		{
			name:     "a note that wraps keeps its second line",
			body:     "[bo@example.com: reworded the message,\n  and fixed the build]\nSigned-off-by: Bo <bo@example.com>\nNote: a value\n  that wraps",
			rest:     "[bo@example.com: reworded the message,\n  and fixed the build]",
			trailers: []Trailer{{"Signed-off-by", "Bo <bo@example.com>"}, {"Note", "a value that wraps"}},
		},
		{
			name:     "the note a cherry-pick leaves",
			body:     "Explains it.\n\nSigned-off-by: Ada <ada@example.com>\n(cherry picked from commit 0123456789abcdef0123456789abcdef01234567)",
			rest:     "Explains it.\n\n(cherry picked from commit 0123456789abcdef0123456789abcdef01234567)",
			trailers: []Trailer{{"Signed-off-by", "Ada <ada@example.com>"}},
		},
		{
			name: "a sign-off under prose that outweighs it",
			body: "Intro.\n\nOne line of prose\ntwo lines of prose\nthree lines of prose\nfour lines of prose\nSigned-off-by: Ada <ada@example.com>",
			rest: "Intro.\n\nOne line of prose\ntwo lines of prose\nthree lines of prose\nfour lines of prose\nSigned-off-by: Ada <ada@example.com>",
		},
		{
			// Without a line git writes, a stray line still means it isn't a trailer paragraph.
			name: "a note among trailers nobody signed",
			body: "Intro.\n\n[a note]\nReviewed-by: Ada <ada@example.com>\nCloses: #4",
			rest: "Intro.\n\n[a note]\nReviewed-by: Ada <ada@example.com>\nCloses: #4",
		},
		{
			// Only the last paragraph may be mixed: one before it must be all trailers.
			name:     "a mixed paragraph before the last",
			body:     "[a note]\nSigned-off-by: Ada <ada@example.com>\n\nCloses #4",
			rest:     "[a note]\nSigned-off-by: Ada <ada@example.com>",
			trailers: []Trailer{{"Closes", "#4"}},
		},
		{
			name:     "a paragraph of trailers before a mixed one",
			body:     "Explains it.\n\nCloses #4\n\n[a note]\nSigned-off-by: Ada <ada@example.com>",
			rest:     "Explains it.\n\n[a note]",
			trailers: []Trailer{{"Closes", "#4"}, {"Signed-off-by", "Ada <ada@example.com>"}},
		},
		{
			// Git once listed a merge's conflicts under the message, and reads past the list.
			name:     "a list of conflicts under the sign-off",
			body:     "Merge to resolve conflicts.\n\nSigned-off-by: Ada <ada@example.com>\n\nConflicts:\n\tdrivers/a.c\n\tdrivers/b.c",
			rest:     "Merge to resolve conflicts.\n\nConflicts:\n\tdrivers/a.c\n\tdrivers/b.c",
			trailers: []Trailer{{"Signed-off-by", "Ada <ada@example.com>"}},
		},
		{
			name:     "comment lines at the end",
			body:     "Explains it.\n\nReviewed-by: Ada <ada@example.com>\n\n# Conflicts:\n#\tdrivers/a.c",
			rest:     "Explains it.\n\n# Conflicts:\n#\tdrivers/a.c",
			trailers: []Trailer{{"Reviewed-by", "Ada <ada@example.com>"}},
		},
		{
			name:     "comment lines among the trailers",
			body:     "Explains it.\n\n# Conflicts:\n#\tdrivers/a.c\nFrom: Bo <bo@example.com>\nLink: https://lore.example/r/2",
			rest:     "Explains it.\n\n# Conflicts:\n#\tdrivers/a.c",
			trailers: []Trailer{{"From", "Bo <bo@example.com>"}, {"Link", "https://lore.example/r/2"}},
		},
		{
			name: "a list of conflicts in the middle of a message",
			body: "Conflicts:\n\tdrivers/a.c\n\nThen prose at the end.",
			rest: "Conflicts:\n\tdrivers/a.c\n\nThen prose at the end.",
		},
		{name: "only a comment", body: "# nothing to see", rest: "# nothing to see"},
		{name: "empty", body: "", rest: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rest, trailers := SplitTrailers(c.body)
			if rest != c.rest || !reflect.DeepEqual(trailers, c.trailers) {
				t.Errorf("SplitTrailers(%q) = %q, %v; want %q, %v", c.body, rest, trailers, c.rest, c.trailers)
			}
		})
	}
}

func TestParseTrailerQuery(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want TrailerQuery
		ok   bool
	}{
		{"Co-authored-by: ada", TrailerQuery{"Co-authored-by", "ada"}, true},
		{"signed-off-by:", TrailerQuery{"signed-off-by", ""}, true},
		{"Closes: #12", TrailerQuery{"Closes", "#12"}, true},
		{"link: patch.msgid", TrailerQuery{"link", "patch.msgid"}, true},
		{"fix: crash on start", TrailerQuery{}, false},
		{"feat: icons", TrailerQuery{}, false},
		{"just text", TrailerQuery{}, false},
	}
	for _, c := range cases {
		got, ok := ParseTrailerQuery(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseTrailerQuery(%q) = %+v, %v; want %+v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestTrailerQueryMatchesIssueRefsWhole(t *testing.T) {
	t.Parallel()
	q := TrailerQuery{"closes", "#12"}
	for value, want := range map[string]bool{"#12": true, "#11, #12": true, "#123": false, "#112": false} {
		if got := q.Matches(Trailer{"Closes", value}); got != want {
			t.Errorf("closes #12 vs %q = %v, want %v", value, got, want)
		}
	}
}

func TestCommitsWithTrailer(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	ada := fx.CommitFile("a.txt", "a", "Pair work\n\nCo-authored-by: Ada Lovelace <ada@example.com>")
	fx.CommitFile("b.txt", "b", "Mentions it in prose\n\nCo-authored-by: Ada was here, said the note.\nMore prose after.")
	closes := fx.CommitFile("c.txt", "c", "Fix it\n\nCloses #12")
	fx.CommitFile("d.txt", "d", "Other\n\nCloses #123")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	got, err := repo.CommitsWithTrailer(context.Background(), TrailerQuery{"co-authored-by", "ada"})
	if err != nil || !reflect.DeepEqual(got, []string{ada}) {
		t.Fatalf("co-authored-by ada = %v, %v; want only %s", got, err, ada)
	}
	got, err = repo.CommitsWithTrailer(context.Background(), TrailerQuery{"Closes", "#12"})
	if err != nil || !reflect.DeepEqual(got, []string{closes}) {
		t.Fatalf("closes #12 = %v, %v; want only %s", got, err, closes)
	}
}
