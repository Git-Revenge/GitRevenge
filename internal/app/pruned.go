package app

import (
	"fmt"
	"slices"
	"strings"
)

// droppedMessage says which remote-tracking branches a fetch or pull dropped because their
// remote deleted the branch, so a branch leaving the Remotes panel is never silent (#923). It's
// "" when none went. what names what was fetched: a remote, or "" for every remote.
func droppedMessage(what string, before, after map[string]bool) string {
	var gone []string
	for name := range before {
		if !after[name] {
			gone = append(gone, name)
		}
	}
	if len(gone) == 0 {
		return ""
	}
	slices.Sort(gone)
	fetched := "Fetched every remote."
	if what != "" {
		fetched = "Fetched " + what + "."
	}
	names := strings.Join(gone, ", ")
	if len(gone) > 3 {
		names = strings.Join(gone[:2], ", ") + fmt.Sprintf(" and %d more", len(gone)-2)
	}
	if len(gone) == 1 {
		return fetched + " 1 deleted branch dropped: " + names + "."
	}
	return fmt.Sprintf("%s %d deleted branches dropped: %s.", fetched, len(gone), names)
}
