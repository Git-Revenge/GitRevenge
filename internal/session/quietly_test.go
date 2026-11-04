package session

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/watch"
)

func TestQuietly(t *testing.T) {
	r := &Repo{}
	var got []watch.Kind
	r.changes.deliver = func(k watch.Kind) { got = append(got, k) }

	r.changes.report(watch.Refs)
	if len(got) != 1 || got[0] != watch.Refs {
		t.Fatalf("outside: %v", got)
	}

	got = nil
	r.Quietly(func() {
		r.changes.report(watch.Refs)
		r.Quietly(func() { r.changes.report(watch.Head) })
		if len(got) != 0 {
			t.Errorf("delivered while an operation runs: %v", got)
		}
		r.changes.report(watch.Refs)
	})
	if len(got) != 1 || got[0] != watch.Refs|watch.Head {
		t.Errorf("should arrive once, combined: %v", got)
	}

	got = nil
	r.Quietly(func() {})
	if len(got) != 0 {
		t.Errorf("nothing changed: %v", got)
	}
	r.changes.report(watch.Head)
	if len(got) != 1 {
		t.Errorf("should report again afterwards: %v", got)
	}
}
