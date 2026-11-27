// Command gitrevenge is the Git Revenge desktop git client.
package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/git-revenge/gitrevenge/internal/app"
	"github.com/git-revenge/gitrevenge/internal/askpass"
	"github.com/git-revenge/gitrevenge/internal/pathenv"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Started by git or ssh to ask for a credential: relay it to the running app and exit.
	if askpass.IsHelper() {
		os.Exit(askpass.RunHelper(os.Stdout, os.Stderr))
	}

	// Started by the Windows installer or uninstaller to put the launcher's folder on the
	// PATH, or take it off: do that and exit.
	if code, handled := pathenv.Run(os.Args[1:], os.Stderr); handled {
		os.Exit(code)
	}

	carryOverWebStorage()

	a := app.New()

	err := wails.Run(&options.App{
		Title:     "Git Revenge",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 500,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 26, G: 22, B: 20, A: 1},
		// Files dropped on the window open the patches preview (#1025). Not DisableWebViewDrop:
		// on Windows it turns WebView2's AllowExternalDrop off, which refuses every file dropped
		// from outside. The page never navigates to a dropped file anyway (linkGuard.ts, and
		// Wails' own drop handler).
		DragAndDrop:   &options.DragAndDrop{EnableFileDrop: true},
		Linux:         linuxOptions(),
		OnStartup:     a.Startup,
		OnBeforeClose: beforeClose,
		OnShutdown:    a.Shutdown,
		Bind: []any{
			a,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
