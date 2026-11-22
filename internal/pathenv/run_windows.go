package pathenv

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Flag is the first argument that makes the app edit the PATH and exit instead of starting.
// The installer and uninstaller run it as:
//
//	gitrevenge.exe --installer-path add|remove <folder>
//
// It edits the current user's PATH and no other. The machine's PATH reaches every account on
// the machine, the system's too, and the folder may be one that others can write to (#895).
const Flag = "--installer-path"

// userEnvironment is where Windows keeps the current user's own environment variables.
const userEnvironment = "Environment"

// Run does what the installer asked for, when args start with Flag. It returns the process's
// exit code, and false when args are an ordinary start of the app.
func Run(args []string, stderr io.Writer) (code int, handled bool) {
	if len(args) == 0 || args[0] != Flag {
		return 0, false
	}
	if err := run(args[1:]); err != nil {
		_, _ = fmt.Fprintln(stderr, "gitrevenge:", err)
		return 1, true
	}
	return 0, true
}

func run(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: " + Flag + " add|remove <folder>")
	}
	var change func(path, dir string) (string, bool)
	switch args[0] {
	case "add":
		change = Add
	case "remove":
		change = Remove
	default:
		return fmt.Errorf("unknown action %q", args[0])
	}
	dir := filepath.Clean(args[1])
	// PATH separates its entries with semicolons, so it can't hold a folder with one in its name.
	if !filepath.IsAbs(dir) || strings.Contains(dir, ";") {
		return fmt.Errorf("%q can't be a PATH entry", args[1])
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, userEnvironment, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("opening the environment: %w", err)
	}
	defer func() { _ = key.Close() }()
	return edit(key, func(path string) (string, bool) { return change(path, dir) })
}

// edit rewrites key's Path value with change, as the type of value it already is. It writes
// nothing when change leaves the path alone, and refuses a value that isn't a string. A path
// that change empties is taken away: a user's PATH is often only what an installer added, and
// an empty value and none are the same to Windows, so an uninstall leaves things as they were.
func edit(key registry.Key, change func(path string) (string, bool)) error {
	path, kind, err := key.GetStringValue("Path")
	if errors.Is(err, registry.ErrNotExist) {
		// The type Windows itself gives PATH, so entries may use variables.
		path, kind, err = "", registry.EXPAND_SZ, nil
	}
	if err != nil {
		return fmt.Errorf("reading PATH: %w", err)
	}
	updated, changed := change(path)
	if !changed {
		return nil
	}
	switch {
	case updated == "":
		err = key.DeleteValue("Path")
	case kind == registry.SZ:
		err = key.SetStringValue("Path", updated)
	default:
		err = key.SetExpandStringValue("Path", updated)
	}
	if err != nil {
		return fmt.Errorf("writing PATH: %w", err)
	}
	return nil
}
