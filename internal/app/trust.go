package app

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/trust"
)

// untrustedSuffix ends the message for a repository that wasn't opened because it would run
// programs of its own. The frontend recognises the refusal by asking TakeTrustQuestion.
const untrustedSuffix = " has git settings or hooks of its own that run programs, and you haven't said it may."

// TrustQuestion is what to ask before opening a repository whose own config or hooks would
// make git run programs. It must not be an error: see Problem.
type TrustQuestion struct {
	// Path is the repository's root; Name its folder's name.
	Path string `json:"path"`
	Name string `json:"name"`
	// Findings are the settings and hooks that would run something.
	Findings []trust.Finding `json:"findings"`
}

// trustQuestions keeps the question from the last repository that was refused.
type trustQuestions struct {
	mu   sync.Mutex
	last *TrustQuestion
}

// askTrust keeps the question to ask when err is a repository refused for lack of trust,
// and returns err as it came.
func (a *App) askTrust(err error) error {
	var untrusted *session.UntrustedError
	if errors.As(err, &untrusted) {
		a.trustAsk.mu.Lock()
		a.trustAsk.last = &TrustQuestion{Path: untrusted.Root, Name: filepath.Base(untrusted.Root), Findings: untrusted.Findings}
		a.trustAsk.mu.Unlock()
	}
	return err
}

// TakeTrustQuestion returns the question for the repository that was just refused, once, or
// nil when the last failure wasn't about trust. The frontend calls it when opening fails,
// shows what the repository would run, and calls TrustRepo if the user agrees.
func (a *App) TakeTrustQuestion() *TrustQuestion {
	a.trustAsk.mu.Lock()
	defer a.trustAsk.mu.Unlock()
	q := a.trustAsk.last
	a.trustAsk.last = nil
	return q
}

// TrustRepo records that the user agrees the repository at path may run what its config and
// hooks name. It doesn't open it.
func (a *App) TrustRepo(path string) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	_, err = m.Trust(context.Background(), path)
	return friendly(err)
}

// trustNew trusts a repository the app has just made itself, by cloning or creating it.
func (a *App) trustNew(dir string) {
	if m, err := a.sessions(); err == nil {
		_, _ = m.Trust(context.Background(), dir)
	}
}
