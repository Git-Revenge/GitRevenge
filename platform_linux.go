//go:build linux

package main

import (
	_ "embed"

	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed build/appicon.png
var linuxIcon []byte

// linuxOptions names and badges the window on Linux. ProgramName must match StartupWMClass in
// the packages' .desktop file, or desktops won't group the window with its launcher.
//
// Rendering stays in software, as it is when Wails has no Linux options, because hardware
// acceleration can leave a blank window on some NVIDIA and DMABUF setups.
func linuxOptions() *linux.Options {
	return &linux.Options{
		Icon:             linuxIcon,
		ProgramName:      "gitrevenge",
		WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
	}
}
