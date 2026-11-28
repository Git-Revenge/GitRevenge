package git

import (
	"context"
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestTreePathsNamesFoldersAndFiles(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("src/alerts/sms.js", "x\n", "sms")
	fx.CommitFile("src/alerts-old/x.js", "x\n", "old")
	fx.CommitFile("README.md", "r\n", "readme")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	got, err := repo.TreePaths(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"src/", "src/alerts/", "src/alerts/sms.js", "src/alerts-old/", "README.md"} {
		if !slices.Contains(got, want) {
			t.Errorf("%q missing from %v", want, got)
		}
	}
	if slices.Contains(got, "src/alerts") || slices.Contains(got, "README.md/") {
		t.Errorf("a folder without its slash, or a file with one: %v", got)
	}
}

func TestMatchPathsRanksWhatStartsWithItFirst(t *testing.T) {
	t.Parallel()
	paths := []string{"docs/", "docs/alerts.md", "src/", "src/alerts/", "src/alerts/gusts.js", "src/alerts/sms.js", "src/salerts.js", "Alerts.txt"}
	cases := []struct {
		typed string
		want  []string
	}{
		{"", []string{"docs/", "src/"}},
		{"src/al", []string{"src/alerts/", "src/alerts/gusts.js", "src/alerts/sms.js"}},
		// A name that starts with it beats one that only holds it; folders before files.
		{"alerts", []string{"Alerts.txt", "src/alerts/", "docs/alerts.md", "src/alerts/gusts.js", "src/alerts/sms.js", "src/salerts.js"}},
		{`src\alerts\s`, []string{"src/alerts/sms.js"}},
		{"nothing", []string{}},
	}
	for _, c := range cases {
		if got := MatchPaths(paths, c.typed, 10); !slices.Equal(got, c.want) {
			t.Errorf("%q: got %v, want %v", c.typed, got, c.want)
		}
	}
	if got := MatchPaths(paths, "s", 2); len(got) != 2 {
		t.Errorf("limit: %v", got)
	}
}
