package app

import (
	"strconv"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestRepoStorageAndMaintenance(t *testing.T) {
	isolateGit(t)
	// git's own hook for testing maintenance: no scheduler on the machine is touched.
	t.Setenv("GIT_TEST_MAINT_SCHEDULER", "launchctl:true,schtasks:true,crontab:true,systemctl:true")
	a := newTestApp(t)
	fx := gittest.New(t)
	for i := range 4 {
		fx.CommitFile("f"+strconv.Itoa(i)+".txt", strconv.Itoa(i)+"\n", "c")
	}
	fx.Git("config", "gc.auto", "3")
	path := openForMap(t, a, fx)

	s, err := a.RepoStorage(path)
	if err != nil || s.Loose != 12 || !s.Due || s.Registered {
		t.Fatalf("storage = %+v, %v", s, err)
	}
	if err := a.RunMaintenance(path); err != nil {
		t.Fatal(err)
	}
	if s, _ = a.RepoStorage(path); s.Loose != 0 || s.Due || s.Packs == 0 || s.Bytes != s.PackBytes {
		t.Errorf("after maintenance: %+v", s)
	}
	if err := a.ScheduleMaintenance(path, true); err != nil {
		t.Fatal(err)
	}
	if s, _ = a.RepoStorage(path); !s.Registered {
		t.Error("not on the schedule")
	}
	if err := a.ScheduleMaintenance(path, false); err != nil {
		t.Fatal(err)
	}
	if s, _ = a.RepoStorage(path); s.Registered {
		t.Error("still on the schedule")
	}
}
