package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/settings"
)

// The Pushing settings (#927): git's own keys for where pushes go, read and written in one
// scope, this repository's config or the global one. A row is written only when it's changed.

// pushKeys are the keys the panel edits, and the values each may take ("" unsets it).
var pushKeys = map[string][]string{
	"remote.pushDefault":   nil, // a remote's name
	"push.default":         {"simple", "current", "upstream", "matching", "nothing"},
	"push.autoSetupRemote": {"true", "false"},
	"push.followTags":      {"true", "false"},
}

// PushSetting is one key: what the scope says, and what git uses, from every scope.
type PushSetting struct {
	Key string `json:"key"`
	// Here is the value in the scope shown, or "" when it doesn't set it.
	Here string `json:"here"`
	// InUse is what git reads, from whichever scope sets it, or "" when none does.
	InUse string `json:"inUse"`
}

// PushSettings is the Pushing panel's content for one scope.
type PushSettings struct {
	Scope    string        `json:"scope"`
	Settings []PushSetting `json:"settings"`
	Remotes  []string      `json:"remotes"`
	// FetchAfterPush is the app's own setting: after a push to several places, fetch the
	// remotes those places belong to.
	FetchAfterPush bool `json:"fetchAfterPush"`
}

func scopeFlag(scope string) (string, error) {
	switch scope {
	case "local":
		return "--local", nil
	case "global":
		return "--global", nil
	}
	return "", userError("Unknown scope " + scope + ".")
}

// PushSettingsFor reads the Pushing panel for a repository, in scope "local" or "global".
func (a *App) PushSettingsFor(path, scope string) (PushSettings, error) {
	// Checked only: the read below covers every scope and keeps this one's values.
	_, err := scopeFlag(scope)
	if err != nil {
		return PushSettings{}, err
	}
	r, err := a.openRepo(path)
	if err != nil {
		return PushSettings{}, err
	}
	ctx := context.Background()
	out := PushSettings{Scope: scope, Settings: []PushSetting{}, Remotes: []string{}, FetchAfterPush: a.fetchAfterPush()}
	// Two reads at once, one of them all four keys in every scope: the panel reloads after
	// each change, and on Windows each git call costs about 40ms.
	var (
		remotes []git.Remote
		raw     []byte
		wg      sync.WaitGroup
	)
	wg.Go(func() { remotes, _ = r.Git.Remotes(ctx) })
	wg.Go(func() {
		raw, _ = r.Git.Read(ctx, "config", "--show-scope", "-z", "--get-regexp", `^(remote\.pushdefault|push\.(default|autosetupremote|followtags))$`)
	})
	wg.Wait()
	for _, rm := range remotes {
		out.Remotes = append(out.Remotes, rm.Name)
	}
	here, inUse := map[string]string{}, map[string]string{}
	fields := strings.Split(string(raw), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		key, value, ok := strings.Cut(fields[i+1], "\n")
		if !ok {
			// A key written bare is true.
			value = "true"
		}
		key = strings.ToLower(key)
		// The last one read wins, as git's own --get does.
		inUse[key] = value
		if fields[i] == scope {
			here[key] = value
		}
	}
	for _, k := range []string{"remote.pushDefault", "push.default", "push.autoSetupRemote", "push.followTags"} {
		out.Settings = append(out.Settings, PushSetting{Key: k, Here: here[strings.ToLower(k)], InUse: inUse[strings.ToLower(k)]})
	}
	return out, nil
}

// SetPushSetting writes one key in a scope, or with value "" unsets it there.
func (a *App) SetPushSetting(path, scope, key, value string) error {
	flag, err := scopeFlag(scope)
	if err != nil {
		return err
	}
	allowed, ok := pushKeys[key]
	if !ok {
		return userError("Git Revenge doesn't set " + key + " here.")
	}
	value = strings.TrimSpace(value)
	if value != "" && allowed != nil && !slices.Contains(allowed, value) {
		return userError(value + " isn't a value " + key + " takes.")
	}
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if value == "" {
		_, err = r.Git.Write(ctx, git.Cmd{Args: []string{"config", flag, "--unset-all", key}})
		var gerr *git.Error
		if errors.As(err, &gerr) && gerr.ExitCode == 5 {
			return nil
		}
		return writeErr(err)
	}
	if key == "remote.pushDefault" {
		if err := git.CheckRevs(value); err != nil {
			return userError("That isn't a remote's name.")
		}
	}
	_, err = r.Git.Write(ctx, git.Cmd{Args: []string{"config", flag, key, value}})
	return writeErr(err)
}

// SetBranchPushRemote makes one branch push to a remote other than where the rest go
// (branch.<name>.pushRemote), or with remote "" goes back to the usual place.
func (a *App) SetBranchPushRemote(path, branch, remote string) error {
	if err := git.CheckRevs(branch, remote); err != nil {
		return err
	}
	r, err := a.openRepo(path)
	if err != nil {
		return err
	}
	ctx := context.Background()
	key := "branch." + branch + ".pushRemote"
	if remote == "" {
		_, err = r.Git.Write(ctx, git.Cmd{Args: []string{"config", "--local", "--unset-all", key}})
		var gerr *git.Error
		if errors.As(err, &gerr) && gerr.ExitCode == 5 {
			err = nil
		}
	} else {
		_, err = r.Git.Write(ctx, git.Cmd{Args: []string{"config", "--local", key, remote}})
	}
	if err == nil {
		r.ReloadGraph()
	}
	return writeErr(err)
}

// fetchAfterPush is the app setting: after a push to several places, fetch the remotes those
// places belong to.
func (a *App) fetchAfterPush() bool {
	m, err := a.sessions()
	if err != nil {
		return false
	}
	return m.Settings().Get().FetchAfterPush
}

// SetFetchAfterPush turns the app setting on or off.
func (a *App) SetFetchAfterPush(on bool) error {
	m, err := a.sessions()
	if err != nil {
		return err
	}
	return m.Settings().Update(func(s *settings.Settings) { s.FetchAfterPush = on })
}
