package app

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/git-revenge/gitrevenge/internal/git"
)

// Patches from a dropped file or the clipboard (#1025): the same previews as Apply patches or
// a diff… (#1022, #1024), reached without the file picker.

// DroppedFiles is the "patches:dropped" event: files dropped on the window, each now one the
// backend may read.
type DroppedFiles struct {
	Paths []string `json:"paths"`
}

// filesDropped takes files dropped on the window as chosen, as a file dialog's pick is, and
// tells the window. Being chosen first, they're readable by the time the window asks.
func (a *App) filesDropped(paths []string) {
	var files []string
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			a.chosen.add(p)
			files = append(files, p)
		}
	}
	if len(files) > 0 {
		a.emit("patches:dropped", DroppedFiles{Paths: files})
	}
}

// ReadPatchFiles reads files dropped together for the preview. Several are one series, in name
// order (0001-, 0002-…), or one diff. Only files dropped or picked are read.
func (a *App) ReadPatchFiles(path string, files []string) (PatchFile, error) {
	if len(files) == 1 {
		return a.ReadPatches(path, files[0])
	}
	if len(files) == 0 {
		return PatchFile{}, userError("Drop a file of patches or a diff.")
	}
	r, err := a.openRepo(path)
	if err != nil {
		return PatchFile{}, err
	}
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(x, y string) int { return strings.Compare(filepath.Base(x), filepath.Base(y)) })
	var data []byte
	kinds := map[string][]string{}
	for _, f := range sorted {
		if !a.chosen.has(f) {
			return PatchFile{}, userError("Choose the file of patches first.")
		}
		info, err := os.Stat(f)
		if err != nil {
			return PatchFile{}, userError("That file can't be read: " + f)
		}
		if int64(len(data))+info.Size() > maxPatchFile {
			return PatchFile{}, userError("Those files are too big to read as patches together.")
		}
		text, err := os.ReadFile(f)
		if err != nil {
			return PatchFile{}, writeErr(err)
		}
		kind := git.PatchFileKind(text)
		kinds[kind] = append(kinds[kind], filepath.Base(f))
		if len(data) > 0 && data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		data = append(data, text...)
	}
	if neither := kinds[""]; len(neither) > 0 {
		return PatchFile{Name: filesSaid(neither), Kind: "", Messages: []git.MailMessage{}}, nil
	}
	if len(kinds) > 1 {
		return PatchFile{}, userError("Those are patches and diffs together. Drop the patches, or the diffs, on their own.")
	}
	name := filepath.Base(sorted[0]) + " and " + strconv.Itoa(len(sorted)-1) + " more"
	return a.readPatchData(r, name, data)
}

// ReadClipboardPatches reads the clipboard's text for the preview, as Apply the patch on the
// clipboard does: the pair to Copy as a patch, or a patch copied from a mail or a web page.
func (a *App) ReadClipboardPatches(path string) (PatchFile, error) {
	r, err := a.openRepo(path)
	if err != nil {
		return PatchFile{}, err
	}
	text, err := a.clipboardText()
	if err != nil {
		return PatchFile{}, userError("The clipboard can't be read: " + err.Error())
	}
	if len(text) > maxPatchFile {
		return PatchFile{}, userError("The clipboard holds too much to read as patches.")
	}
	// Copied text can carry Windows line ends; git's mail and diff readers want git's own.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return a.readPatchData(r, "the clipboard", []byte(text))
}

func (a *App) clipboardText() (string, error) {
	if a.clipboardHook != nil {
		return a.clipboardHook()
	}
	return runtime.ClipboardGetText(a.ctx)
}
