package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Find in files (#979): where a text is in the files, now or at a commit. The Find panel asks;
// the lines reach it as "find:lines" events while git reads, and the call answers when the
// search is over. A newer search in the same repository stops the one before, and closing
// the repository stops it too.

// FindLines is a batch of a search's lines, sent as "find:lines".
type FindLines struct {
	Path string `json:"path"`
	// Token is the one the window gave the search, so it can drop a stopped search's lines.
	Token string `json:"token"`
	// Commit is the commit searched, as a full ID, or "" for the working tree.
	Commit string         `json:"commit"`
	Lines  []git.GrepLine `json:"lines"`
}

// FindResult is how a search ended.
type FindResult struct {
	// Commit is the commit searched, as a full ID, or "" for the working tree: a branch named
	// in the search is read once, so a line opens the file it was found in even after the
	// branch moves.
	Commit string `json:"commit"`
	Lines  int    `json:"lines"`
	Files  int    `json:"files"`
	// Truncated says it stopped at git.GrepLimit lines, with more to find.
	Truncated bool `json:"truncated"`
	// Ms is how long it took.
	Ms int64 `json:"ms"`
	// Superseded says a newer search, or the repository closing, stopped it. The lines it
	// sent stand; the window drops them by their token.
	Superseded bool `json:"superseded"`
}

// FindCounted is how many lines and files a search would find, for the search box's
// suggestion, or Superseded when a newer count stopped it.
type FindCounted struct {
	Lines      int  `json:"lines"`
	Files      int  `json:"files"`
	Superseded bool `json:"superseded"`
}

// findEvery is how often a running search sends the lines it has found since it last did:
// often enough to look live, seldom enough not to flood the window.
const findEvery = 50 * time.Millisecond

// FindInFiles searches the repository's files for o, sending what it finds as "find:lines"
// events tagged with token, and answers with how the search ended.
func (a *App) FindInFiles(path, token string, o git.GrepOptions) (FindResult, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return FindResult{}, err
	}
	ctx, done := a.latest.start("find", r.Path)
	defer done()
	start := time.Now()
	named := o.Rev
	if named != "" {
		sha, err := r.Git.ReadString(ctx, "rev-parse", "--verify", "--quiet", "--end-of-options", named+"^{commit}")
		if errors.Is(err, context.Canceled) {
			return FindResult{Superseded: true}, nil
		}
		if err != nil || sha == "" {
			return FindResult{}, findError(git.ErrUnknownRevision, named)
		}
		o.Rev = sha
	}

	var mu sync.Mutex
	var pending []git.GrepLine
	send := func() {
		mu.Lock()
		lines := pending
		pending = nil
		mu.Unlock()
		if len(lines) > 0 {
			a.emit("find:lines", FindLines{Path: r.Path, Token: token, Commit: o.Rev, Lines: lines})
		}
	}
	tick := time.NewTicker(findEvery)
	stop := make(chan struct{})
	var sender sync.WaitGroup
	sender.Go(func() {
		for {
			select {
			case <-tick.C:
				send()
			case <-stop:
				return
			}
		}
	})
	sum, err := r.Git.Grep(ctx, o, git.GrepLimit, func(l git.GrepLine) {
		mu.Lock()
		pending = append(pending, l)
		mu.Unlock()
	})
	tick.Stop()
	close(stop)
	sender.Wait() // so the last lines go after every batch before them
	send()

	res := FindResult{Commit: o.Rev, Lines: sum.Lines, Files: sum.Files, Truncated: sum.Truncated, Ms: time.Since(start).Milliseconds()}
	if errors.Is(err, context.Canceled) {
		res.Superseded = true
		return res, nil
	}
	return res, findError(err, named)
}

// FindCount counts what FindInFiles would find, without the lines. A newer count in the same
// repository stops this one.
func (a *App) FindCount(path string, o git.GrepOptions) (FindCounted, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return FindCounted{}, err
	}
	ctx, done := a.latest.start("findcount", r.Path)
	defer done()
	sum, err := r.Git.GrepCount(ctx, o)
	if errors.Is(err, context.Canceled) {
		return FindCounted{Superseded: true}, nil
	}
	if err != nil {
		return FindCounted{}, findError(err, o.Rev)
	}
	return FindCounted{Lines: sum.Lines, Files: sum.Files}, nil
}

// findError says why a search couldn't run, in words.
func findError(err error, rev string) error {
	var pe *git.PatternError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &pe):
		return userError("Not a regular expression: " + pe.Reason + ". Turn off Regular expression to search for it as written.")
	case errors.Is(err, git.ErrUnknownRevision):
		return userError("There's no branch, tag or commit called “" + rev + "”.")
	case errors.Is(err, git.ErrNotDownloaded):
		return userError("This is a partial clone, and some of " + rev + "'s files were never downloaded, so it can't be searched. A search never downloads anything.")
	case errors.Is(err, git.ErrLazyFetch):
		return userError("This is a partial clone, and your git, older than 2.44, would download " + rev + "'s files to search them. Search the working tree, or update git.")
	case errors.Is(err, git.ErrFindText):
		return userError("Find looks for text within one line.")
	}
	return err
}
