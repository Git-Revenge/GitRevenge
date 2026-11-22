package git

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestParseMailmap(t *testing.T) {
	text := "# People\n" +
		"\n" +
		"Jane Doe <jdoe@old.example>\n" +
		"<jane@example.com> <jane@work.example>\n" +
		"Jane Doe <jane@example.com> <jd@home.example>\n" +
		"Jane Doe <jane@example.com> jdoe <jd@home.example>\n" +
		"Leo Brandt leo@example.com\n" +
		"Sam <>\n" +
		"Sam <sam@example.com\n" +
		"   # indented\n" +
		"  Padded Name   <pad@example.com>  \r\n"
	got := ParseMailmap(text)
	want := []MailmapLine{
		{Line: 1, Kind: MailmapComment},
		{Line: 2, Kind: MailmapBlank},
		{Line: 3, Kind: MailmapMapping, NewName: "Jane Doe", OldEmail: "jdoe@old.example"},
		{Line: 4, Kind: MailmapMapping, NewEmail: "jane@example.com", hasNewEmail: true, OldEmail: "jane@work.example"},
		{Line: 5, Kind: MailmapMapping, NewName: "Jane Doe", NewEmail: "jane@example.com", hasNewEmail: true, OldEmail: "jd@home.example"},
		{Line: 6, Kind: MailmapMapping, NewName: "Jane Doe", NewEmail: "jane@example.com", hasNewEmail: true, OldName: "jdoe", hasOldName: true, OldEmail: "jd@home.example"},
		{Line: 7, Kind: MailmapSkipped, Why: "no address in < >"},
		{Line: 8, Kind: MailmapSkipped, Why: "the address is empty"},
		{Line: 9, Kind: MailmapSkipped, Why: "the address has no closing >"},
		{Line: 10, Kind: MailmapComment},
		{Line: 11, Kind: MailmapMapping, NewName: "Padded Name", OldEmail: "pad@example.com"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v\nwant     %+v", i+1, got[i], want[i])
		}
	}
	if ParseMailmap("") != nil || ParseMailmap("\n") != nil {
		t.Error("empty text should have no lines")
	}
}

// The reader has to agree with git, or the People view would promise something a save
// doesn't deliver. Each text is saved as the repository's .mailmap and every identity is
// asked of both.
func TestMailmapResolvesAsGitDoes(t *testing.T) {
	t.Parallel()
	ids := []Identity{
		{Name: "jdoe", Email: "jdoe@old.example"},
		{Name: "JDoe", Email: "JDOE@OLD.example"},
		{Name: "Jane", Email: "jane@work.example"},
		{Name: "jd", Email: "jd@home.example"},
		{Name: "JDOE", Email: "jd@home.example"},
		{Name: "someone else", Email: "jd@home.example"},
		{Name: "Sam Ortiz", Email: "sam@example.com"},
		{Name: "sam", Email: "sam@example.com"},
		{Name: "Nobody", Email: "nobody@example.com"},
		{Name: "Ünïcode Ñame", Email: "uni@example.com"},
		{Name: "ünïcode ñame", Email: "uni@example.com"},
		{Name: "Chain", Email: "first@example.com"},
	}
	texts := map[string]string{
		"a name for an address":           "Jane Doe <jdoe@old.example>\n",
		"an address for an address":       "<jane@example.com> <jane@work.example>\n",
		"both for an address":             "Jane Doe <jane@example.com> <jd@home.example>\n",
		"a name and an address matched":   "Jane Doe <jane@example.com> jdoe <jd@home.example>\n",
		"named lines beside a plain one":  "Plain <plain@example.com> <jd@home.example>\nJane Doe <jane@example.com> jdoe <jd@home.example>\n",
		"a named line then a plain one":   "Jane Doe <jane@example.com> jdoe <jd@home.example>\nPlain <plain@example.com> <jd@home.example>\n",
		"a later line replaces the name":  "First <jdoe@old.example>\nSecond <jdoe@old.example>\n",
		"name and address from two lines": "Jane Doe <jdoe@old.example>\n<jane@example.com> <jdoe@old.example>\n",
		"address then name":               "<jane@example.com> <jdoe@old.example>\nJane Doe <jdoe@old.example>\n",
		"a full line then a name":         "Jane Doe <jane@example.com> <jdoe@old.example>\nJ. Doe <jdoe@old.example>\n",
		"a named line replaced whole":     "Jane Doe <jane@example.com> jdoe <jd@home.example>\nJ. D. <jd@home.example> JDOE <jd@home.example>\n",
		"upper case in the file":          "Jane Doe <JDOE@OLD.EXAMPLE>\nSam O. <sam@example.com> SAM <SAM@example.com>\n",
		"names beyond ASCII":              "Uni <uni2@example.com> ünïcode ñame <uni@example.com>\n",
		"nothing is followed twice":       "Chain <second@example.com> <first@example.com>\nEnd <third@example.com> <second@example.com>\n",
		"comments, blanks and bad lines":  "# people\n\nLeo leo@example.com\nSam <>\n  # indented\nJane Doe <jdoe@old.example>\n",
		"text after the second address":   "Jane Doe <jane@example.com> <jdoe@old.example> # since 2024\n",
		"spaces around a name":            "   Jane   Doe   <jane@example.com>    jdoe   <jd@home.example>\n",
		"Windows line endings":            "Jane Doe <jdoe@old.example>\r\n<jane@example.com> <jane@work.example>\r\n",
		"an empty mailmap":                "",
	}
	for name, text := range texts {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := gittest.New(t)
			fx.Linear(1)
			if text != "" {
				fx.WriteFile(".mailmap", text)
			}
			r := OpenRepo(newTestRunner(t), fx.Dir)
			want, err := r.Mailmap(context.Background(), ids)
			if err != nil {
				t.Fatal(err)
			}
			m := NewMailmap()
			m.Add(MailmapWorkTree, text)
			for i, id := range ids {
				got, by := m.Resolve(id)
				if got != want[i] {
					t.Errorf("%s <%s> reads as %s <%s>; git says %s <%s>", id.Name, id.Email, got.Name, got.Email, want[i].Name, want[i].Email)
				}
				if (got != id) && len(by) == 0 {
					t.Errorf("%s <%s> changed with no line named for it", id.Name, id.Email)
				}
			}
		})
	}
}

// mailmap.file is read after the work tree's .mailmap and replaces what it says.
func TestMailmapSourcesLayerAsGitDoes(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.Linear(1)
	fx.WriteFile(".mailmap", "Shared Name <shared@example.com> <jdoe@old.example>\nKept <kept@example.com> <sam@example.com>\n")
	fx.WriteFile("mine.txt", "My Name <jdoe@old.example>\n")
	fx.Git("config", "mailmap.file", "mine.txt")
	r := OpenRepo(newTestRunner(t), fx.Dir)
	ctx := context.Background()
	ids := []Identity{{Name: "jdoe", Email: "jdoe@old.example"}, {Name: "sam", Email: "sam@example.com"}}

	want, err := r.Mailmap(ctx, ids)
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.ReadMailmap(ctx, "", false)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		if got, _ := m.Resolve(id); got != want[i] {
			t.Errorf("%s reads as %+v; git says %+v", id.Name, got, want[i])
		}
	}
	// The name comes from the user's own file, the address from the shared one.
	got, by := m.Resolve(ids[0])
	if got != (Identity{Name: "My Name", Email: "shared@example.com"}) {
		t.Fatalf("jdoe = %+v", got)
	}
	if len(by) != 2 || by[0] != (MailmapRef{Source: MailmapWorkTree, Line: 1}) || by[1] != (MailmapRef{Source: MailmapFile, Line: 1}) {
		t.Fatalf("decided by %+v, want the shared file for the address and mailmap.file for the name", by)
	}

	// Text being edited stands in for the saved .mailmap, and the other source still counts.
	m, err = r.ReadMailmap(ctx, "Edited <edited@example.com> <sam@example.com>\n", true)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Resolve(ids[1]); got != (Identity{Name: "Edited", Email: "edited@example.com"}) {
		t.Errorf("with unsaved text, sam = %+v", got)
	}
	if got, _ := m.Resolve(ids[0]); got != (Identity{Name: "My Name", Email: "jdoe@old.example"}) {
		t.Errorf("with unsaved text, jdoe = %+v, want only what mailmap.file says", got)
	}
}

func TestMailmapNotes(t *testing.T) {
	text := "# Tidewater\n" +
		"Priya Raman <priya@tidewater.example> <priya@oldmill.example>\n" +
		"Jo Whitlock <jo@tidewater.example> <jo@harbour.example>\n" +
		"Leo Brandt leo@tidewater.example\n" +
		"First <sam@tidewater.example>\n" +
		"Sam Ortiz <sam@tidewater.example>\n" +
		"Maya Okafor <maya@tidewater.example>\n"
	history := []HistoryIdentity{
		{Identity: Identity{Name: "Priya R", Email: "priya@oldmill.example"}, Authored: 9, CoAuthored: 2},
		{Identity: Identity{Name: "priya", Email: "PRIYA@oldmill.example"}, Authored: 1},
		{Identity: Identity{Name: "sam", Email: "sam@tidewater.example"}, Authored: 3},
		{Identity: Identity{Name: "Sam Ortiz", Email: "sam@tidewater.example"}, Authored: 51},
		{Identity: Identity{Name: "Maya Okafor", Email: "maya@tidewater.example"}, Authored: 71},
	}
	m := NewMailmap()
	m.Add(MailmapWorkTree, text)
	notes := m.Notes(MailmapWorkTree, text, history)
	if len(notes) != 7 {
		t.Fatalf("got %d notes", len(notes))
	}
	type row struct {
		kind                            string
		identities, commits, coAuthored int
		replaced                        bool
		same                            int
	}
	want := []row{
		{kind: MailmapComment},
		{kind: MailmapMapping, identities: 2, commits: 10, coAuthored: 2},
		{kind: MailmapMapping}, // matches nobody now
		{kind: MailmapSkipped},
		{kind: MailmapMapping, replaced: true}, // the next line names the same address
		// Renames "sam", and matches "Sam Ortiz", who is already spelled that way.
		{kind: MailmapMapping, identities: 1, commits: 3, same: 1},
		{kind: MailmapMapping, same: 1}, // maps Maya to herself: applies, and joins nothing
	}
	for i, w := range want {
		n := notes[i]
		if n.Kind != w.kind || n.Identities != w.identities || n.Commits != w.commits || n.CoAuthored != w.coAuthored || n.Replaced != w.replaced || n.Same != w.same {
			t.Errorf("line %d = kind %s, %d identities, %d commits, %d co-authored, replaced %v, same %d\nwant %+v", i+1, n.Kind, n.Identities, n.Commits, n.CoAuthored, n.Replaced, n.Same, w)
		}
	}
	if notes[3].Why == "" {
		t.Error("a skipped line should say why")
	}
}

func TestMailmapMappedTo(t *testing.T) {
	t.Parallel()
	m := NewMailmap()
	m.Add(MailmapWorkTree, strings.Join([]string{
		"Jane Doe <jane@example.com> <JDoe@old.example.com>",
		"Jane Doe <jane@example.com> jdoe <team@example.com>",
		"Robin <robin@example.com> <robin@old.example.com>",
		"Jane D. <jane@example.com>",
		"<jane@work.example.com> <jane@example.com>",
	}, "\n"))
	hers := map[string]bool{"jane@example.com": true}
	if got, want := m.mappedTo(hers), []string{"jdoe@old.example.com", "team@example.com"}; !slices.Equal(got, want) {
		t.Errorf("mappedTo = %q, want %q", got, want)
	}
	if got := m.mappedTo(map[string]bool{"nobody@example.com": true}); len(got) != 0 {
		t.Errorf("mappedTo for someone no line names = %q", got)
	}
	var none *Mailmap
	if got := none.mappedTo(hers); got != nil {
		t.Errorf("no mailmap maps %q", got)
	}
}
