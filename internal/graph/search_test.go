package graph

import (
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
)

func searchGraph(t *testing.T) *Graph {
	t.Helper()
	g := New()
	commits := []git.LogCommit{
		{ID: "abcdef0000000000000000000000000000000001", Parents: []string{"abcdef0000000000000000000000000000000002"}, AuthorName: "Ada Lovelace", Subject: "Fix Login bug"},
		{ID: "abcdef0000000000000000000000000000000002", Parents: []string{"1234560000000000000000000000000000000003"}, AuthorName: "Grace Hopper", Subject: "Add search"},
		{ID: "1234560000000000000000000000000000000003", AuthorName: "José Núñez", Subject: "Initial commit"},
	}
	for _, c := range commits {
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func TestSearch(t *testing.T) {
	g := searchGraph(t)
	tests := []struct {
		q    string
		want []int
	}{
		{"login", []int{0}},     // subject, case-insensitive
		{"GRACE", []int{1}},     // author
		{"núñez", []int{2}},     // non-ASCII author
		{"abcdef", []int{0, 1}}, // SHA prefix
		{"12345", []int{2}},     // odd-length SHA prefix
		{"a", []int{0, 1, 2}},   // short query: text only
		{"zzz", []int{}},        // nothing
		{"  ", []int{}},         // blank
	}
	for _, tt := range tests {
		got := g.Search(tt.q, 100)
		if !slices.Equal(got.Rows, tt.want) || got.Total != len(tt.want) {
			t.Errorf("Search(%q) = %v (total %d), want %v", tt.q, got.Rows, got.Total, tt.want)
		}
	}
}

func TestSearchFindsAnAuthorUnderEveryName(t *testing.T) {
	g := New()
	commits := []git.LogCommit{
		// One person, read through a mailmap: shown as Sarah Lind, written two ways.
		{ID: "abcdef0000000000000000000000000000000001", AuthorName: "Sarah Lind", AuthorEmail: "sarah@tidewater.example", WrittenName: "slind", WrittenEmail: "slind@users.noreply.example", Subject: "Tide table"},
		{ID: "abcdef0000000000000000000000000000000002", AuthorName: "Sarah Lind", AuthorEmail: "sarah@tidewater.example", WrittenName: "Sarah Lind", WrittenEmail: "sarah@tidewater.example", Subject: "Units"},
		{ID: "abcdef0000000000000000000000000000000003", AuthorName: "Sam Ortiz", AuthorEmail: "sam@users.noreply.example", WrittenName: "Sam Ortiz", WrittenEmail: "sam@users.noreply.example", Subject: "Alerts"},
	}
	for _, c := range commits {
		if err := g.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		q    string
		want []int
	}{
		{"sarah", []int{0, 1}},              // the name the mailmap gives
		{"slind", []int{0}},                 // the name one commit was written with
		{"slind@users", []int{0}},           // an address a commit has
		{"@tidewater.example", []int{0, 1}}, // the address the mailmap gives
		{"noreply", []int{}},                // not an address, so addresses aren't searched
		{"@users.noreply", []int{0, 2}},     // an address, so they are
	}
	for _, tt := range tests {
		got := g.Search(tt.q, 100)
		if !slices.Equal(got.Rows, tt.want) || got.Total != len(tt.want) {
			t.Errorf("Search(%q) = %v (total %d), want %v", tt.q, got.Rows, got.Total, tt.want)
		}
	}
	if rows := g.Rows(0, 1); rows[0].Author != "Sarah Lind" {
		t.Errorf("a row shows %q, want the name the mailmap gives", rows[0].Author)
	}
}

func TestSearchLimitKeepsTotal(t *testing.T) {
	g := searchGraph(t)
	got := g.Search("a", 2)
	if len(got.Rows) != 2 || got.Total != 3 {
		t.Fatalf("got %+v", got)
	}
}

func BenchmarkSearch(b *testing.B) {
	g := New()
	for i, ps := range randomDAG(100_000, 5) {
		c := git.LogCommit{ID: fakeID(i), AuthorName: "Some Author", Subject: "fix: a reasonably long commit subject line here"}
		for _, p := range ps {
			c.Parents = append(c.Parents, fakeID(p))
		}
		_ = g.Add(c)
	}
	b.ResetTimer()
	for b.Loop() {
		g.Search("nomatch", 1000)
	}
}
