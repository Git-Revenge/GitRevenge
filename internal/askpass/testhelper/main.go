// Command testhelper is a stand-in for the gitrevenge executable in askpass tests: it only
// runs the helper side, so tests don't need to build the whole app.
package main

import (
	"os"

	"github.com/git-revenge/gitrevenge/internal/askpass"
)

func main() {
	os.Exit(askpass.RunHelper(os.Stdout, os.Stderr))
}
