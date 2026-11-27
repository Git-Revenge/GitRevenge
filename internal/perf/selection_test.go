package perf

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// Several commits selected (#1018): Shift+click across the kernel selects 100,000 rows, and
// that mustn't stall. What the selection costs is reading those rows' commits from the graph
// and sending their IDs to the UI; the panel then reads files for the oldest 500.
const (
	selectionRows       = 100_000
	targetSelection     = 500 * time.Millisecond
	selectionFileRows   = 500
	targetSelectionFile = 10 * time.Second
)

func TestSelection(t *testing.T) {
	benchRepo(t)
	dir := os.Getenv("GITREVENGE_BENCH_REPO")
	ctx := context.Background()
	bin, _ := exec.LookPath("git")
	store, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := session.NewManager(git.NewRunner(bin), store)
	defer m.CloseAll()
	repo, err := m.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	repo.DemandRows(selectionRows)
	deadline := time.Now().Add(10 * time.Minute)
	for {
		g, loading := repo.Graph()
		if g != nil && (g.Len() >= selectionRows || !loading) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the graph never read 100,000 rows")
		}
		repo.DemandRows(selectionRows)
		time.Sleep(100 * time.Millisecond)
	}
	g, _ := repo.Graph()
	rows := min(g.Len(), selectionRows)

	start := time.Now()
	commits := g.Commits(0, rows)
	ids := make([]string, 0, len(commits))
	for i := len(commits) - 1; i >= 0; i-- {
		ids = append(ids, commits[i].ID)
	}
	out, err := json.Marshal(struct {
		IDs []string `json:"ids"`
	}{ids})
	if err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	t.Logf("selecting %d rows: %v, %d KB of IDs to the UI", rows, took, len(out)/1024)
	if took > targetSelection {
		t.Errorf("selecting %d rows took %v, want under %v", rows, took, targetSelection)
	}

	start = time.Now()
	byCommit, err := repo.Git.FilesByCommit(ctx, ids[:min(len(ids), selectionFileRows)])
	if err != nil {
		t.Fatal(err)
	}
	took = time.Since(start)
	files := 0
	for _, fs := range byCommit {
		files += len(fs)
	}
	t.Logf("their files, from the oldest %d commits: %v, %d changes", selectionFileRows, took, files)
	if took > targetSelectionFile {
		t.Errorf("reading the files took %v, want under %v", took, targetSelectionFile)
	}
}
