package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// ObjectInfo describes a git object.
type ObjectInfo struct {
	ID   string
	Type string // "commit", "tree", "blob" or "tag"
	Size int64
}

// ErrObjectMissing means the requested object or revision does not exist.
var ErrObjectMissing = errors.New("object not found")

// ObjectReader reads objects through one long-lived `git cat-file --batch-command` process,
// avoiding a process spawn per object. It is safe for concurrent use; requests are
// processed one at a time. If the process dies, the next request restarts it.
type ObjectReader struct {
	runner *Runner
	dir    string

	mu     sync.Mutex
	cancel context.CancelFunc
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

// NewObjectReader returns a reader for the repository at dir. The process starts lazily.
func NewObjectReader(r *Runner, dir string) *ObjectReader {
	return &ObjectReader{runner: r, dir: dir}
}

// Info returns an object's ID, type and size without reading its contents.
func (o *ObjectReader) Info(rev string) (ObjectInfo, error) {
	var info ObjectInfo
	err := o.do("info", rev, func(rd *bufio.Reader) error {
		var err error
		info, err = readHeader(rd, rev)
		return err
	})
	return info, err
}

// Contents returns an object's header and full contents.
func (o *ObjectReader) Contents(rev string) (ObjectInfo, []byte, error) {
	var (
		info ObjectInfo
		data []byte
	)
	err := o.do("contents", rev, func(rd *bufio.Reader) error {
		var err error
		if info, err = readHeader(rd, rev); err != nil {
			return err
		}
		// Contents are followed by a single LF.
		data = make([]byte, info.Size+1)
		if _, err := io.ReadFull(rd, data); err != nil {
			return err
		}
		data = data[:info.Size]
		return nil
	})
	return info, data, err
}

// Close stops the cat-file process.
func (o *ObjectReader) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stopLocked()
	return nil
}

func (o *ObjectReader) do(command, rev string, read func(*bufio.Reader) error) error {
	if rev == "" || strings.ContainsAny(rev, "\n\r") {
		return fmt.Errorf("invalid revision %q", rev)
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	// One retry covers a process that exited between requests.
	for attempt := 0; ; attempt++ {
		if o.cmd == nil {
			if err := o.startLocked(); err != nil {
				return err
			}
		}
		err := o.requestLocked(command, rev, read)
		if err == nil || errors.Is(err, ErrObjectMissing) {
			return err
		}
		// The stream is now out of sync or dead; restart before any further use.
		o.stopLocked()
		if attempt == 1 {
			return fmt.Errorf("cat-file %s %s: %w", command, rev, err)
		}
	}
}

func (o *ObjectReader) requestLocked(command, rev string, read func(*bufio.Reader) error) error {
	// One request is one line: a line break in rev would start a request of its own.
	if strings.ContainsAny(rev, "\r\n\x00") {
		return fmt.Errorf("%q isn't an object name", rev)
	}
	if _, err := io.WriteString(o.stdin, command+" "+rev+"\n"); err != nil {
		return err
	}
	return read(o.stdout)
}

func (o *ObjectReader) startLocked() error {
	ctx, cancel := context.WithCancel(context.Background())
	// It reads commits, which every clone has: never a download, which in a partial clone
	// would hold up every read queued behind it.
	cmd := o.runner.command(ctx, Cmd{Dir: o.dir, Args: []string{"cat-file", "--batch-command"}, ReadOnly: true, Env: noLazyFetch})
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("starting git cat-file: %w", err)
	}
	o.cancel, o.cmd, o.stdin, o.stdout = cancel, cmd, stdin, bufio.NewReaderSize(stdout, 64*1024)
	return nil
}

func (o *ObjectReader) stopLocked() {
	if o.cmd == nil {
		return
	}
	_ = o.stdin.Close()
	o.cancel()
	_ = o.cmd.Wait()
	o.cancel, o.cmd, o.stdin, o.stdout = nil, nil, nil, nil
}

// readHeader parses "<oid> <type> <size>" or "<rev> missing" / "<rev> ambiguous".
func readHeader(rd *bufio.Reader, rev string) (ObjectInfo, error) {
	line, err := rd.ReadString('\n')
	if err != nil {
		return ObjectInfo{}, err
	}
	line = strings.TrimSuffix(line, "\n")
	if rest, ok := strings.CutPrefix(line, rev+" "); ok && (rest == "missing" || rest == "ambiguous") {
		return ObjectInfo{}, fmt.Errorf("%s: %w (%s)", rev, ErrObjectMissing, rest)
	}
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return ObjectInfo{}, fmt.Errorf("unexpected cat-file header %q", line)
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("unexpected cat-file header %q", line)
	}
	return ObjectInfo{ID: fields[0], Type: fields[1], Size: size}, nil
}
