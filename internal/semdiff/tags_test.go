package semdiff

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// grammarPackages are the gotreesitter grammar packages lang.go imports, by language id.
var grammarPackages = map[string]string{
	"go": "go", "javascript": "javascript", "typescript": "typescript", "tsx": "tsx",
	"python": "python", "csharp": "c_sharp", "java": "java", "css": "css", "scss": "scss",
	"json": "json", "rust": "rust", "vue": "vue", "html": "html", "lua": "lua", "xml": "xml",
}

// TestBuildTagsMatchTheLanguages checks build/tags, which every build that ships uses: it
// keeps copyleft grammar code out (gotreesitter links the Nim, Caddy and disassembly
// scanners otherwise), and a language left out of it wouldn't compile into the app.
func TestBuildTagsMatchTheLanguages(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "build", "tags"))
	if err != nil {
		t.Fatal(err)
	}
	tags := strings.Split(strings.TrimSpace(string(raw)), ",")
	for _, must := range []string{"grammar_subset", "gotreesitter_no_copyleft"} {
		if !slices.Contains(tags, must) {
			t.Errorf("build/tags lacks %s", must)
		}
	}
	want := map[string]bool{"grammar_subset": true, "gotreesitter_no_copyleft": true}
	for _, l := range languages {
		pkg, ok := grammarPackages[l.id]
		if !ok {
			t.Fatalf("no grammar package recorded for %s", l.id)
		}
		want["grammar_subset_"+pkg] = true
		if !slices.Contains(tags, "grammar_subset_"+pkg) {
			t.Errorf("build/tags lacks grammar_subset_%s, so %s wouldn't build into the app", pkg, l.name)
		}
	}
	for _, tag := range tags {
		if !want[tag] {
			t.Errorf("build/tags has %q, which no language needs", tag)
		}
	}
}
