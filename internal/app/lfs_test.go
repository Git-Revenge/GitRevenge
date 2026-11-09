package app

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestLFSAndSubmoduleBindings(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	fx := gittest.New(t)
	fx.SetupLFS("*.bin")
	fx.WriteFile("a.bin", "\x00x")
	fx.Commit("bin")
	sub := gittest.New(t)
	sub.Linear(1)
	fx.AddSubmodule(sub, "vendor/lib")
	tab, _ := a.OpenRepo(fx.Dir)

	info, err := a.LFS(tab.Path)
	if err != nil || !info.Available || len(info.Status.Files) != 1 {
		t.Fatalf("lfs = %+v, %v", info, err)
	}
	if err := a.LFSAction(tab.Path, "track", []string{"*.wav", ""}); err != nil {
		t.Fatal(err)
	}
	if info, _ := a.LFS(tab.Path); len(info.Status.Patterns) != 2 {
		t.Fatalf("patterns = %+v", info.Status.Patterns)
	}
	if err := a.LFSAction(tab.Path, "explode", nil); err == nil {
		t.Fatal("unknown action accepted")
	}

	subs, err := a.Submodules(tab.Path)
	if err != nil || len(subs) != 1 || subs[0].Path != "vendor/lib" {
		t.Fatalf("subs = %+v, %v", subs, err)
	}
}
