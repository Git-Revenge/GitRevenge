package app

import "testing"

// Closing a tab lets go of the trailer reading and the People history worked out for it,
// which on a long history are hundreds of MB (#903).
func TestClosingATabForgetsWhatWasReadOfIt(t *testing.T) {
	t.Parallel()
	a, _, path := peopleRepo(t)
	trailersRead(t, a, path)
	if _, err := a.TrailerKeys(path); err != nil {
		t.Fatal(err)
	}
	if _, err := a.People(path, PeopleRequest{}); err != nil {
		t.Fatal(err)
	}
	kept := func() (reading, people bool) {
		a.trailerReadings.mu.Lock()
		reading = len(a.trailerReadings.kept) > 0
		a.trailerReadings.mu.Unlock()
		a.people.mu.Lock()
		people = len(a.people.kept) > 0
		a.people.mu.Unlock()
		return reading, people
	}
	if reading, people := kept(); !reading || !people {
		t.Fatalf("before closing: reading kept %v, people kept %v", reading, people)
	}

	if err := a.CloseRepo(path); err != nil {
		t.Fatal(err)
	}
	if reading, people := kept(); reading || people {
		t.Fatalf("after closing: reading kept %v, people kept %v", reading, people)
	}
}
