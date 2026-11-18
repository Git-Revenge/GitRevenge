// Package trust finds what a repository would make git run.
//
// git takes orders from a repository's own .git/config and hooks: a program to run whenever
// files are checked for changes, a filter for every file read, a command to connect with.
// A clone never brings these along, so in a cloned repository they are the user's own. A
// repository that arrives with its .git folder already made (an archive, a shared folder, a
// USB stick) can hold whatever its maker put there, and git's own guard, safe.directory, only
// asks who owns the folder.
//
// Scan reads the config and lists the hooks without running git inside the repository, so
// nothing found there can act before the user has seen it.
package trust

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Finding is one thing a repository would make git run.
type Finding struct {
	// Kind is "config" for a setting or "hook" for a hook script.
	Kind string `json:"kind"`
	// Name is the setting's key, or the hook's file name.
	Name string `json:"name"`
	// Value is the setting's value; empty for a hook.
	Value string `json:"value"`
	// Why says what it does, to finish the sentence "This …".
	Why string `json:"why"`
}

// exact are settings that name a program or redirect git, by whole key.
var exact = map[string]string{
	"core.fsmonitor":             "runs a program every time the repository's files are checked for changes",
	"core.sshcommand":            "is the program run to connect on every fetch, pull and push",
	"core.hookspath":             "makes git run hook scripts from another folder",
	"core.editor":                "is the program git starts to edit a message",
	"sequence.editor":            "is the program git starts to edit a rebase",
	"core.pager":                 "is the program git pipes its output through",
	"core.askpass":               "is the program git asks for your passwords",
	"core.gitproxy":              "is a program git connects through",
	"core.alternaterefscommand":  "is a program git runs to list refs",
	"core.worktree":              "points git at working files in another folder",
	"diff.external":              "is a program run to compare files",
	"gpg.program":                "is the program run to sign and check signatures",
	"uploadpack.packobjectshook": "is a program run when the repository is fetched from",
	"credential.helper":          "is a program that is handed your credentials",
	"include.path":               "pulls in more settings from another file",
	"protocol.allow":             "changes which ways of connecting git allows",
}

// keyed are settings of the form section.<name>.key, by section and key.
var keyed = map[string]map[string]string{
	"filter":     {"clean": "is a program run on every file that is staged", "smudge": "is a program run on every file that is checked out or shown", "process": "is a program run on every file that is staged, checked out or shown"},
	"diff":       {"textconv": "is a program run on files before they are compared", "command": "is a program run to compare files"},
	"merge":      {"driver": "is a program run to merge files"},
	"mergetool":  {"cmd": "is the command run as the merge tool", "path": "is the program run as the merge tool"},
	"difftool":   {"cmd": "is the command run as the diff tool", "path": "is the program run as the diff tool"},
	"gpg":        {"program": "is the program run to sign and check signatures"},
	"credential": {"helper": "is a program that is handed your credentials"},
	"includeif":  {"path": "pulls in more settings from another file"},
	"remote":     {"uploadpack": "is a program run when fetching", "receivepack": "is a program run when pushing", "proxy": "is where connections to the remote are sent through", "vcs": "is a helper program run to talk to the remote"},
	"protocol":   {"allow": "changes which ways of connecting git allows"},
	"pager":      {"": "is a program git pipes a command's output through"},
	"url":        {"insteadof": "sends fetches and pushes to a different address than the one written", "pushinsteadof": "sends pushes to a different address than the one written"},
	"submodule":  {"update": "is a command run to update a submodule"},
}

// lfsFilter is what `git lfs install --local` writes: Git LFS's own filter, not a stranger's.
var lfsFilter = map[string][]string{
	"filter.lfs.clean":   {"git-lfs clean -- %f"},
	"filter.lfs.smudge":  {"git-lfs smudge -- %f", "git-lfs smudge --skip -- %f"},
	"filter.lfs.process": {"git-lfs filter-process", "git-lfs filter-process --skip"},
}

// lfsHooks are the hooks Git LFS installs, by file name.
var lfsHooks = []string{"pre-push", "post-checkout", "post-commit", "post-merge"}

// hookWhen says when git runs a hook, for the common ones.
var hookWhen = map[string]string{
	"pre-commit":            "runs when you commit",
	"prepare-commit-msg":    "runs when you commit",
	"commit-msg":            "runs when you commit",
	"post-commit":           "runs after you commit",
	"pre-push":              "runs when you push",
	"post-checkout":         "runs when you check out a branch",
	"post-merge":            "runs after a merge or a pull",
	"pre-rebase":            "runs when you rebase",
	"post-rewrite":          "runs after a rebase or an amend",
	"pre-merge-commit":      "runs when you merge",
	"reference-transaction": "runs whenever a branch or tag changes, including on fetch",
	"post-index-change":     "runs whenever staged files change",
	"fsmonitor-watchman":    "runs when files are checked for changes, if it's switched on",
	"pre-auto-gc":           "runs when git tidies the repository",
	"post-applypatch":       "runs after a patch is applied",
	"pre-applypatch":        "runs when a patch is applied",
	"applypatch-msg":        "runs when a patch is applied",
}

// Scan lists what the repository whose git folders are gitDir and commonDir would make git
// run: settings in its own config (and a worktree's config.worktree), and its hook scripts.
// They're the same folder except in a linked worktree. Git LFS's own filter and hooks aren't
// listed. Nothing in the repository is run to find out.
func Scan(ctx context.Context, runner *git.Runner, gitDir, commonDir string) ([]Finding, error) {
	var found []Finding
	files := []string{filepath.Join(commonDir, "config")}
	if wt := filepath.Join(gitDir, "config.worktree"); !slices.Contains(files, wt) {
		files = append(files, wt)
	}
	for _, f := range files {
		entries, err := readConfig(ctx, runner, f)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if why := risk(e[0], e[1]); why != "" {
				found = append(found, Finding{Kind: "config", Name: e[0], Value: e[1], Why: why})
			}
		}
	}
	hooks, err := os.ReadDir(filepath.Join(commonDir, "hooks"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, h := range hooks {
		name := h.Name()
		if h.IsDir() || strings.HasSuffix(name, ".sample") {
			continue
		}
		if slices.Contains(lfsHooks, name) && isLFSHook(filepath.Join(commonDir, "hooks", name), name) {
			continue
		}
		why := hookWhen[name]
		if why == "" {
			why = "is a hook script git may run"
		}
		found = append(found, Finding{Kind: "hook", Name: name, Why: why})
	}
	return found, nil
}

// readConfig lists one config file's settings as key and value, without following includes.
// git reads the file as data, from a folder outside the repository, so the repository's own
// settings aren't in force while it does. A missing file has none.
func readConfig(ctx context.Context, runner *git.Runner, file string) ([][2]string, error) {
	if _, err := os.Stat(file); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out, err := runner.Run(ctx, git.Cmd{
		Dir:      os.TempDir(),
		Args:     []string{"config", "--file", file, "--list", "--no-includes", "-z"},
		ReadOnly: true,
	})
	if err != nil {
		return nil, err
	}
	var entries [][2]string
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		// "key\nvalue"; a key with no "=" has no value, which git reads as true.
		key, value, _ := strings.Cut(string(rec), "\n")
		entries = append(entries, [2]string{key, value})
	}
	return entries, nil
}

// risk says why a setting is one to ask about, or "" when it isn't.
func risk(key, value string) string {
	if allowed, ok := lfsFilter[key]; ok && slices.Contains(allowed, value) {
		return ""
	}
	lower := strings.ToLower(key)
	if lower == "core.fsmonitor" {
		// A true or false value only switches git's built-in watcher; anything else is a program.
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "", "true", "false", "yes", "no", "on", "off", "1", "0":
			return ""
		}
	}
	if why := exact[lower]; why != "" {
		return why
	}
	// pager.<command> has no middle part.
	if strings.HasPrefix(lower, "pager.") {
		return keyed["pager"][""]
	}
	first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
	if first < 0 || first == last {
		return ""
	}
	section, name := strings.ToLower(key[:first]), strings.ToLower(key[last+1:])
	names, ok := keyed[section]
	if !ok {
		return ""
	}
	if section == "submodule" && !strings.HasPrefix(strings.TrimSpace(value), "!") {
		return ""
	}
	return names[name]
}

// isLFSHook reports whether a hook file is the one Git LFS installs: a check that git-lfs is
// there, then "git lfs <name>". Anything else in the file, and it's listed like any other.
func isLFSHook(file, name string) bool {
	info, err := os.Stat(file)
	if err != nil || info.Size() > 4096 {
		return false
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	ran := false
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case line == `git lfs `+name+` "$@"`:
			ran = true
		case strings.HasPrefix(line, "command -v git-lfs >/dev/null 2>&1 || {") && strings.HasSuffix(line, "exit 2; }") &&
			!strings.ContainsAny(line, "`") && !strings.Contains(line, "$("):
		default:
			return false
		}
	}
	return ran
}
