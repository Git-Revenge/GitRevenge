//go:build !darwin

package main

// carryOverWebStorage has nothing to do here: only macOS keeps the web view's data under the
// bundle ID that changed in 1.0.
func carryOverWebStorage() {}
