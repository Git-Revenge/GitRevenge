package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// chipsOf waits for the graph's rows to carry the chips want says, by subject, each as
// "Key=value" joined with commas, and "" for a row with none.
func chipsOf(t *testing.T, a *App, path string, want map[string]string) {
	t.Helper()
	var got map[string]string
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		page, err := a.GraphRows(path, 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		got = map[string]string{}
		for _, row := range page.Rows {
			var said []string
			for _, c := range page.Chips[row.ID] {
				said = append(said, c.KeyLabel+"="+c.Label)
			}
			got[row.Subject] = strings.Join(said, ",")
		}
		same := len(got) == len(want)
		for subject, chips := range want {
			if got[subject] != chips {
				same = false
			}
		}
		if same && !page.Loading {
			return
		}
	}
	t.Fatalf("the rows' chips = %v, want %v", got, want)
}

// A pinned key's values ride on the graph's rows, and the pin is a line in the repository's
// git config.
func TestTrailerPinShowsChipsOnRows(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "One", "Changelog: fixed", "Closes: #12")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two", "changelog: added", "Closes: #14, #15")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "c.txt", "Three", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "d.txt", "Four")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 4)

	// Nothing is pinned until someone pins it.
	if pins, err := a.TrailerPins(path); err != nil || len(pins) != 0 {
		t.Fatalf("pins at first = %+v, %v", pins, err)
	}
	if page, _ := a.GraphRows(path, 0, 10); page.Chips != nil {
		t.Fatalf("chips with nothing pinned = %v", page.Chips)
	}

	// Pinned by any spelling; kept as the repository writes the key.
	pins, err := a.SetTrailerPin(path, "CHANGELOG", true)
	if err != nil || len(pins) != 1 || pins[0].Key != "changelog" || pins[0].Label != "Changelog" || pins[0].Scope != "local" {
		t.Fatalf("pins = %+v, %v", pins, err)
	}
	config, err := os.ReadFile(filepath.Join(fx.Dir, ".git", "config"))
	if err != nil || !strings.Contains(string(config), "[gitrevenge-trailer \"Changelog\"]") || !strings.Contains(string(config), "show = rows") {
		t.Fatalf("the repository's config, %v:\n%s", err, config)
	}
	chipsOf(t, a, path, map[string]string{"One": "Changelog=fixed", "Two": "Changelog=added", "Three": "Changelog=fixed", "Four": ""})
	page, _ := a.GraphRows(path, 0, 10)
	for _, row := range page.Rows {
		if row.Subject != "One" {
			continue
		}
		if c := page.Chips[row.ID][0]; c.Key != "changelog" || c.Value != "text:fixed" || c.Kind == "" {
			t.Fatalf("One's chip = %+v, want what a pick is made of", c)
		}
	}

	// A second key: a row shows both, in the commit's own order, and a line with two tickets is two chips.
	if pins, err = a.SetTrailerPin(path, "closes", true); err != nil || len(pins) != 2 {
		t.Fatalf("pins = %+v, %v", pins, err)
	}
	chipsOf(t, a, path, map[string]string{"One": "Changelog=fixed,Closes=#12", "Two": "Changelog=added,Closes=#14,Closes=#15", "Three": "Changelog=fixed", "Four": ""})

	// A new commit's chip arrives once its trailers are read, without anyone asking.
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "e.txt", "Five", "Changelog: changed")
	chipsOf(t, a, path, map[string]string{"Five": "Changelog=changed", "One": "Changelog=fixed,Closes=#12", "Two": "Changelog=added,Closes=#14,Closes=#15", "Three": "Changelog=fixed", "Four": ""})

	// A chip is what a filter is made of.
	state, err := a.SetTrailerFilter(path, []trailerindex.Pick{{Key: "changelog", Value: "text:fixed"}}, false)
	if err != nil || state.Kept != 2 {
		t.Fatalf("filtering by One's chip = %+v, %v", state, err)
	}
	if _, err = a.SetTrailerFilter(path, nil, true); err != nil {
		t.Fatal(err)
	}

	// The same pin unpins, and leaves nothing behind.
	if _, err = a.SetTrailerPin(path, "Closes", false); err != nil {
		t.Fatal(err)
	}
	if pins, err = a.SetTrailerPin(path, "changelog", false); err != nil || len(pins) != 0 {
		t.Fatalf("pins after unpinning = %+v, %v", pins, err)
	}
	if config, _ = os.ReadFile(filepath.Join(fx.Dir, ".git", "config")); strings.Contains(string(config), "gitrevenge-trailer") {
		t.Fatalf("the repository's config after unpinning:\n%s", config)
	}
	chipsOf(t, a, path, map[string]string{"Five": "", "One": "", "Two": "", "Three": "", "Four": ""})
}

// A key the global config pins shows in every repository; unpinning it in one turns it off
// there and leaves the global config alone.
func TestTrailerPinFromTheGlobalConfig(t *testing.T) {
	isolateGlobal(t)
	global := os.Getenv("GIT_CONFIG_GLOBAL")
	if err := os.WriteFile(global, []byte("[gitrevenge-trailer \"Changelog\"]\n\tshow = rows\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t)
	fx := gittest.New(t)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "a.txt", "One", "Changelog: fixed")
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "b.txt", "Two")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 2)

	pins, err := a.TrailerPins(path)
	if err != nil || len(pins) != 1 || pins[0].Key != "changelog" || pins[0].Scope != "global" {
		t.Fatalf("pins = %+v, %v, want the global one", pins, err)
	}
	chipsOf(t, a, path, map[string]string{"One": "Changelog=fixed", "Two": ""})

	if pins, err = a.SetTrailerPin(path, "changelog", false); err != nil || len(pins) != 0 {
		t.Fatalf("pins after unpinning here = %+v, %v", pins, err)
	}
	if text, _ := os.ReadFile(global); !strings.Contains(string(text), "show = rows") {
		t.Fatalf("the global config was changed:\n%s", text)
	}
	if local, _ := os.ReadFile(filepath.Join(fx.Dir, ".git", "config")); !strings.Contains(string(local), "show = none") {
		t.Fatalf("the repository's config doesn't turn it off:\n%s", local)
	}
	chipsOf(t, a, path, map[string]string{"One": "", "Two": ""})

	// Pinning it again here is the repository's own pin.
	if pins, err = a.SetTrailerPin(path, "changelog", true); err != nil || len(pins) != 1 || pins[0].Scope != "local" {
		t.Fatalf("pins after pinning here = %+v, %v", pins, err)
	}
	chipsOf(t, a, path, map[string]string{"One": "Changelog=fixed", "Two": ""})
}
