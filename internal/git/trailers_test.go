package git

import (
	"context"
	"reflect"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestSplitTrailers(t *testing.T) {
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
	q := TrailerQuery{"closes", "#12"}
	for value, want := range map[string]bool{"#12": true, "#11, #12": true, "#123": false, "#112": false} {
		if got := q.Matches(Trailer{"Closes", value}); got != want {
			t.Errorf("closes #12 vs %q = %v, want %v", value, got, want)
		}
	}
}

func TestCommitsWithTrailer(t *testing.T) {
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
