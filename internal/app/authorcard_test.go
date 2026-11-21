package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// A repository with no mailmap answers as its commits are written, and reading makes no file.
func TestAuthorCardWithNoMailmap(t *testing.T) {
	t.Parallel()
	a, fx, path := peopleRepo(t)
	fx.Git("checkout", "-q", "HEAD~1") // Priya R's commit
	priyaR := git.Identity{Name: "Priya R", Email: "priya@oldmill.example"}

	card, err := a.AuthorCard(path, fx.Head())
	if err != nil {
		t.Fatal(err)
	}
	if card.Shown != priyaR || card.Written != priyaR || len(card.Lines) != 0 || card.Bot {
		t.Fatalf("card = %+v, want the commit's own author and no lines", card)
	}
	p, err := a.PersonCard(path, card.Shown)
	if err != nil {
		t.Fatal(err)
	}
	if p.Commits != 1 || p.CoAuthored != 0 || len(p.Identities) != 1 {
		t.Fatalf("person = %+v, want one commit under one identity", p)
	}
	if _, err := os.Stat(filepath.Join(fx.Dir, ".mailmap")); err == nil {
		t.Fatal("asking who wrote a commit created a .mailmap")
	}
}

// With a line that joins her old identity, the card shows who she's read as, what the commit
// says, and the line that decides it; the count is everything read as her.
func TestAuthorCardThroughTheMailmap(t *testing.T) {
	t.Parallel()
	a, fx, path := peopleRepo(t)
	fx.WriteFile(".mailmap", "# Tidewater\nPriya Raman <priya@tidewater.example> <priya@oldmill.example>\n")
	old := fx.Git("rev-parse", "HEAD~1")
	priya := git.Identity{Name: "Priya Raman", Email: "priya@tidewater.example"}

	card, err := a.AuthorCard(path, old)
	if err != nil {
		t.Fatal(err)
	}
	if card.Shown != priya || card.Written != (git.Identity{Name: "Priya R", Email: "priya@oldmill.example"}) {
		t.Fatalf("card = %+v", card)
	}
	if len(card.Lines) != 1 || card.Lines[0].Source != git.MailmapWorkTree || card.Lines[0].Line != 2 {
		t.Fatalf("lines = %+v, want line 2 of .mailmap", card.Lines)
	}
	// What git itself says, since the card must match the graph.
	if got, err := gitRepo(t, a, path).Mailmap(t.Context(), []git.Identity{card.Written}); err != nil || got[0] != card.Shown {
		t.Fatalf("git reads the author as %+v (%v), the card as %+v", got, err, card.Shown)
	}

	p, err := a.PersonCard(path, card.Shown)
	if err != nil {
		t.Fatal(err)
	}
	if p.Commits != 2 || len(p.Identities) != 2 || p.Bot {
		t.Fatalf("person = %+v, want both identities and two commits", p)
	}
}

func TestPersonCardCountsCreditsAndMarksBots(t *testing.T) {
	t.Parallel()
	a, _, path := peopleRepo(t)

	// Sarah never wrote a commit here; one credits her.
	sarah, err := a.PersonCard(path, git.Identity{Name: "Sarah Lind", Email: "sarah@tidewater.example"})
	if err != nil || sarah.Commits != 0 || sarah.CoAuthored != 1 {
		t.Fatalf("Sarah = %+v, %v", sarah, err)
	}
	bot, err := a.PersonCard(path, git.Identity{Name: "dependabot[bot]", Email: "bot@users.noreply.example"})
	if err != nil || !bot.Bot || bot.Commits != 1 {
		t.Fatalf("the bot = %+v, %v", bot, err)
	}
	// Someone the history doesn't name: nothing counted, and no error.
	nobody, err := a.PersonCard(path, git.Identity{Name: "Nobody", Email: "nobody@example.com"})
	if err != nil || nobody.Commits != 0 || nobody.CoAuthored != 0 || nobody.Identities == nil || len(nobody.Identities) != 0 {
		t.Fatalf("nobody = %+v, %v", nobody, err)
	}
}

func TestAuthorCardForACommitThatIsGone(t *testing.T) {
	t.Parallel()
	a, _, path := peopleRepo(t)
	if _, err := a.AuthorCard(path, "0123456789012345678901234567890123456789"); err == nil {
		t.Fatal("a commit that doesn't exist gave a card")
	}
}

// gitRepo is the git side of an open repository, for asking git what it would say.
func gitRepo(t *testing.T, a *App, path string) *git.Repo {
	t.Helper()
	r, err := a.openRepo(path)
	if err != nil {
		t.Fatal(err)
	}
	return r.Git
}
