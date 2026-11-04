package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestResolveWindow(t *testing.T) {
	// Thursday 2026-10-29 17:42:00 UTC.
	asOf := time.Date(2026, 10, 29, 17, 42, 0, 0, time.UTC).Unix()
	utc := func(y int, m time.Month, d int) int64 { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() }
	for _, c := range []struct {
		in    InsightsInputs
		since int64
	}{
		{InsightsInputs{Window: "7d"}, utc(2026, 10, 23)},
		{InsightsInputs{Window: "14d"}, utc(2026, 10, 16)},
		{InsightsInputs{Window: "month"}, utc(2026, 10, 1)},
	} {
		since, until, err := resolveWindow(c.in, asOf)
		if err != nil || since != c.since || until != asOf {
			t.Errorf("%s: %d..%d, %v; want %d..%d", c.in.Window, since, until, err, c.since, asOf)
		}
	}
	if s, u, err := resolveWindow(InsightsInputs{Window: "custom", Since: 10, Until: 20}, asOf); err != nil || s != 10 || u != 20 {
		t.Errorf("custom = %d..%d, %v", s, u, err)
	}
	if _, _, err := resolveWindow(InsightsInputs{Window: "custom", Since: 20, Until: 10}, asOf); err == nil {
		t.Error("a backwards custom window was accepted")
	}
	if _, _, err := resolveWindow(InsightsInputs{Window: "fortnight"}, asOf); err == nil {
		t.Error("an unknown window was accepted")
	}
}

func TestInsightsShippedIsReproducible(t *testing.T) {
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "1\n", "Start")
	fx.CommitFile("a.txt", "2\n", "Second (TIDE-1)")
	fx.Git("tag", "v1")
	fx.CommitFile("a.txt", "3\n", "Third (TIDE-2)")
	fx.Git("tag", "v2")
	a := newTestApp(t)

	first, err := a.InsightsShipped(fx.Dir, InsightsInputs{Window: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Window.Target != "main" || first.Window.TargetSHA != fx.Head() || len(first.Items) != 3 {
		t.Fatalf("result = %+v", first)
	}
	// The as-of point is the tip's commit date, not the time the test runs.
	if first.Window.AsOf != first.Items[0].Committed {
		t.Errorf("as of = %d, want the tip's commit date %d", first.Window.AsOf, first.Items[0].Committed)
	}
	again, err := newTestApp(t).InsightsShipped(fx.Dir, InsightsInputs{Window: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, again) {
		t.Errorf("same repository and inputs gave different results:\n%+v\n%+v", first, again)
	}

	rel, err := a.InsightsShipped(fx.Dir, InsightsInputs{Window: "releases", From: "v1", To: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if rel.Window.Range != "v1..v2" || len(rel.Items) != 1 || rel.Items[0].Title != "Third (TIDE-2)" {
		t.Errorf("releases = %+v", rel)
	}
	if _, err := a.InsightsShipped(fx.Dir, InsightsInputs{Window: "releases", From: "v1", To: "nope"}); err == nil {
		t.Error("a missing tag was accepted")
	}
	if _, err := a.InsightsShipped(fx.Dir, InsightsInputs{Target: "nope"}); err == nil {
		t.Error("a missing target was accepted")
	}
	tags, err := a.InsightsTags(fx.Dir)
	if err != nil || len(tags) != 2 {
		t.Errorf("tags = %+v, %v", tags, err)
	}
}
