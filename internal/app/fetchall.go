package app

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// fetchRemotes fetches remote, or every remote when it's "", and returns the remotes that
// were fetched. Git calls a fetch of every remote failed when any one of them fails, though
// the others are up to date: here the ones it reached are returned, along with a Problem
// that names the ones it didn't (#806). With nothing fetched, the error is git's own.
func fetchRemotes(ctx context.Context, g *git.Repo, remote string, o git.NetOptions) ([]string, error) {
	err := g.Fetch(ctx, remote, o)
	switch {
	case err == nil:
		return remotesFor(ctx, g, remote), nil
	case remote != "" || ctx.Err() != nil:
		return nil, err
	}
	failed := git.FetchFailures(err)
	asked, lerr := g.FetchAllRemotes(ctx)
	if len(failed) == 0 || lerr != nil {
		return nil, err
	}
	// In the order the remotes are listed everywhere else: git reports them as they finish.
	var reached []string
	var missed []git.FetchFailure
	for _, name := range asked {
		if i := slices.IndexFunc(failed, func(f git.FetchFailure) bool { return f.Remote == name }); i >= 0 {
			missed = append(missed, failed[i])
		} else {
			reached = append(reached, name)
		}
	}
	if len(reached) == 0 || len(missed) == 0 {
		return nil, err
	}
	return reached, partFetched(err, missed, len(reached))
}

// partFetched is the Problem for a fetch that brought in some remotes and not others. It
// says which, and that the rest were fetched; remotes with no repository behind them any
// more can be removed from here.
func partFetched(err error, missed []git.FetchFailure, reached int) error {
	p := &Problem{Actions: []ProblemAction{}}
	var gerr *git.Error
	if errors.As(err, &gerr) {
		p.Output, p.Hidden = tidyOutput(gerr.Stderr)
		p.Full = strings.TrimSpace(gerr.Stderr)
	}
	var gone, other []string
	for _, f := range missed {
		if noRepository(f.Output) {
			gone = append(gone, f.Remote)
		} else {
			other = append(other, f.Remote)
		}
	}

	verb := "reach"
	if len(other) == 0 {
		verb = "find"
	}
	if len(missed) == 1 {
		p.Title = "Fetch couldn't " + verb + " the remote " + missed[0].Remote
	} else {
		p.Title = "Fetch couldn't " + verb + " " + strconv.Itoa(len(missed)) + " of your " + strconv.Itoa(len(missed)+reached) + " remotes"
	}

	var detail []string
	switch {
	case len(gone) == 1 && len(missed) == 1:
		detail = append(detail, "No repository was found at its URL. It may have been deleted, or your account can't see it.")
	case len(gone) == 1:
		detail = append(detail, "No repository was found for "+gone[0]+". It may have been deleted, or your account can't see it.")
	case len(gone) > 1:
		detail = append(detail, "No repository was found for "+someOf(gone)+". They may have been deleted, or your account can't see them.")
	}
	switch {
	case len(other) == 1 && len(missed) == 1:
		// Why, in the words a fetch of that remote alone would use.
		var why *problemErr
		if errors.As(netProblem("", &git.Error{Stderr: missed[0].Output}), &why) {
			detail = append(detail, why.p.Title+".")
		}
	case len(other) == 1:
		detail = append(detail, other[0]+" wasn't fetched: Details has what git said.")
	case len(other) > 1:
		detail = append(detail, someOf(other)+" weren't fetched: Details has what git said.")
	}
	if reached == 1 {
		detail = append(detail, "Your other remote was fetched.")
	} else {
		detail = append(detail, "Your other "+strconv.Itoa(reached)+" remotes were fetched.")
	}
	p.Detail = strings.Join(detail, " ")

	if len(gone) > 0 {
		label := "Remove them…"
		switch {
		case len(gone) == 1:
			label = "Remove " + gone[0] + "…"
		case len(other) > 0:
			label = "Remove the " + strconv.Itoa(len(gone)) + " not found…"
		}
		p.Actions = append(p.Actions, ProblemAction{Kind: "remove-remotes", Label: label, Names: gone})
	}
	if len(other) > 0 {
		p.Actions = append(p.Actions, ProblemAction{Kind: "retry", Label: "Try again"})
	}
	return &problemErr{p}
}

// noRepository says whether git's output is a server or a folder with no repository at the
// address it was given.
func noRepository(s string) bool {
	return strings.Contains(s, "Repository not found") || strings.Contains(s, "' does not exist") ||
		strings.Contains(s, "does not appear to be a git repository") ||
		strings.Contains(s, "repository '") && strings.Contains(s, "' not found")
}

// someOf lists names for a sentence, the first three by name: "a, b, c and 4 more".
func someOf(names []string) string {
	switch n := len(names); {
	case n == 0:
		return ""
	case n == 1:
		return names[0]
	case n <= 3:
		return strings.Join(names[:n-1], ", ") + " and " + names[n-1]
	default:
		return strings.Join(names[:3], ", ") + " and " + strconv.Itoa(n-3) + " more"
	}
}
