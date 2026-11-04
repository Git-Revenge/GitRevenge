package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestConflictBindings(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.WithConflict(t)
	tab, _ := a.OpenRepo(fx.Dir)

	cf, err := a.ConflictFile(tab.Path, "conflict.txt")
	if err != nil || cf.Ours.Text != "ours\n" {
		t.Fatalf("conflict = %+v, %v", cf, err)
	}
	if err := a.ResolveConflict(tab.Path, "conflict.txt", "merged\n"); err != nil {
		t.Fatal(err)
	}
	if err := a.TakeConflictSide(tab.Path, "conflict.txt", "ours"); err == nil || err.Error() != "conflict.txt is no longer conflicted." {
		t.Fatalf("err = %v", err)
	}
	// All conflicts resolved: the merge can be continued from the banner.
	if err := a.RunOperationAction(tab.Path, "continue"); err != nil {
		t.Fatal(err)
	}
	if st, _ := a.RepoState(tab.Path); st.Operation != "" {
		t.Fatalf("operation still in progress: %+v", st)
	}
}
