package git

import (
	"context"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestFileHistoryFollowsRenames(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("old.txt", strings.Repeat("content\n", 10), "create")
	fx.CommitFile("other.txt", "x", "unrelated")
	fx.CommitFile("old.txt", strings.Repeat("content\n", 10)+"more\n", "edit")
	fx.Git("mv", "old.txt", "new.txt")
	fx.Commit("rename")
	fx.CommitFile("new.txt", strings.Repeat("content\n", 10)+"more\nand more\n", "edit after rename")

	hist, err := OpenRepo(newTestRunner(t), fx.Dir).FileHistory(context.Background(), "new.txt")
	if err != nil {
		t.Fatal(err)
	}
	var subjects []string
	for _, c := range hist {
		subjects = append(subjects, c.Subject)
	}
	if strings.Join(subjects, ",") != "edit after rename,rename,edit,create" {
		t.Fatalf("history = %v", subjects)
	}
	if hist[1].Status != "R" || hist[1].OldPath != "old.txt" || hist[1].Path != "new.txt" {
		t.Fatalf("rename entry = %+v", hist[1])
	}
	if hist[2].Path != "old.txt" || hist[3].Status != "A" {
		t.Fatalf("older entries should use the old name: %+v / %+v", hist[2], hist[3])
	}
	if hist[0].Author != "Test User" || hist[0].Time == 0 || len(hist[0].ID) != 40 {
		t.Fatalf("fields = %+v", hist[0])
	}
}

func TestBlame(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	first := fx.CommitFile("f.go", "package main\n\nfunc a() {}\n", "add a")
	second := fx.CommitFile("f.go", "package main\n\nfunc a() {}\nfunc b() {}\n", "add b")
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	lines, err := repo.Blame(context.Background(), "f.go", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 4 {
		t.Fatalf("got %d lines", len(lines))
	}
	if lines[0].Commit != first || lines[0].Summary != "add a" || lines[0].Line != "package main" {
		t.Fatalf("line 1 = %+v", lines[0])
	}
	if lines[3].Commit != second || lines[3].Summary != "add b" || lines[3].Line != "func b() {}" || lines[3].Author != "Test User" {
		t.Fatalf("line 4 = %+v", lines[3])
	}

	// Uncommitted edits are attributed to the work tree.
	fx.WriteFile("f.go", "package main\n\nfunc a() {}\nfunc b() {}\nfunc c() {}\n")
	lines, _ = repo.Blame(context.Background(), "f.go", "")
	if last := lines[len(lines)-1]; !strings.HasPrefix(last.Commit, "0000000") {
		t.Fatalf("uncommitted line = %+v", last)
	}
	// At a revision.
	lines, _ = repo.Blame(context.Background(), "f.go", first)
	if len(lines) != 3 {
		t.Fatalf("blame at first commit = %d lines", len(lines))
	}
}
