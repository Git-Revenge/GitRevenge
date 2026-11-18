package session

import (
	"context"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestClosingARepositoryStopsItsReads(t *testing.T) {
	m, _ := newManager(t)
	fx := gittest.New(t)
	fx.Linear(1)
	r, err := m.Open(context.Background(), fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	g := r.Git
	if _, err := g.ReadString(context.Background(), "rev-parse", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(r.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := g.ReadString(context.Background(), "rev-parse", "HEAD"); err == nil {
		t.Fatal("a read ran in a repository that had closed")
	}
}
