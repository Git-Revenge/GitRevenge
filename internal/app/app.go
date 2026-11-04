// Package app is the API surface bound into the Wails frontend.
package app

import (
	"context"
	"errors"
	"sync"

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

	net  network
	undo undoStacks
	// upstreams remembers where branches' upstreams were before fetches prune them, for
	// cleanup on pull.
	upstreams upstreamMemory
	// promptHook answers credential prompts in tests instead of the UI.
	promptHook func(text string, kind askpass.Kind) (string, bool)
	// emitHook receives events in tests, where there is no Wails runtime to send them to.
	emitHook func(name string, data any)
	// updateURL overrides the GitHub releases endpoint; tests use it.
	updateURL string
	searches  searches
	sigs      sigCache
	configs   gitConfigs
	conflicts conflictChecks
	insights  insightsCache
	// configBackupDir overrides where config backups are kept; tests use it.
	configBackupDir string
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
}

// Shutdown is called by Wails when the app quits. It stops background git processes.
func (a *App) Shutdown(context.Context) {
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
