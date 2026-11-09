package git

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func newTestObjectReader(t *testing.T) (*ObjectReader, *gittest.Repo) {
	t.Helper()
	fx := gittest.New(t)
	o := NewObjectReader(newTestRunner(t), fx.Dir)
	t.Cleanup(func() { _ = o.Close() })
	return o, fx
}

func TestObjectReaderContents(t *testing.T) {
	t.Parallel()
	o, fx := newTestObjectReader(t)
	fx.CommitFile("hello.txt", "hello\nworld\n", "add hello")

	info, data, err := o.Contents("HEAD:hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "blob" || info.Size != 12 || string(data) != "hello\nworld\n" {
		t.Fatalf("got %+v %q", info, data)
	}

	// The stream must stay in sync across requests.
	info, data, err = o.Contents("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "commit" || info.ID != fx.Head() || len(data) != int(info.Size) {
		t.Fatalf("commit read wrong: %+v (%d bytes)", info, len(data))
	}
}

func TestObjectReaderInfo(t *testing.T) {
	t.Parallel()
	o, fx := newTestObjectReader(t)
	fx.CommitFile("a.txt", "abc", "add a")

	info, err := o.Info("HEAD:a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if info.Type != "blob" || info.Size != 3 {
		t.Fatalf("got %+v", info)
	}
}

func TestObjectReaderMissing(t *testing.T) {
	t.Parallel()
	o, fx := newTestObjectReader(t)
	fx.Linear(1)

	if _, _, err := o.Contents("HEAD:nope.txt"); !errors.Is(err, ErrObjectMissing) {
		t.Fatalf("err = %v, want ErrObjectMissing", err)
	}
	// A missing object must not break the stream for the next request.
	if _, err := o.Info("HEAD"); err != nil {
		t.Fatal(err)
	}
}

func TestObjectReaderRestartsAfterProcessDies(t *testing.T) {
	t.Parallel()
	o, fx := newTestObjectReader(t)
	fx.Linear(1)
	if _, err := o.Info("HEAD"); err != nil {
		t.Fatal(err)
	}

	o.mu.Lock()
	_ = o.cmd.Process.Kill()
	_ = o.cmd.Wait()
	o.mu.Unlock()

	if _, err := o.Info("HEAD"); err != nil {
		t.Fatalf("reader should restart the process: %v", err)
	}
}

func TestObjectReaderConcurrent(t *testing.T) {
	t.Parallel()
	o, fx := newTestObjectReader(t)
	shas := fx.Linear(10)

	var wg sync.WaitGroup
	errs := make(chan error, len(shas)*5)
	for range 5 {
		for _, sha := range shas {
			wg.Go(func() {
				info, err := o.Info(sha)
				if err == nil && info.ID != sha {
					err = errors.New("got " + info.ID + " for " + sha)
				}
				errs <- err
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestObjectReaderRejectsNewlines(t *testing.T) {
	t.Parallel()
	o, _ := newTestObjectReader(t)
	if _, err := o.Info("HEAD\ncontents HEAD"); err == nil {
		t.Fatal("revision containing a newline must be rejected")
	}
}

// BenchmarkObjectInfo compares a process per object with the persistent reader.
func BenchmarkObjectInfo(b *testing.B) {
	fx := gittest.New(b)
	shas := fx.Linear(20)
	bin, err := exec.LookPath("git")
	if err != nil {
		b.Skip("git not installed")
	}
	r := NewRunner(bin)

	b.Run("spawn", func(b *testing.B) {
		for i := 0; b.Loop(); i++ {
			if _, err := r.Output(context.Background(), Cmd{Dir: fx.Dir, Args: []string{"cat-file", "-t", shas[i%len(shas)]}}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("batch", func(b *testing.B) {
		o := NewObjectReader(r, fx.Dir)
		b.Cleanup(func() { _ = o.Close() })
		for i := 0; b.Loop(); i++ {
			if _, err := o.Info(shas[i%len(shas)]); err != nil {
				b.Fatal(err)
			}
		}
	})
}
