package app

import (
	"context"
	"slices"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/semdiff"
)

// The conflict tool reads the code (ADR 0015, #946). git's line merge still decides what
// conflicts; inside each conflicted place, the three versions are merged again token by
// token, and where the sides changed different tokens the result is suggested, never kept on
// the user's behalf.

// ConflictPlace is one conflicted place's three versions, as the conflict tool split them.
type ConflictPlace struct {
	Ours   string `json:"ours"`
	Base   string `json:"base"`
	Theirs string `json:"theirs"`
	// HasBase is false when the original isn't known; such a place gets no suggestion.
	HasBase bool `json:"hasBase"`
}

// ConflictReading is what reading a conflicted file's code adds to the conflict tool.
type ConflictReading struct {
	// Language is the name of the language the file was read in, or "" when it isn't one
	// read as code.
	Language string `json:"language"`
	// Fallback says why a file in such a language wasn't read (semdiff's Fallback
	// constants); its places then get no suggestions.
	Fallback string `json:"fallback"`
	// Suggestions are the places merged by token, in order, when the file was read.
	Suggestions []semdiff.Suggestion `json:"suggestions"`
	// Driver is the merge driver the file's merge attribute names, such as mergiraf, and
	// DriverCommand what git config says it runs, "" when it isn't set up there. Both are
	// "" when the attribute names none or one of git's own.
	Driver        string `json:"driver"`
	DriverCommand string `json:"driverCommand"`
}

// builtinDrivers are git's own merge drivers, which a merge attribute can also name.
var builtinDrivers = map[string]bool{"text": true, "binary": true, "union": true}

// mergeDriver is the merge driver a merge attribute's value names, and the command git
// config gives it; both "" for none or one of git's own.
func mergeDriver(ctx context.Context, g *git.Repo, attr string) (name, command string) {
	if attr == "" || attr == "set" || attr == "unset" || builtinDrivers[attr] {
		return "", ""
	}
	command, _ = g.ReadString(ctx, "config", "--get", "merge."+attr+".driver")
	return attr, command
}

// ConflictSuggestions reads a conflicted file's code and merges each of its places by token,
// for the conflict tool to pre-fill wherever the sides changed different things. It also
// names the merge driver git ran on the file, if any. The places are the tool's own, so the
// suggestions line up with them.
func (a *App) ConflictSuggestions(path, file string, places []ConflictPlace) (ConflictReading, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return ConflictReading{}, err
	}
	ctx := r.Git.Context()
	cf, err := r.Git.Conflict(ctx, file)
	if err != nil {
		return ConflictReading{}, err
	}
	// The tool is already open with the lines when this arrives, so it can wait a while.
	return readConflict(ctx, r.Git, file, cf, places, semdiff.PatientRead), nil
}

// readConflict reads a conflicted file as code, within limit per read, and merges each of
// places by token.
func readConflict(ctx context.Context, g *git.Repo, file string, cf git.ConflictFile, places []ConflictPlace, limit time.Duration) ConflictReading {
	out := ConflictReading{Suggestions: []semdiff.Suggestion{}}
	attrs, _ := g.Attributes(ctx, file, "merge")
	out.Driver, out.DriverCommand = mergeDriver(ctx, g, attrs["merge"])
	if cf.Binary || !cf.Ours.Exists || !cf.Theirs.Exists {
		return out
	}
	id := fileLanguage(g, file)
	if id == "" {
		id = scriptLanguage(cf.Ours.Text)
	}
	if id == "" {
		return out
	}
	out.Language = semdiff.LanguageName(id)
	// Both sides have to read as code; a file that doesn't keeps the line-based tool.
	res, err := semdiff.Diff(ctx, cf.Ours.Text, cf.Theirs.Text, semdiff.Options{Language: id, ReadTime: limit})
	if err != nil {
		out.Fallback = semdiff.FallbackTime
		return out
	}
	if res.Engine != semdiff.EngineCode {
		out.Fallback = res.Fallback
		return out
	}
	for _, p := range places {
		if !p.HasBase {
			out.Suggestions = append(out.Suggestions, semdiff.Suggestion{Reason: "the original isn't known", Origins: []semdiff.Origin{}, Ours: []semdiff.Stretch{}, Theirs: []semdiff.Stretch{}})
			continue
		}
		out.Suggestions = append(out.Suggestions, semdiff.MergePlace(ctx, id, p.Base, p.Ours, p.Theirs))
	}
	return out
}

// placesOf is a conflicted file's places as the conflict tool will split them: from the
// re-merge that keeps each place's original when there is one.
func placesOf(cf git.ConflictFile) []ConflictPlace {
	text := cf.Diff3
	if text == "" {
		text = cf.Merged.Text
	}
	var out []ConflictPlace
	for _, t := range git.ConflictTexts(text) {
		out = append(out, ConflictPlace{Ours: t.Ours, Base: t.Base, Theirs: t.Theirs, HasBase: t.HasBase})
	}
	return out
}

// ConflictTally is how one conflicted file stands, as the conflict tool will open it.
type ConflictTally struct {
	Path string `json:"path"`
	// Places is how many conflicted places it has, and Suggested how many of them can be
	// combined, so the tool will pre-fill them.
	Places    int `json:"places"`
	Suggested int `json:"suggested"`
	// Language is what it was read as, "" when it wasn't read as code.
	Language string `json:"language"`
	// Driver is the merge driver git ran on it first, if any.
	Driver string `json:"driver"`
}

// DriverSettled is a file a merge driver merged on its own, without a conflict.
type DriverSettled struct {
	Path   string `json:"path"`
	Driver string `json:"driver"`
}

// MergeDriver is a merge driver named in .gitattributes, with the command git config gives
// it, "" when git config doesn't set one up.
type MergeDriver struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// ConflictStop is how the operation's current stop stands: each conflicted file's places and
// suggestions, and the files a merge driver settled.
type ConflictStop struct {
	Files   []ConflictTally `json:"files"`
	Settled []DriverSettled `json:"settled"`
	// Drivers are the merge drivers named for any of these files, by name.
	Drivers []MergeDriver `json:"drivers"`
}

// conflictTallyCap is how many conflicted files a stop reads, and conflictTallyTime how long
// it reads for in all; the rest are listed unread.
const (
	conflictTallyCap  = 30
	conflictTallyTime = 10 * time.Second
)

// ConflictStop tallies the stop the operation in progress is at, for the operation strip and
// the conflict tool's list of files.
func (a *App) ConflictStop(path string) (ConflictStop, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return ConflictStop{}, err
	}
	ctx := r.Git.Context()
	out := ConflictStop{Files: []ConflictTally{}, Settled: []DriverSettled{}, Drivers: []MergeDriver{}}
	st, err := r.Git.OperationState(ctx, r.GitDir)
	if err != nil || st.Op == "" {
		return out, err
	}
	drivers := map[string]string{}
	// Each file gets the patient read the conflict tool gives it, so the two agree; the tally
	// arrives on its own, after the files are listed.
	reading, cancel := context.WithTimeout(ctx, conflictTallyTime)
	defer cancel()
	for i, file := range st.Conflicts {
		t := ConflictTally{Path: file}
		if i < conflictTallyCap && reading.Err() == nil {
			if cf, err := r.Git.Conflict(reading, file); err == nil {
				places := placesOf(cf)
				read := readConflict(reading, r.Git, file, cf, places, semdiff.PatientRead)
				t.Places, t.Language, t.Driver = len(places), read.Language, read.Driver
				if read.Fallback != "" {
					t.Language = ""
				}
				for _, s := range read.Suggestions {
					if s.OK {
						t.Suggested++
					}
				}
				if read.Driver != "" {
					drivers[read.Driver] = read.DriverCommand
				}
			}
		}
		out.Files = append(out.Files, t)
	}
	// The files merged without a conflict whose attributes send them to a merge driver. A
	// driver only runs on a file both sides changed.
	clean := r.Git.MergedCleanly(ctx)
	attrs, _ := r.Git.AttributesOf(ctx, clean, "merge")
	var sent []string
	named := map[string]string{}
	for _, f := range clean {
		if d, cmd := mergeDriver(ctx, r.Git, attrs[f]["merge"]); d != "" && cmd != "" {
			sent = append(sent, f)
			named[f] = d
			drivers[d] = cmd
		}
	}
	both := r.Git.ChangedOnBothSides(ctx, r.GitDir, sent)
	for _, f := range sent {
		if both[f] {
			out.Settled = append(out.Settled, DriverSettled{Path: f, Driver: named[f]})
		}
	}
	for name, cmd := range drivers {
		out.Drivers = append(out.Drivers, MergeDriver{Name: name, Command: cmd})
	}
	slices.SortFunc(out.Drivers, func(a, b MergeDriver) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return out, nil
}

// combinable counts the places in a forecast's conflicted file whose sides changed
// different tokens: the conflict tool would suggest them.
func combinable(ctx context.Context, id string, texts []git.ConflictText) int {
	if id == "" {
		return 0
	}
	n := 0
	for _, t := range texts {
		if t.HasBase && semdiff.MergePlace(ctx, id, t.Base, t.Ours, t.Theirs).OK {
			n++
		}
	}
	return n
}
