package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestRulerMarks(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	shas := fx.Linear(5) // oldest first; main ends at the last
	fx.Git("branch", "topic", shas[1])
	fx.Git("tag", "v1", shas[2])
	fx.Git("tag", "v0", shas[0])
	fx.Git("notes", "add", "-m", "deployed", shas[3])
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if page, _ := a.GraphRows(tab.Path, 0, 1); !page.Loading {
			break
		}
	}
	marks, err := a.RulerMarks(tab.Path)
	if err != nil {
		t.Fatal(err)
	}
	// Rows are newest first: shas[4] is row 0, shas[0] row 4.
	if marks.Loaded != 5 {
		t.Fatalf("loaded = %d, want 5", marks.Loaded)
	}
	if want := []int{0, 3}; !reflect.DeepEqual(marks.Tips, want) {
		t.Fatalf("tips = %v, want %v (main and topic)", marks.Tips, want)
	}
	if want := []int{2, 4}; !reflect.DeepEqual(marks.Tags, want) {
		t.Fatalf("tags = %v, want %v", marks.Tags, want)
	}
	if want := []int{1}; !reflect.DeepEqual(marks.Notes, want) {
		t.Fatalf("notes = %v, want %v", marks.Notes, want)
	}
	if len(marks.Months) == 0 || marks.Months[0].Row != 0 {
		t.Fatalf("months = %+v, want the first row's month first", marks.Months)
	}
	// A repository that isn't open has none, as empty lists.
	none, err := a.RulerMarks(fx.Dir + "-elsewhere")
	if err == nil || none.Tips == nil || none.Tags == nil || none.Notes == nil || none.Months == nil {
		t.Fatalf("not open: %+v, %v", none, err)
	}
}
