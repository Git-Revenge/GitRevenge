package git

import (
	"context"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// #1081: a block's own history, as the demo's pick32 has it: added, renamed, its return
// changed. Lines added above it move it down, and a commit elsewhere in the file isn't in it.
func TestBlockHistory(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	top := "// alerts\n\nexport function clamp(v) {\n  return v\n}\n"
	added := fx.CommitFile("a.js", top+"\nexport function pick245(w) {\n  return w * 75\n}\n", "add pick245")
	renamed := fx.CommitFile("a.js", top+"\nexport function pick32(w) {\n  return w * 75\n}\n", "rename it")
	fx.CommitFile("a.js", "// alerts, and more\n"+top[len("// alerts\n"):]+"\nexport function pick32(w) {\n  return w * 75\n}\n", "touch the top")
	changed := fx.CommitFile("a.js", "// alerts, and more\n\n// a note\n"+top[len("// alerts\n\n"):]+"\nexport function pick32(w) {\n  return w * 18\n}\n", "change its return")
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()

	// At the newest, pick32 is lines 8 to 10.
	got, err := repo.BlockHistory(ctx, "a.js", changed, 8, 10)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, v := range got {
		ids = append(ids, v.Subject)
	}
	if strings.Join(ids, ", ") != "change its return, rename it, add pick245" {
		t.Fatalf("versions = %v", ids)
	}
	if got[0].ID != changed || got[1].ID != renamed || got[2].ID != added {
		t.Errorf("ids = %s %s %s", got[0].ID[:7], got[1].ID[:7], got[2].ID[:7])
	}
	// Where the lines were in each: they moved down when the note went in above them.
	if got[0].From != 8 || got[0].To != 10 || got[1].From != 7 || got[1].To != 9 {
		t.Errorf("places = %d-%d, %d-%d", got[0].From, got[0].To, got[1].From, got[1].To)
	}
	var change []string
	for _, l := range got[0].Lines {
		change = append(change, l.Kind+l.Text)
	}
	if strings.Join(change, "|") != " export function pick32(w) {|-  return w * 75|+  return w * 18| }" {
		t.Errorf("the newest's change = %q", change)
	}
	if got[2].Lines[0].Kind != "+" {
		t.Errorf("where it began: %+v", got[2].Lines)
	}
	if _, err := repo.BlockHistory(ctx, "a.js", "", 1, 2); err == nil {
		t.Error("took the work tree")
	}
	if _, err := repo.BlockHistory(ctx, "a.js", changed, 3, 2); err == nil {
		t.Error("took a range that ends before it starts")
	}
}
