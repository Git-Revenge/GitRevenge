package gittest

import "testing"

// TestMain keeps the user's own git config out of the tests (see Main).
func TestMain(m *testing.M) { Main(m) }
