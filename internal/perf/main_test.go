package perf

import (
	"os"
	"testing"
)

// TestMain runs the benchmark with the git config of whoever runs it. Every other package's
// tests replace that config with their own (see gittest.Main), so a global core.hooksPath or
// the like can't change what they check. Here it would change what's measured: the
// benchmark times a real repository as its owner set it up, and on the Linux kernel
// `git status` takes about 0.4 seconds with core.fsmonitor and core.untrackedCache, which
// people usually set globally, and 2 seconds without (#914). The benchmark only reads, so
// nothing that isolation guards against applies.
func TestMain(m *testing.M) { os.Exit(m.Run()) }
