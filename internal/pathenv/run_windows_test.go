package pathenv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// scratchKey is an empty registry key of the test's own, standing in for the environment.
func scratchKey(t *testing.T) registry.Key {
	t.Helper()
	name := fmt.Sprintf(`Software\GitRevenge-test-%d-%s`, os.Getpid(), strings.ReplaceAll(t.Name(), "/", "-"))
	key, _, err := registry.CreateKey(registry.CURRENT_USER, name, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, name); err != nil {
			t.Errorf("removing the scratch key: %v", err)
		}
	})
	return key
}

func adding(path string) (string, bool)   { return Add(path, bin) }
func removing(path string) (string, bool) { return Remove(path, bin) }

func readPath(t *testing.T, key registry.Key) (string, uint32) {
	t.Helper()
	path, kind, err := key.GetStringValue("Path")
	if err != nil {
		t.Fatal(err)
	}
	return path, kind
}

// A PATH longer than the 1024 characters NSIS can read, with variables in it, comes through
// an add and a remove unchanged, still as an expandable string.
func TestEditKeepsALongExpandablePath(t *testing.T) {
	key := scratchKey(t)
	long := `%SystemRoot%\system32;%SystemRoot%`
	for i := 0; len(long) < 5000; i++ {
		long += fmt.Sprintf(`;C:\Program Files\Some Tool %d\bin`, i)
	}
	if err := key.SetExpandStringValue("Path", long); err != nil {
		t.Fatal(err)
	}

	if err := edit(key, adding); err != nil {
		t.Fatal(err)
	}
	if got, kind := readPath(t, key); got != long+";"+bin || kind != registry.EXPAND_SZ {
		t.Fatalf("after add: type %d, %d characters ending %q", kind, len(got), got[max(0, len(got)-60):])
	}

	// A second install changes nothing.
	if err := edit(key, adding); err != nil {
		t.Fatal(err)
	}
	if got, _ := readPath(t, key); got != long+";"+bin {
		t.Fatalf("a second add changed the path: %d characters", len(got))
	}

	if err := edit(key, removing); err != nil {
		t.Fatal(err)
	}
	if got, kind := readPath(t, key); got != long || kind != registry.EXPAND_SZ {
		t.Fatalf("after remove: type %d, path changed: %v", kind, got != long)
	}
}

func TestEditKeepsAPlainStringPlain(t *testing.T) {
	key := scratchKey(t)
	if err := key.SetStringValue("Path", `C:\Windows`); err != nil {
		t.Fatal(err)
	}
	if err := edit(key, adding); err != nil {
		t.Fatal(err)
	}
	if got, kind := readPath(t, key); got != `C:\Windows;`+bin || kind != registry.SZ {
		t.Fatalf("got type %d, %q", kind, got)
	}
}

func TestEditCreatesAMissingPathAsExpandable(t *testing.T) {
	key := scratchKey(t)
	if err := edit(key, adding); err != nil {
		t.Fatal(err)
	}
	if got, kind := readPath(t, key); got != bin || kind != registry.EXPAND_SZ {
		t.Fatalf("got type %d, %q", kind, got)
	}
}

// A user's PATH is often only what an installer added. Taking the entry off takes the
// value away, so an install and an uninstall leave the environment as they found it (#895).
func TestEditTakesAwayAPathItEmpties(t *testing.T) {
	key := scratchKey(t)
	if err := edit(key, adding); err != nil {
		t.Fatal(err)
	}
	if err := edit(key, removing); err != nil {
		t.Fatal(err)
	}
	if _, _, err := key.GetStringValue("Path"); !errors.Is(err, registry.ErrNotExist) {
		t.Fatalf("an empty Path was left behind: %v", err)
	}
	// One that has something else in it stays.
	if err := key.SetStringValue("Path", `C:\Tools;`+bin); err != nil {
		t.Fatal(err)
	}
	if err := edit(key, removing); err != nil {
		t.Fatal(err)
	}
	if got, kind := readPath(t, key); got != `C:\Tools` || kind != registry.SZ {
		t.Fatalf("got type %d, %q", kind, got)
	}
}

func TestEditLeavesAMissingPathMissingWhenNothingChanges(t *testing.T) {
	key := scratchKey(t)
	if err := edit(key, removing); err != nil {
		t.Fatal(err)
	}
	if _, _, err := key.GetStringValue("Path"); !errors.Is(err, registry.ErrNotExist) {
		t.Fatalf("Path was written: %v", err)
	}
}

func TestEditRefusesAPathThatIsNotAString(t *testing.T) {
	key := scratchKey(t)
	if err := key.SetDWordValue("Path", 7); err != nil {
		t.Fatal(err)
	}
	if err := edit(key, adding); err == nil {
		t.Fatal("no error for a DWORD Path")
	}
	if got, _, err := key.GetIntegerValue("Path"); err != nil || got != 7 {
		t.Fatalf("the value was touched: %d, %v", got, err)
	}
}

func TestRunIgnoresAnOrdinaryStart(t *testing.T) {
	for _, args := range [][]string{nil, {}, {`C:\some\repo`}, {"add", bin}, {"x", Flag, "add", bin}} {
		if _, handled := Run(args, &bytes.Buffer{}); handled {
			t.Errorf("Run(%q) took an ordinary start as the installer's", args)
		}
	}
}

// Each of these fails before the registry is opened. The machine's PATH can't be asked for
// in any spelling (#895).
func TestRunRefusesWhatItCannotDo(t *testing.T) {
	tests := map[string][]string{
		"no arguments":        {Flag},
		"too few":             {Flag, "add"},
		"unknown action":      {Flag, "prepend", bin},
		"the machine's PATH":  {Flag, "add", "machine", bin},
		"a scope at all":      {Flag, "add", "user", bin},
		"relative folder":     {Flag, "add", `Git Revenge\bin`},
		"semicolon in folder": {Flag, "add", `C:\Tools;More\bin`},
		"empty folder":        {Flag, "add", ""},
		"extra argument":      {Flag, "add", bin, "more"},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			code, handled := Run(args, &stderr)
			if !handled || code != 1 || stderr.Len() == 0 {
				t.Fatalf("Run(%q) = %d, %v, stderr %q", args, code, handled, stderr.String())
			}
		})
	}
}
