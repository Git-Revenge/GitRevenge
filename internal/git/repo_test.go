package git

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func openTestRepo(t *testing.T) (*Repo, *gittest.Repo) {
	t.Helper()
	fx := gittest.New(t)
	fx.Linear(1)
	return OpenRepo(newTestRunner(t), fx.Dir), fx
}

func TestWriteRetriesUntilLockReleased(t *testing.T) {
	repo, fx := openTestRepo(t)
	fx.WriteFile("a.txt", "a\n")
	lock := fx.Path(".git/index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(250 * time.Millisecond)
		_ = os.Remove(lock)
	}()

	if _, err := repo.Write(context.Background(), Cmd{Args: []string{"add", "a.txt"}}); err != nil {
		t.Fatalf("Write should succeed once the lock is released: %v", err)
	}
}

func TestWriteReportsPersistentLock(t *testing.T) {
	repo, fx := openTestRepo(t)
	fx.WriteFile("a.txt", "a\n")
	if err := os.WriteFile(fx.Path(".git/index.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := repo.Write(context.Background(), Cmd{Args: []string{"add", "a.txt"}})
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v, want ErrLocked", err)
	}
	var lerr *LockedError
	if !errors.As(err, &lerr) || lerr.Path == "" {
		t.Fatalf("err = %#v, want *LockedError with path", err)
	}
}

func TestWritesAreSerialized(t *testing.T) {
	repo, fx := openTestRepo(t)
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	names := make([]string, n)
	for i := range names {
		names[i] = string(rune('a'+i)) + ".txt"
		fx.WriteFile(names[i], names[i])
	}
	for _, name := range names {
		wg.Go(func() {
			// Concurrent adds would collide on index.lock; serialization means none do.
			_, err := repo.Write(context.Background(), Cmd{Args: []string{"add", name}})
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	staged, err := repo.ReadString(context.Background(), "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(strings.Fields(staged)); got != n {
		t.Fatalf("staged %d files, want %d", got, n)
	}
}

func TestWriteWaitRespectsContext(t *testing.T) {
	repo, _ := openTestRepo(t)
	repo.writes <- struct{}{} // simulate a long-running write holding the slot
	defer func() { <-repo.writes }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := repo.Write(ctx, Cmd{Args: []string{"status"}}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

func TestNotIgnored(t *testing.T) {
	repo, fx := openTestRepo(t)
	fx.CommitFile(".gitignore", "build/\n*.log\n", "ignore")

	got, err := repo.NotIgnored(context.Background(), []string{"src/a.go", "build/out.bin", "debug.log", "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "src/a.go,README.md" {
		t.Fatalf("NotIgnored = %v", got)
	}

	got, err = repo.NotIgnored(context.Background(), []string{"src/a.go"})
	if err != nil || len(got) != 1 {
		t.Fatalf("no ignored paths: got %v, %v", got, err)
	}
}
