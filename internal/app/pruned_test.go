package app

import "testing"

func TestDroppedMessage(t *testing.T) {
	t.Parallel()
	set := func(names ...string) map[string]bool {
		m := map[string]bool{}
		for _, n := range names {
			m[n] = true
		}
		return m
	}
	before := set("origin/main", "origin/a", "origin/b", "origin/c", "origin/d")
	cases := []struct {
		what  string
		after map[string]bool
		want  string
	}{
		{"origin", before, ""},
		{"origin", set("origin/main", "origin/b", "origin/c", "origin/d"), "Fetched origin. 1 deleted branch dropped: origin/a."},
		{"", set("origin/main", "origin/c", "origin/d"), "Fetched every remote. 2 deleted branches dropped: origin/a, origin/b."},
		{"origin", set("origin/main"), "Fetched origin. 4 deleted branches dropped: origin/a, origin/b and 2 more."},
	}
	for _, c := range cases {
		if got := droppedMessage(c.what, before, c.after); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
