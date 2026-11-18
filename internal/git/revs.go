package git

import (
	"fmt"
	"strings"
)

// CheckRevs refuses any value git would read as an option: one that starts with "-". It
// returns an error that is ErrInvalidName, naming the first such value.
//
// Branch, tag, remote and revision names reach git as plain arguments, and a name isn't
// always the user's own: "refs/tags/--output=x" is a valid ref, and arrives with a clone or a
// fetch from whoever controls the remote. Passed where a revision is expected, git takes it
// for the option it spells. So every function here that puts a name it was given on git's
// command line checks it first, and so must any caller that runs git directly with one.
//
// An empty value passes: callers treat it as "none" or as HEAD.
func CheckRevs(values ...string) error {
	for _, v := range values {
		if strings.HasPrefix(v, "-") {
			return fmt.Errorf("%q would be read by git as an option, not a name: %w", v, ErrInvalidName)
		}
	}
	return nil
}
