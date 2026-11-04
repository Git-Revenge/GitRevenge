package app

import (
	"slices"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestRepoStateAndAbort(t *testing.T) {
	a := newTestApp(t)
	fx := gittest.WithConflict(t)
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}

	st, err := a.RepoState(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Operation != "merge" || !slices.Equal(st.Actions, []string{"continue", "abort"}) || len(st.Conflicts) != 1 {
		t.Fatalf("state = %+v", st)
	}
	if err := a.RunOperationAction(tab.Path, "skip"); err == nil {
		t.Fatal("skip is not valid for a merge")
	}
	if err := a.RunOperationAction(tab.Path, "abort"); err != nil {
		t.Fatal(err)
	}
	if st, _ := a.RepoState(tab.Path); st.Operation != "" {
		t.Fatalf("after abort: %+v", st)
	}
}
