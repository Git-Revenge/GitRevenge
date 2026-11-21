package pathenv

import "testing"

const bin = `C:\Program Files\Git Revenge\bin`

func TestAdd(t *testing.T) {
	tests := []struct {
		name, path, want string
		changed          bool
	}{
		{"appends", `C:\Windows;C:\Tools`, `C:\Windows;C:\Tools;` + bin, true},
		{"empty path", "", bin, true},
		{"trailing semicolon", `C:\Windows;`, `C:\Windows;` + bin + `;`, true},
		{"keeps variables unexpanded", `%SystemRoot%\system32;%USERPROFILE%\bin`, `%SystemRoot%\system32;%USERPROFILE%\bin;` + bin, true},
		{"already there", `C:\Windows;` + bin + `;C:\Tools`, `C:\Windows;` + bin + `;C:\Tools`, false},
		{"already there in another case", `c:\program files\git revenge\BIN`, `c:\program files\git revenge\BIN`, false},
		{"already there with a trailing separator", bin + `\;C:\Tools`, bin + `\;C:\Tools`, false},
		{"already there quoted", `"` + bin + `"`, `"` + bin + `"`, false},
		{"a longer folder isn't it", bin + `2`, bin + `2;` + bin, true},
		{"its parent isn't it", `C:\Program Files\Git Revenge`, `C:\Program Files\Git Revenge;` + bin, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := Add(tt.path, bin)
			if got != tt.want || changed != tt.changed {
				t.Errorf("Add(%q) = %q, %v; want %q, %v", tt.path, got, changed, tt.want, tt.changed)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name, path, want string
		changed          bool
	}{
		{"last entry", `C:\Windows;C:\Tools;` + bin, `C:\Windows;C:\Tools`, true},
		{"first entry", bin + `;C:\Windows`, `C:\Windows`, true},
		{"middle entry", `C:\Windows;` + bin + `;C:\Tools`, `C:\Windows;C:\Tools`, true},
		{"only entry", bin, "", true},
		{"every copy", bin + `;C:\Windows;` + bin + `\`, `C:\Windows`, true},
		{"another case", `C:\Windows;c:\PROGRAM FILES\git revenge\bin`, `C:\Windows`, true},
		{"quoted", `C:\Windows;"` + bin + `"`, `C:\Windows`, true},
		{"not there", `C:\Windows;C:\Tools`, `C:\Windows;C:\Tools`, false},
		{"a longer folder stays", bin + `2;C:\Windows`, bin + `2;C:\Windows`, false},
		// What isn't removed keeps its exact spelling: empty entries, spaces, quotes, a
		// trailing semicolon and variables.
		{"the rest is untouched", `C:\Windows;;  C:\Sp ace ;"C:\a;b";%SystemRoot%\x\;` + bin + `;`, `C:\Windows;;  C:\Sp ace ;"C:\a;b";%SystemRoot%\x\;`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := Remove(tt.path, bin)
			if got != tt.want || changed != tt.changed {
				t.Errorf("Remove(%q) = %q, %v; want %q, %v", tt.path, got, changed, tt.want, tt.changed)
			}
		})
	}
}

func TestAddThenRemoveGivesBackThePath(t *testing.T) {
	for _, path := range []string{"", `C:\Windows`, `C:\Windows;`, `%SystemRoot%;;"C:\a;b"`} {
		added, _ := Add(path, bin)
		if got, _ := Remove(added, bin); got != path {
			t.Errorf("after adding to and removing from %q: %q", path, got)
		}
	}
}
