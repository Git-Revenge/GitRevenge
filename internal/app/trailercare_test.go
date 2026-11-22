package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
	"github.com/git-revenge/gitrevenge/internal/trailerindex"
)

// careRepo is a history with a slip of spelling, two ticket verbs and a rare key.
func careRepo(t *testing.T) (*App, *gittest.Repo, string) {
	t.Helper()
	a := newTestApp(t)
	fx := gittest.New(t)
	sam := "Sam Ortiz <sam@tidewater.example>"
	for i := range 12 {
		commitAs(fx, "Sam Ortiz", "sam@tidewater.example", fmt.Sprintf("f%d.txt", i), fmt.Sprintf("Commit %d", i), "Reviewed-by: "+sam, fmt.Sprintf("Closes: #%d", i), "Changelog: fixed")
	}
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "slip.txt", "A slip", "Rviewed-by: "+sam)
	for i := range 4 {
		commitAs(fx, "Sam Ortiz", "sam@tidewater.example", fmt.Sprintf("x%d.txt", i), fmt.Sprintf("Fix %d", i), fmt.Sprintf("Fixes: #%d", 100+i))
	}
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "note.txt", "A note", "Note: once only")
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 18)
	return a, fx, path
}

func careKey(c TrailerCare, id string) (CareKey, bool) {
	for _, k := range c.Keys {
		if k.Key.ID == id {
			return k, true
		}
	}
	return CareKey{}, false
}

func careAsks(c TrailerCare) []string {
	var out []string
	for _, q := range c.Questions {
		out = append(out, q.Kind+":"+q.OtherLabel+" as "+q.KeyLabel)
	}
	return out
}

// Trailer care asks what it can't decide, and an answer is a rule in the repository's git
// config that changes how the history is read and nothing else.
func TestTrailerCareAsksAndRemembersTheAnswer(t *testing.T) {
	t.Parallel()
	a, fx, path := careRepo(t)
	config := func() string {
		text, err := os.ReadFile(filepath.Join(fx.Dir, ".git", "config"))
		if err != nil {
			t.Fatal(err)
		}
		return string(text)
	}
	before := fx.Git("rev-parse", "HEAD")

	care, err := a.TrailerCare(path)
	if err != nil {
		t.Fatal(err)
	}
	asks := careAsks(care)
	if len(asks) != 2 || care.Open != 2 || asks[0] != "spelling:Rviewed-by as Reviewed-by" || asks[1] != "same:Fixes as Closes" {
		t.Fatalf("questions = %v (%d open), want the slip, then the ticket verbs", asks, care.Open)
	}
	// The table says where each key shows, and that it's a guess.
	if k, ok := careKey(care, "note"); !ok || k.Where != "held" || k.Key.Held != trailerindex.HeldRare || k.WhereSaid || k.KindSaid {
		t.Fatalf("Note in the table = %+v, %v", k, ok)
	}
	if k, _ := careKey(care, "changelog"); k.Where != "panel" || k.Scope != "" {
		t.Fatalf("Changelog in the table = %+v", k)
	}
	// The panel's pill counts the same questions.
	if panel, _ := a.TrailerPanel(path, "", 0); panel.Care != 2 {
		t.Fatalf("the panel counts %d questions, want 2", panel.Care)
	}

	// Read the slip as the key it's a slip for.
	care, err = a.AnswerTrailerQuestion(path, "reviewed-by", "rviewed-by", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(config(), "[gitrevenge-trailer \"Reviewed-by\"]") || !strings.Contains(config(), "also = Rviewed-by") {
		t.Fatalf("the config after joining:\n%s", config())
	}
	if asks = careAsks(care); len(asks) != 1 || asks[0] != "same:Fixes as Closes" {
		t.Fatalf("questions after joining = %v", asks)
	}
	k, _ := careKey(care, "reviewed-by")
	if k.Key.Commits != 13 || len(k.Also) != 1 || k.Also[0] != "Rviewed-by" || k.Scope != "local" {
		t.Fatalf("Reviewed-by after joining = %+v, want 13 commits and the join to undo", k)
	}
	if _, still := careKey(care, "rviewed-by"); still {
		t.Fatal("the slip is still a key of its own")
	}
	// The panel, a search and a filter all read it the new way.
	if values, _ := a.TrailerValues(path, "Rviewed-by", "", 0, 0); values.Matching != 1 || values.Values[0].Commits != 13 {
		t.Fatalf("values asked for by the slip = %+v", values)
	}
	if found, _ := a.SearchGraph(path, "Reviewed-by: Sam"); found.Total != 13 {
		t.Fatalf("a search finds %d commits, want 13", found.Total)
	}

	// Keep the ticket verbs apart: the question isn't asked again, and nothing joins.
	care, err = a.AnswerTrailerQuestion(path, "closes", "fixes", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(care.Questions) != 0 || care.Open != 0 {
		t.Fatalf("questions after keeping apart = %v", careAsks(care))
	}
	if !strings.Contains(config(), "apart = Fixes") {
		t.Fatalf("the config after keeping apart:\n%s", config())
	}
	if k, _ = careKey(care, "fixes"); k.Key.Commits != 4 {
		t.Fatalf("Fixes after being kept apart = %+v", k)
	}
	if panel, _ := a.TrailerPanel(path, "", 0); panel.Care != 0 {
		t.Fatalf("the panel still counts %d questions", panel.Care)
	}

	// Undo the join: the slip is its own key again, and the question comes back.
	care, err = a.UndoTrailerJoin(path, "reviewed-by", "Rviewed-by")
	if err != nil {
		t.Fatal(err)
	}
	if asks = careAsks(care); len(asks) != 1 || asks[0] != "spelling:Rviewed-by as Reviewed-by" {
		t.Fatalf("questions after undoing the join = %v", asks)
	}
	if strings.Contains(config(), "also = Rviewed-by") {
		t.Fatalf("the join is still in the config:\n%s", config())
	}

	// Nothing but the config changed.
	if after := fx.Git("rev-parse", "HEAD"); after != before {
		t.Fatalf("HEAD moved from %s to %s", before, after)
	}
	if status := fx.Git("status", "--porcelain"); status != "" {
		t.Fatalf("the working tree changed:\n%s", status)
	}
}

// Several slips of one key are answered together, and written as a line each.
func TestTrailerCareAnswersSeveralSlipsTogether(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	sam := "Sam Ortiz <sam@tidewater.example>"
	for i := range 12 {
		commitAs(fx, "Sam Ortiz", "sam@tidewater.example", fmt.Sprintf("f%d.txt", i), fmt.Sprintf("Commit %d", i), "Reviewed-by: "+sam)
	}
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "s1.txt", "A slip", "Rviewed-by: "+sam)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "s2.txt", "Another", "Reviewed-bt: "+sam)
	commitAs(fx, "Sam Ortiz", "sam@tidewater.example", "s3.txt", "A third", "Reviewed-byy: "+sam)
	path := openForMap(t, a, fx)
	trailersRead(t, a, path)
	graphRows(t, a, path, 15)
	if care, _ := a.TrailerCare(path); care.Open != 3 {
		t.Fatalf("%d questions to begin with: %v", care.Open, careAsks(care))
	}

	care, err := a.AnswerTrailerSlips(path, "reviewed-by", []string{"rviewed-by", "reviewed-bt"})
	if err != nil {
		t.Fatal(err)
	}
	if asks := careAsks(care); len(asks) != 1 || asks[0] != "spelling:Reviewed-byy as Reviewed-by" {
		t.Fatalf("questions after answering two together = %v, want the third", asks)
	}
	k, _ := careKey(care, "reviewed-by")
	if k.Key.Commits != 14 || len(k.Also) != 2 {
		t.Fatalf("Reviewed-by after = %+v, want 14 commits and two joins to undo", k)
	}
	text, _ := os.ReadFile(filepath.Join(fx.Dir, ".git", "config"))
	if strings.Count(string(text), "also = ") != 2 || strings.Count(string(text), "[gitrevenge-trailer") != 1 {
		t.Fatalf("the config:\n%s", text)
	}
	// One that can't be joined stops there and says so, and what was written stands.
	if _, err = a.AnswerTrailerSlips(path, "reviewed-by", []string{"reviewed-byy", "reviewed-by"}); err == nil {
		t.Fatal("a key was joined to itself")
	}
	if care, _ = a.TrailerCare(path); care.Open != 0 {
		t.Fatalf("the one before the bad one wasn't written: %v", careAsks(care))
	}
}

// What a key's values are, and where it shows, are guesses until a rule says.
func TestTrailerCareSetsKindAndPlace(t *testing.T) {
	t.Parallel()
	a, _, path := careRepo(t)

	// A key held back as rare is offered once the repository says so.
	care, err := a.SetTrailerPlace(path, "note", "panel")
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := careKey(care, "note"); k.Where != "panel" || !k.WhereSaid || k.Key.Held != "" || k.Scope != "local" {
		t.Fatalf("Note offered = %+v", k)
	}
	if panel, _ := a.TrailerPanel(path, "", 0); !panelHas(panel, "note") {
		t.Fatal("the panel doesn't offer a key the rules offer")
	}
	// And one that's offered can be held back.
	if care, err = a.SetTrailerPlace(path, "changelog", "held"); err != nil {
		t.Fatal(err)
	}
	if k, _ := careKey(care, "changelog"); k.Where != "held" || !k.WhereSaid || k.Key.Held != trailerindex.HeldRule {
		t.Fatalf("Changelog held = %+v", k)
	}
	if panel, _ := a.TrailerPanel(path, "", 0); panelHas(panel, "changelog") {
		t.Fatal("the panel still offers a key the rules hold back")
	}
	// On rows is a pin: the same thing the pin beside a key does.
	if care, err = a.SetTrailerPlace(path, "closes", "rows"); err != nil {
		t.Fatal(err)
	}
	if k, _ := careKey(care, "closes"); k.Where != "rows" || !k.WhereSaid {
		t.Fatalf("Closes on rows = %+v", k)
	}
	if pins, _ := a.TrailerPins(path); len(pins) != 1 || pins[0].Key != "closes" {
		t.Fatalf("pins = %+v", pins)
	}
	// Left to be guessed again.
	if care, err = a.SetTrailerPlace(path, "changelog", ""); err != nil {
		t.Fatal(err)
	}
	if k, _ := careKey(care, "changelog"); k.Where != "panel" || k.WhereSaid {
		t.Fatalf("Changelog guessed again = %+v", k)
	}

	// What its values are.
	if care, err = a.SetTrailerKind(path, "note", "labels"); err != nil {
		t.Fatal(err)
	}
	if k, _ := careKey(care, "note"); k.Key.Kind != trailerindex.KindLabel || !k.KindSaid {
		t.Fatalf("Note as labels = %+v", k)
	}
	if care, err = a.SetTrailerKind(path, "note", ""); err != nil {
		t.Fatal(err)
	}
	if k, _ := careKey(care, "note"); k.KindSaid {
		t.Fatalf("Note guessed again = %+v", k)
	}

	// What can't be said is refused, and says so.
	if _, err = a.SetTrailerKind(path, "note", "tikets"); err == nil {
		t.Fatal("a kind there's no word for was accepted")
	}
	if _, err = a.SetTrailerPlace(path, "note", "everywhere"); err == nil {
		t.Fatal("a place there's no word for was accepted")
	}
	if _, err = a.AnswerTrailerQuestion(path, "closes", "closed", true); err == nil {
		t.Fatal("a key was joined to itself")
	}
}

func panelHas(p TrailerPanel, id string) bool {
	for _, k := range p.Keys {
		if k.Key.ID == id {
			return true
		}
	}
	return false
}

// A rule written by hand, or by another window, is heard without being told.
func TestTrailerCareHearsARuleWrittenByHand(t *testing.T) {
	t.Parallel()
	a, fx, path := careRepo(t)
	if care, _ := a.TrailerCare(path); care.Open != 2 {
		t.Fatalf("%d questions to begin with", care.Open)
	}
	fx.Git("config", "--local", "--add", "gitrevenge-trailer.Reviewed-by.also", "Rviewed-by")
	var care TrailerCare
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		care, _ = a.TrailerCare(path)
		if care.Open == 1 {
			break
		}
	}
	if care.Open != 1 {
		t.Fatalf("a rule written by hand wasn't heard: %v", careAsks(care))
	}
	if k, _ := careKey(care, "reviewed-by"); k.Key.Commits != 13 {
		t.Fatalf("Reviewed-by = %+v", k)
	}
}
