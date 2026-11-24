package app

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestPushSettings(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.Commit("one")
	fx.Git("remote", "add", "origin", "https://example.com/up.git")
	fx.Git("remote", "add", "fork", "https://example.com/fork.git")
	tab, err := a.OpenRepo(fx.Dir)
	if err != nil {
		t.Fatal(err)
	}
	get := func(scope, key string) PushSetting {
		t.Helper()
		ps, err := a.PushSettingsFor(tab.Path, scope)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(ps.Settings, func(s PushSetting) bool { return s.Key == key })
		if i < 0 {
			t.Fatalf("no %s in %+v", key, ps.Settings)
		}
		return ps.Settings[i]
	}
	if s := get("local", "remote.pushDefault"); s.Here != "" || s.InUse != "" {
		t.Fatalf("untouched = %+v", s)
	}
	if err := a.SetPushSetting(tab.Path, "local", "remote.pushDefault", "fork"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetPushSetting(tab.Path, "local", "push.default", "upstream"); err != nil {
		t.Fatal(err)
	}
	if s := get("local", "remote.pushDefault"); s.Here != "fork" || s.InUse != "fork" {
		t.Fatalf("after setting = %+v", s)
	}
	// The global scope doesn't set it, but git uses the repository's.
	if s := get("global", "remote.pushDefault"); s.Here != "" || s.InUse != "fork" {
		t.Fatalf("global view = %+v", s)
	}
	if err := a.SetPushSetting(tab.Path, "local", "push.default", "sideways"); err == nil {
		t.Fatal("took a value push.default doesn't")
	}
	if err := a.SetPushSetting(tab.Path, "local", "core.editor", "vi"); err == nil {
		t.Fatal("wrote a key the panel doesn't own")
	}
	if err := a.SetPushSetting(tab.Path, "local", "remote.pushDefault", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.SetPushSetting(tab.Path, "local", "remote.pushDefault", ""); err != nil {
		t.Fatalf("unsetting twice: %v", err)
	}
	if s := get("local", "remote.pushDefault"); s.Here != "" {
		t.Fatalf("still set = %+v", s)
	}
	// push.default upstream pushes only where a branch tracks; back to the app's default.
	if err := a.SetPushSetting(tab.Path, "local", "push.default", ""); err != nil {
		t.Fatal(err)
	}

	// One branch pushing somewhere else: on its ref, and in the plan.
	if err := a.SetBranchPushRemote(tab.Path, "main", "fork"); err != nil {
		t.Fatal(err)
	}
	var mainRef string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		loadedPage(t, a, tab.Path)
		refs, _ := a.Refs(tab.Path)
		for _, r := range refs {
			if r.Short == "main" && r.Kind == "local" {
				mainRef = r.PushRemote
			}
		}
		if mainRef == "fork" {
			break
		}
	}
	if mainRef != "fork" {
		t.Fatalf("main's pushRemote on its ref = %q", mainRef)
	}
	if plan, _ := a.PushPlan(tab.Path, "main"); plan.Remote != "fork" || plan.Why != "branch.main.pushRemote" {
		t.Fatalf("plan = %+v", plan)
	}
	if err := a.SetBranchPushRemote(tab.Path, "main", ""); err != nil {
		t.Fatal(err)
	}
	if out := fx.Git("config", "--local", "--list"); strings.Contains(out, "pushremote") {
		t.Fatalf("pushRemote left behind:\n%s", out)
	}
}

func TestFetchAfterPush(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	local, origin := gittest.WithRemote(t, 1)
	mirror := gittest.NewBare(t)
	local.Git("remote", "add", "mirror", mirror.Dir)
	local.Git("remote", "set-url", "--add", "--push", "origin", origin.Dir)
	local.Git("remote", "set-url", "--add", "--push", "origin", mirror.Dir)
	local.Commit("two")
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	has := func() bool { return strings.TrimSpace(local.Git("for-each-ref", "refs/remotes/mirror/main")) != "" }

	// Off, the default: mirror's copy isn't fetched.
	if err := a.PushCurrent(tab.Path, false); err != nil {
		t.Fatal(err)
	}
	if has() {
		t.Fatal("mirror was fetched with the setting off")
	}
	if err := a.SetFetchAfterPush(true); err != nil {
		t.Fatal(err)
	}
	local.Commit("three")
	if err := a.PushCurrent(tab.Path, false); err != nil {
		t.Fatal(err)
	}
	if !has() {
		t.Fatal("mirror wasn't fetched after the push")
	}
}
