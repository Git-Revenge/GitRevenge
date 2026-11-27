package git

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// loose is a repository of n commits, every object still loose.
func loose(t *testing.T, n int) (*Repo, *gittest.Repo) {
	t.Helper()
	fx := gittest.New(t)
	for i := range n {
		fx.CommitFile("f"+strconv.Itoa(i)+".txt", strconv.Itoa(i)+"\n", "c"+strconv.Itoa(i))
	}
	return OpenRepo(newTestRunner(t), fx.Dir), fx
}

func TestStorageCountsAndSaysWhenGitWouldTidy(t *testing.T) {
	t.Parallel()
	repo, fx := loose(t, 3)
	ctx := context.Background()
	s, err := repo.Storage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Three commits of one file each: a commit, a tree and a blob apiece.
	// git counts sizes in whole KiB: nine tiny objects come to nothing.
	if s.Loose != 9 || s.Packs != 0 || s.GCAuto != defaultGCAuto || s.AutoPackLimit != defaultAutoPackLimit || s.Due() {
		t.Fatalf("storage = %+v", s)
	}
	// git's own thresholds decide: past gc.auto, it's due.
	fx.Git("config", "gc.auto", "5")
	if s, _ = repo.Storage(ctx); s.GCAuto != 5 || !s.Due() {
		t.Errorf("past gc.auto: %+v", s)
	}
	fx.Git("config", "gc.auto", "0")
	if s, _ = repo.Storage(ctx); s.Due() {
		t.Errorf("gc.auto 0 turns it off: %+v", s)
	}
	fx.Git("gc", "-q")
	if s, _ = repo.Storage(ctx); s.Loose != 0 || s.Packs != 1 || s.PackBytes == 0 || s.Bytes() != s.PackBytes {
		t.Errorf("after gc: %+v", s)
	}
}

func TestRunMaintenanceTidies(t *testing.T) {
	t.Parallel()
	repo, _ := loose(t, 3)
	ctx := context.Background()
	if err := repo.RunMaintenance(ctx, NetOptions{}); err != nil {
		t.Fatal(err)
	}
	if s, _ := repo.Storage(ctx); s.Loose != 0 || s.Packs == 0 {
		t.Errorf("after maintenance: %+v", s)
	}
}

// git maintenance start registers the repository in the global config and sets up the system's
// scheduler. The test's global config is its own, and GIT_TEST_MAINT_SCHEDULER, git's own hook
// for this, stands in for every scheduler: nothing is scheduled on the machine running it.
func TestScheduleMaintenance(t *testing.T) {
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = T\n\temail = t@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TEST_MAINT_SCHEDULER", "launchctl:true,schtasks:true,crontab:true,systemctl:true")
	repo, _ := loose(t, 1)
	ctx := context.Background()
	if s, _ := repo.Storage(ctx); s.Registered {
		t.Fatal("registered before it was")
	}
	if err := repo.ScheduleMaintenance(ctx, true); err != nil {
		t.Fatal(err)
	}
	if s, _ := repo.Storage(ctx); !s.Registered {
		t.Fatal("not registered after starting")
	}
	if b, _ := os.ReadFile(global); !strings.Contains(string(b), "[maintenance]") {
		t.Errorf("the global config has no maintenance.repo:\n%s", b)
	}
	if err := repo.ScheduleMaintenance(ctx, false); err != nil {
		t.Fatal(err)
	}
	if s, _ := repo.Storage(ctx); s.Registered {
		t.Error("still registered")
	}
	// Taking it off again is no error.
	if err := repo.ScheduleMaintenance(ctx, false); err != nil {
		t.Errorf("again: %v", err)
	}
}

func TestSamePath(t *testing.T) {
	t.Parallel()
	if !samePath("C:/work/repo/", filepath.FromSlash("C:/work/repo")) || samePath("/a/b", "/a/c") {
		t.Error("samePath")
	}
}
