// Package app is the API surface bound into the Wails frontend.
package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/git-revenge/gitrevenge/internal/askpass"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
	"github.com/git-revenge/gitrevenge/internal/version"
)

// App holds application-wide state. Its exported methods are callable from the frontend.
type App struct {
	ctx context.Context
	// settingsPath overrides the settings file location; tests use it.
	settingsPath string

	envOnce sync.Once
	env     Environment

	sessOnce sync.Once
	sess     *session.Manager
	sessErr  error

	net network
	// halfCloned holds clones made with only some folders, between the history arriving and
	// the folders being chosen (#1004).
	halfCloned halfClones
	undo       undoStacks
	// problems holds the last structured error, until the frontend takes it.
	problems problems
	// upstreams remembers where branches' upstreams were before fetches prune them, for
	// cleanup on pull.
	upstreams upstreamMemory
	// trustAsk holds the question to put to the user about a repository (see TakeTrustQuestion).
	trustAsk trustQuestions
	// chosen holds the files picked in a file dialog (see chosenFiles).
	chosen chosenFiles
	// patches holds the files of patches open in previews (#1022).
	patches patchSets
	// applied holds each repository's last diff applied, for its Undo (#1024).
	applied appliedDiffs
	// describing holds each repository's describe in flight, so a newer one stops it (#940).
	describing describeCalls
	// versionCap, when set, stands in for maxVersionCommits in tests (#1041).
	versionCap int
	// promptHook answers credential prompts in tests instead of the UI.
	promptHook func(text string, kind askpass.Kind) (string, bool)
	// emitHook receives events in tests, where there is no Wails runtime to send them to.
	emitHook func(name string, data any)
	// clipboardHook reads the clipboard in tests (#1025).
	clipboardHook func() (string, error)
	// updateURL overrides the GitHub releases endpoint; tests use it.
	updateURL string
	latest    latest
	sigs      sigCache
	configs   gitConfigs
	conflicts conflictChecks
	// diffs keeps recent comparisons for the diff views (see FileDiff).
	diffs    diffCache
	insights insightsCache
	// mapAuthors holds the Map's Authors mode results, by repository, window and ref state.
	mapAuthors insightsCache
	// people holds who each repository's history names, for the People view (see people.go).
	people peopleReads
	// trailerReadings keeps each open repository's trailers as read through its rules.
	trailerReadings trailerReadings
	// trailerFilters holds each open repository's smart filter, for the session.
	trailerFilters trailerFilters
	// historyNoted is when each running history search last told the window, so it does
	// so at most once a historyEvery (see historysearch.go).
	historyNotes sync.Mutex
	historyNoted map[*historyRun]time.Time
	// historyChunkSize overrides historyChunk; tests set it, for searches that run long
	// enough to stop.
	historyChunkSize int
	// historyCountHold, when set, holds a history search's count until it's closed or the
	// search ends: tests use it to end a search before its count (#1052).
	historyCountHold chan struct{}
	// trailerPins keeps what each open repository's config says about its trailer keys.
	trailerPins trailerPins
	// trailerCares keeps the questions each open repository's trailers raise.
	trailerCares trailerCares
	// configBackupDir overrides where config backups are kept; tests use it.
	configBackupDir string
	// repoConfigMu orders reads of a repository's settings from git config with writes to
	// them, so a read that started earlier can't put back what a write just changed.
	repoConfigMu sync.Mutex
	// background counts work started off a caller's path, which Shutdown waits for. Once
	// closing is set, under backgroundMu, no more is started.
	background   sync.WaitGroup
	backgroundMu sync.Mutex
	closing      bool
	// running is how much of that work hasn't finished, under backgroundMu. The group can
	// only be waited on once nothing more will be added, which is Shutdown; anything that
	// wants to know the work so far is done, while more may still start, reads this.
	running int
}

// inBackground runs fn off the caller's path, unless the app is shutting down. Shutdown
// waits for whatever was started.
func (a *App) inBackground(fn func()) {
	a.backgroundMu.Lock()
	if a.closing {
		a.backgroundMu.Unlock()
		return
	}
	a.background.Add(1)
	a.running++
	a.backgroundMu.Unlock()
	go func() {
		defer a.background.Done()
		defer func() {
			a.backgroundMu.Lock()
			a.running--
			a.backgroundMu.Unlock()
		}()
		fn()
	}()
}

// idle reports whether no background work is running. More can start straight after.
func (a *App) idle() bool {
	a.backgroundMu.Lock()
	defer a.backgroundMu.Unlock()
	return a.running == 0
}

// Environment reports the git installation the app will use.
type Environment struct {
	GitPath    string `json:"gitPath"`
	GitVersion string `json:"gitVersion"`
	LFSVersion string `json:"lfsVersion"`
	// Problem is a user-facing explanation when git is missing or too old; empty when usable.
	Problem    string `json:"problem"`
	MinVersion string `json:"minVersion"`
}

// New creates the App.
func New() *App {
	return &App{}
}

// Startup is called by Wails when the app starts. The context is kept for runtime calls.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	// Files dropped on the window (#1025): main.go turns Wails' file drop on.
	runtime.OnFileDrop(ctx, func(_, _ int, paths []string) { a.filesDropped(paths) })
}

// Shutdown is called by Wails when the app quits. It stops background git processes.
func (a *App) Shutdown(context.Context) {
	a.backgroundMu.Lock()
	a.closing = true
	a.backgroundMu.Unlock()
	a.background.Wait()
	if a.sess != nil {
		a.sess.CloseAll()
	}
	a.configs.close()
	a.conflicts.stopAll()
	if a.net.server != nil {
		_ = a.net.server.Close()
	}
}

func discover(ctx context.Context, override string) Environment {
	env := Environment{MinVersion: git.MinVersion.String()}
	in, err := git.Discover(ctx)
	if override != "" {
		in, err = git.Probe(ctx, override)
	}
	if in != nil {
		env.GitPath = in.Path
		env.GitVersion = in.Version.String()
		env.LFSVersion = in.LFS
	}
	var tooOld *git.TooOldError
	switch {
	case err == nil:
	case errors.Is(err, git.ErrNotFound) && override != "":
		env.Problem = "The git set in Settings wasn't found at " + override + ". Fix the path in settings.json or remove gitPath from it."
	case errors.Is(err, git.ErrNotFound):
		env.Problem = "Git Revenge needs git, but no git installation was found."
	case errors.As(err, &tooOld):
		env.Problem = tooOld.Error()
	default:
		env.Problem = "Git could not be started: " + err.Error()
	}
	return env
}

// Version returns the application version.
func (a *App) Version() string {
	return version.Version
}

// Environment returns the git installation details. Discovery runs once, on first call,
// so it never races Wails' startup ordering.
func (a *App) Environment() Environment {
	a.envOnce.Do(func() { a.env = discover(context.Background(), a.gitOverride()) })
	return a.env
}
