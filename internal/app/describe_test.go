package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestDescribeCommit(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	ids := fx.Linear(3)
	fx.Git("tag", "-a", "v1.0", "-m", "v1.0", ids[0])
	fx.Git("tag", "-a", "v1.1", "-m", "v1.1", ids[2])
	path := openForMap(t, a, fx)
	d, err := a.DescribeCommit(path, ids[1])
	if err != nil || d.After != "v1.0" || d.Since != 1 || d.FirstIn != "v1.1" {
		t.Fatalf("%+v, %v", d, err)
	}
}

// Stepping down the graph asks again before the last answer comes: the newer ask stops the
// one before, for that repository only.
func TestANewerDescribeStopsTheLast(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	ids := fx.Linear(1)
	path := openForMap(t, a, fx)
	r, err := a.openRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	// Stand-ins for a describe still running here, and for another kind of read.
	before, doneBefore := a.latest.start("describe", r.Path)
	defer doneBefore()
	other, doneOther := a.latest.start("blame", r.Path)
	defer doneOther()
	if _, err := a.DescribeCommit(path, ids[0]); err != nil {
		t.Fatal(err)
	}
	if before.Err() == nil {
		t.Error("the newer describe didn't stop the one before")
	}
	if other.Err() != nil {
		t.Error("it stopped a read of another kind")
	}
}
