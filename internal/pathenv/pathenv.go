// Package pathenv puts a folder on the current user's Windows PATH, or takes it off, for the
// installer.
//
// The installer can't do it itself. NSIS reads a registry string into a 1024-character
// buffer, so a longer PATH comes back empty and would be written back that way. Here the
// value is read and written whole, keeps its type, and every other entry stays as it was,
// byte for byte.
package pathenv

import "strings"

// Add returns path with dir as its last entry, and whether that changed it. A path that
// already has dir is returned as it is.
func Add(path, dir string) (string, bool) {
	for entry := range strings.SplitSeq(path, ";") {
		if same(entry, dir) {
			return path, false
		}
	}
	switch {
	case path == "":
		return dir, true
	case strings.HasSuffix(path, ";"):
		// The semicolon stays at the end, so removing dir gives back the path exactly.
		return path + dir + ";", true
	}
	return path + ";" + dir, true
}

// Remove returns path without its entries for dir, and whether that changed it.
func Remove(path, dir string) (string, bool) {
	entries := strings.Split(path, ";")
	kept := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !same(entry, dir) {
			kept = append(kept, entry)
		}
	}
	if len(kept) == len(entries) {
		return path, false
	}
	return strings.Join(kept, ";"), true
}

// same reports whether a PATH entry names dir. Windows paths ignore case, and an entry may
// be quoted or end in a separator.
func same(entry, dir string) bool {
	return strings.EqualFold(bare(entry), bare(dir))
}

func bare(entry string) string {
	entry = strings.Trim(strings.TrimSpace(entry), `"`)
	return strings.TrimRight(entry, `\/`)
}
