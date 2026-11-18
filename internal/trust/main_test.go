package trust

import (
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// TestMain keeps the user's own git config out of the tests (see gittest.Main).
func TestMain(m *testing.M) { gittest.Main(m) }
