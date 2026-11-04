//go:build !linux

package main

import "github.com/wailsapp/wails/v2/pkg/options/linux"

// linuxOptions is only needed on Linux.
func linuxOptions() *linux.Options { return nil }
