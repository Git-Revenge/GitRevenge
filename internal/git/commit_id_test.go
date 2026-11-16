package git

import (
	"strings"
	"testing"
)

func TestIsCommitID(t *testing.T) {
	t.Parallel()
	for s, want := range map[string]bool{
		strings.Repeat("a", 40):               true,
		strings.Repeat("0123456789abcdef", 4): true, // SHA-256
		strings.Repeat("a", 39):               false,
		strings.Repeat("a", 50):               false,
		strings.Repeat("A", 40):               false,
		strings.Repeat("g", 40):               false,
		"main":                                false,
		"":                                    false,
	} {
		if got := IsCommitID(s); got != want {
			t.Errorf("IsCommitID(%q) = %v, want %v", s, got, want)
		}
	}
}
