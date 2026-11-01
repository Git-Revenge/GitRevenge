package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenMissingFileGivesDefaults(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if got.Version != schemaVersion || got.Recent == nil || got.OpenTabs == nil {
		t.Fatalf("unexpected defaults: %+v", got)
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	err = s.Update(func(st *Settings) {
		st.AddRecent("/repos/a", now)
		st.OpenTabs = []string{"/repos/a", "/repos/b"}
		st.ActiveTab = "/repos/b"
	})
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Get()
	if len(got.Recent) != 1 || got.Recent[0].Path != "/repos/a" || !got.Recent[0].LastOpened.Equal(now) {
		t.Fatalf("recent = %+v", got.Recent)
	}
	if strings.Join(got.OpenTabs, ",") != "/repos/a,/repos/b" || got.ActiveTab != "/repos/b" {
		t.Fatalf("tabs = %v active = %q", got.OpenTabs, got.ActiveTab)
	}
}

func TestCorruptFileIsMovedAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Get().Recent) != 0 {
		t.Fatal("expected defaults after corrupt file")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "settings.json.corrupt-*"))
	if len(matches) != 1 {
		t.Fatalf("corrupt file should be kept aside, found %v", matches)
	}
}

func TestNewerSchemaIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"version": 99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "settings.json.corrupt-*"))
	if len(matches) != 1 {
		t.Fatal("a file from a newer version should be preserved, not silently overwritten")
	}
}

func TestUpdateLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		if err := s.Update(func(st *Settings) { st.ActiveTab = fmt.Sprint(i) }); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected only settings.json, found %d entries", len(entries))
	}
}

func TestGetReturnsCopy(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(st *Settings) { st.OpenTabs = []string{"/a"} }); err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	got.OpenTabs[0] = "/mutated"
	if s.Get().OpenTabs[0] != "/a" {
		t.Fatal("Get must return a copy")
	}
}

func TestAddRecentDedupesAndCaps(t *testing.T) {
	var st Settings
	base := time.Now()
	for i := range maxRecent + 5 {
		st.AddRecent(fmt.Sprintf("/repos/%d", i), base.Add(time.Duration(i)*time.Second))
	}
	if len(st.Recent) != maxRecent {
		t.Fatalf("len = %d, want %d", len(st.Recent), maxRecent)
	}
	st.AddRecent("/repos/10", base)
	if st.Recent[0].Path != "/repos/10" || len(st.Recent) != maxRecent {
		t.Fatalf("re-adding should move to front without duplicating: %v", st.Recent[:2])
	}
	st.RemoveRecent("/repos/10")
	for _, r := range st.Recent {
		if r.Path == "/repos/10" {
			t.Fatal("RemoveRecent did not remove")
		}
	}
}

func TestConcurrentUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			if err := s.Update(func(st *Settings) { st.AddRecent(fmt.Sprintf("/r/%d", i), time.Now()) }); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reopened.Get().Recent); got != 20 {
		t.Fatalf("recent entries = %d, want 20", got)
	}
}

func TestMoveGroup(t *testing.T) {
	names := func(s Settings) string {
		var out []string
		for _, g := range s.Groups {
			out = append(out, g.Name)
		}
		return strings.Join(out, ",")
	}
	cases := []struct {
		name string
		to   int
		ok   bool
		want string
	}{
		{"a", 2, true, "b,c,a"},
		{"c", 0, true, "c,a,b"},
		{"b", 1, true, "a,b,c"},
		{"b", 0, true, "b,a,c"},
		{"missing", 0, false, "a,b,c"},
		{"a", -1, false, "a,b,c"},
		{"a", 3, false, "a,b,c"},
	}
	for _, c := range cases {
		s := Settings{Groups: []Group{{Name: "a"}, {Name: "b"}, {Name: "c"}}}
		if got := s.MoveGroup(c.name, c.to); got != c.ok {
			t.Errorf("MoveGroup(%q, %d) = %v, want %v", c.name, c.to, got, c.ok)
		}
		if got := names(s); got != c.want {
			t.Errorf("MoveGroup(%q, %d) left %s, want %s", c.name, c.to, got, c.want)
		}
	}
}

func TestMoveInGroup(t *testing.T) {
	cases := []struct {
		group, path string
		to          int
		ok          bool
		want        string
	}{
		{"g", "/r/a", 2, true, "/r/b,/r/c,/r/a"},
		{"g", "/r/c", 0, true, "/r/c,/r/a,/r/b"},
		{"g", "/r/b", 1, true, "/r/a,/r/b,/r/c"},
		{"missing", "/r/a", 0, false, "/r/a,/r/b,/r/c"},
		{"g", "/r/missing", 0, false, "/r/a,/r/b,/r/c"},
		{"g", "/r/a", -1, false, "/r/a,/r/b,/r/c"},
		{"g", "/r/a", 3, false, "/r/a,/r/b,/r/c"},
	}
	for _, c := range cases {
		s := Settings{Groups: []Group{{Name: "other", Paths: []string{"/r/a"}}, {Name: "g", Paths: []string{"/r/a", "/r/b", "/r/c"}}}}
		if got := s.MoveInGroup(c.group, c.path, c.to); got != c.ok {
			t.Errorf("MoveInGroup(%q, %q, %d) = %v, want %v", c.group, c.path, c.to, got, c.ok)
		}
		if got := strings.Join(s.Groups[1].Paths, ","); got != c.want {
			t.Errorf("MoveInGroup(%q, %q, %d) left %s, want %s", c.group, c.path, c.to, got, c.want)
		}
		if got := strings.Join(s.Groups[0].Paths, ","); got != "/r/a" {
			t.Errorf("MoveInGroup changed another group: %s", got)
		}
	}
}

func TestGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Update(func(st *Settings) {
		st.Groups = append(st.Groups, Group{Name: "work", Paths: []string{}})
		st.AddToGroup("work", "/r/a")
		st.AddToGroup("work", "/r/a") // no duplicate
		st.AddToGroup("work", "/r/b")
		st.RemoveFromGroup("work", "/r/a")
	})
	if err != nil {
		t.Fatal(err)
	}
	if st := s.Get(); st.AddToGroup("missing", "/x") {
		t.Fatal("adding to an unknown group should report false")
	}
	reopened, _ := Open(path)
	g := reopened.Get().Groups
	if len(g) != 1 || g[0].Name != "work" || len(g[0].Paths) != 1 || g[0].Paths[0] != "/r/b" {
		t.Fatalf("groups = %+v", g)
	}
	// Get returns a deep copy.
	got := reopened.Get()
	got.Groups[0].Paths[0] = "/mutated"
	if reopened.Get().Groups[0].Paths[0] != "/r/b" {
		t.Fatal("Get must deep-copy groups")
	}
}
