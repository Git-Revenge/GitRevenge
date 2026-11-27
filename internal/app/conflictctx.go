package app

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/git-revenge/gitrevenge/internal/conflictlog"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/session"
)

// ConflictContext is what the conflict tool shows around the file being resolved (issue #597).
type ConflictContext struct {
	// MergeTool is the user's merge.tool, or "" when none is set.
	MergeTool string `json:"mergeTool"`
	// Clean are the files the operation changed without a conflict, up to a cap.
	Clean []string `json:"clean"`
	// Rerere is what git's reuse of recorded resolutions did here, or could do (#938).
	Rerere ConflictRerere `json:"rerere"`
}

// ConflictRerere is rerere's part in the conflict in progress.
type ConflictRerere struct {
	// On says git reuses recorded resolutions here.
	On bool `json:"on"`
	// Resolved are the conflicted files rerere settled with a resolution recorded before: still
	// conflicted, or staged with rerere.autoUpdate, when they'd otherwise read as clean.
	Resolved []string `json:"resolved"`
	// Recurring, with rerere off, are conflicted files whose conflict was resolved in the app
	// before: the offer to turn it on. It's made once for each repository.
	Recurring []string `json:"recurring"`
}

// maxRecurringFiles caps how many conflicted files are fingerprinted for the offer.
const maxRecurringFiles = 50

// ConflictContext returns what surrounds the conflicted files of the operation in progress.
func (a *App) ConflictContext(path string) (ConflictContext, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return ConflictContext{}, err
	}
	ctx := context.Background()
	out := ConflictContext{Rerere: ConflictRerere{Resolved: []string{}, Recurring: []string{}}}
	// The conflict tool waits for this before opening the next file: its parts, each a few
	// git starts, run together.
	var wg sync.WaitGroup
	wg.Go(func() { out.MergeTool = r.Git.MergeToolName(ctx) })
	wg.Go(func() { out.Clean = r.Git.MergedCleanly(ctx) })
	wg.Go(func() {
		out.Rerere.On = r.Git.RerereOn(ctx)
		if out.Rerere.On {
			if resolved, err := r.Git.RerereResolved(ctx); err == nil && resolved != nil {
				out.Rerere.Resolved = resolved
			}
		} else if !conflictlog.Offered(a.conflictDir(), r.CommonDir) {
			out.Rerere.Recurring = a.recurring(ctx, r.Git, r.CommonDir)
		}
	})
	wg.Wait()
	// A file rerere staged isn't one that merged cleanly.
	out.Clean = slices.DeleteFunc(out.Clean, func(f string) bool { return slices.Contains(out.Rerere.Resolved, f) })
	if out.Clean == nil {
		out.Clean = []string{}
	}
	return out, nil
}

// recurring lists the conflicted files whose conflict was resolved in the app before.
func (a *App) recurring(ctx context.Context, g *git.Repo, commonDir string) []string {
	named, err := g.ConflictFingerprints(ctx, maxRecurringFiles)
	if err != nil {
		return []string{}
	}
	files := make([]string, 0, len(named))
	for f := range named {
		files = append(files, f)
	}
	slices.Sort(files)
	fps := make([]string, len(files))
	for i, f := range files {
		fps[i] = named[f]
	}
	out := []string{}
	for i, seen := range conflictlog.Seen(a.conflictDir(), commonDir, fps) {
		if seen && fps[i] != "" {
			out = append(out, files[i])
		}
	}
	return out
}

// conflictDir is where conflictlog keeps its files: beside the settings in tests.
func (a *App) conflictDir() string {
	if a.settingsPath != "" {
		return filepath.Join(filepath.Dir(a.settingsPath), "conflicts")
	}
	return conflictlog.Dir()
}

// noteResolution keeps a conflict's fingerprint as it's resolved in the app, while rerere is
// off, so the same conflict coming back can be noticed. fp is read before resolving, while
// the file's sides are still in the index.
func (a *App) noteResolution(r *session.Repo, fp string) {
	if fp != "" {
		conflictlog.Resolved(a.conflictDir(), r.CommonDir, fp, time.Now())
	}
}

// fingerprintBefore reads a conflicted file's fingerprint for noteResolution, when rerere is
// off; "" otherwise.
func fingerprintBefore(ctx context.Context, g *git.Repo, file string) string {
	if g.RerereOn(ctx) {
		return ""
	}
	fp, _ := g.ConflictFingerprint(ctx, file)
	return fp
}

// ForgetResolution throws away the resolution rerere reused for file and puts its conflict
// back, to be resolved again (#938).
func (a *App) ForgetResolution(path, file string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	err = r.Git.ForgetResolution(context.Background(), file)
	if errors.Is(err, git.ErrNoResolution) {
		return userError("git has no resolution of " + file + "'s conflict to forget.")
	}
	return writeErr(err)
}

// TurnOnRerere sets rerere.enabled in the repository's own config, from the offer (#938).
func (a *App) TurnOnRerere(path string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	conflictlog.SetOffered(a.conflictDir(), r.CommonDir)
	return writeErr(r.Git.TurnOnRerere(context.Background()))
}

// DeclineRerere turns the offer down: it isn't made again in this repository.
func (a *App) DeclineRerere(path string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	conflictlog.SetOffered(a.conflictDir(), r.CommonDir)
	return nil
}

// ConflictOrigin names the commit that last changed lines start to end (1-based, inclusive)
// of file on one side of the conflict: "ours", "base" or "theirs". Its Commit is "" when
// there's nothing to name.
func (a *App) ConflictOrigin(path, file, side string, start, end int) (git.PlaceOrigin, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return git.PlaceOrigin{}, err
	}
	return r.Git.PlaceOrigin(context.Background(), r.GitDir, file, side, start, end)
}

// OpenMergeTool opens file in the user's own merge tool and returns when they close it. A
// result the tool reports as good is staged, which resolves the file.
func (a *App) OpenMergeTool(path, file string) error {
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	err = r.Git.RunMergeTool(context.Background(), file)
	if errors.Is(err, git.ErrNoMergeTool) {
		return userError("No merge tool is set. Set merge.tool in your git config to use one.")
	}
	var gerr *git.Error
	if errors.As(err, &gerr) {
		// The tool was closed without saving, or failed to start: the file is as it was.
		return userError("The merge tool closed without resolving " + file + ".")
	}
	return err
}
