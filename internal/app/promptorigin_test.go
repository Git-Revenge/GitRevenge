package app

import "testing"

func TestPromptHost(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]string{
		"Username for 'https://github.com': ":                                                       "github.com",
		"Password for 'https://user@GitHub.com': ":                                                  "github.com",
		"Password for 'https://user@git.example.com:8443': ":                                        "git.example.com",
		"git@gitlab.example.com's password: ":                                                       "gitlab.example.com",
		"The authenticity of host 'github.com (140.82.121.4)' can't be established.":                "github.com",
		"The authenticity of host '[git.example.com]:2222 ([10.0.0.1]:2222)' can't be established.": "git.example.com",
		"Enter passphrase for key '/home/u/.ssh/id_ed25519': ":                                      "",
		"Password: ": "",
	} {
		if got := promptHost(text); got != want {
			t.Errorf("promptHost(%q) = %q, want %q", text, got, want)
		}
	}
}

// The dialog says which operation asked and whether the server the prompt names is one the
// operation was set up to talk to.
func TestPromptSaysWhereItCameFrom(t *testing.T) {
	t.Parallel()
	asked := 0
	from := promptOrigin{Op: "fetch", Path: "/work/tidewater", Hosts: func() []string { asked++; return []string{"git.example.com"} }}

	own := Prompt{Text: "Password for 'https://user@git.example.com': "}
	from.describe(&own)
	if own.Op != "fetch" || own.Repo != "tidewater" || own.Host != "git.example.com" || own.Unexpected {
		t.Errorf("own server: %+v", own)
	}

	// A server the repository has no remote on, named by a prompt the server itself wrote.
	other := Prompt{Text: "Password for 'https://github.com': "}
	from.describe(&other)
	if other.Host != "github.com" || !other.Unexpected {
		t.Errorf("another server: %+v", other)
	}

	// No server named: nothing to compare, and the remotes aren't read.
	asked = 0
	key := Prompt{Text: "Enter passphrase for key '/home/u/.ssh/id_ed25519': "}
	from.describe(&key)
	if key.Host != "" || key.Unexpected || asked != 0 {
		t.Errorf("key passphrase: %+v, remotes read %d times", key, asked)
	}
}
