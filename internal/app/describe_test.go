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

// Stepping down the graph asks again before the last answer comes: the newer ask stops it.
func TestANewerDescribeStopsTheLast(t *testing.T) {
	t.Parallel()
	var calls describeCalls
	first, cancelFirst := calls.start("/r")
	defer cancelFirst()
	other, cancelOther := calls.start("/other")
	defer cancelOther()
	second, cancelSecond := calls.start("/r")
	if first.Err() == nil {
		t.Fatal("the newer ask didn't stop the last")
	}
	if second.Err() != nil || other.Err() != nil {
		t.Fatal("it stopped more than the last ask for that repository")
	}
	cancelSecond()
	// Cancelling one that's done does nothing to the next.
	third, cancelThird := calls.start("/r")
	defer cancelThird()
	if third.Err() != nil {
		t.Fatal("a finished ask stopped the next")
	}
}
