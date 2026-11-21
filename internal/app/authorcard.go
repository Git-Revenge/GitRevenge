package app

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// The graph's card about an author asks two things, apart, because one is quick and the
// other can take a while: who wrote this commit (one object and the mailmap), and how much
// that person wrote (the whole history, read once and kept; see historyPeople).

// AuthorCard is who wrote one commit.
type AuthorCard struct {
	// Shown is who the mailmap reads the author as: the name the graph shows.
	Shown git.Identity `json:"shown"`
	// Written is the author as the commit has it, which no mailmap changes.
	Written git.Identity `json:"written"`
	// Lines are the mailmap lines that read Written as Shown; none when the commit is shown
	// as written.
	Lines []git.MailmapRef `json:"lines"`
	// Bot marks an author the bot author patterns match.
	Bot bool `json:"bot"`
}

// AuthorCard returns who wrote the commit id: as it says, and as the mailmap git reads
// shows them. A repository with no mailmap answers the same, with no lines.
func (a *App) AuthorCard(path, id string) (AuthorCard, error) {
	empty := AuthorCard{Lines: []git.MailmapRef{}}
	r, err := a.openRepo(path)
	if err != nil {
		return empty, err
	}
	c, err := git.ReadCommit(r.Objects, id)
	if errors.Is(err, git.ErrObjectMissing) {
		return empty, userError("That commit no longer exists.")
	}
	if err != nil {
		return empty, err
	}
	m, err := r.Git.ReadMailmap(context.Background(), "", false)
	if err != nil {
		return empty, err
	}
	written := git.Identity{Name: c.Author.Name, Email: c.Author.Email}
	shown, by := m.Resolve(written)
	if by == nil {
		by = []git.MailmapRef{}
	}
	return AuthorCard{Shown: shown, Written: written, Lines: by, Bot: a.branchRules(r.Path).BotAuthor(shown.Name, shown.Email)}, nil
}

// PersonCard returns one person as the saved mailmap reads the history: how much they
// wrote, and every name and address read as them. Someone the history doesn't name, such as
// the author of a commit no branch reaches, comes back with nothing counted.
func (a *App) PersonCard(path string, who git.Identity) (Person, error) {
	p := Person{Name: who.Name, Email: who.Email, Identities: []PersonIdentity{}}
	r, err := a.openRepo(path)
	if err != nil {
		return p, err
	}
	ctx := context.Background()
	history, err := a.historyPeople(ctx, r)
	if err != nil {
		return p, err
	}
	m, err := r.Git.ReadMailmap(ctx, "", false)
	if err != nil {
		return p, err
	}
	p.Bot = a.branchRules(r.Path).BotAuthor(who.Name, who.Email)
	for _, h := range history.ids {
		shown, by := m.Resolve(h.Identity)
		if shown != who {
			continue
		}
		if by == nil {
			by = []git.MailmapRef{}
		}
		p.Commits += h.Authored
		p.CoAuthored += h.CoAuthored
		p.Identities = append(p.Identities, PersonIdentity{Name: h.Name, Email: h.Email, Commits: h.Authored, CoAuthored: h.CoAuthored, Lines: by})
	}
	slices.SortStableFunc(p.Identities, func(x, y PersonIdentity) int {
		return cmp.Compare(y.Commits+y.CoAuthored, x.Commits+x.CoAuthored)
	})
	return p, nil
}
