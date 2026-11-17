package askpass

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := map[string]Kind{
		"Username for 'https://github.com': ":                                   Text,
		"Password for 'https://user@github.com': ":                              Secret,
		"Enter passphrase for key '/home/u/.ssh/id_ed25519': ":                  Secret,
		"Are you sure you want to continue connecting (yes/no/[fingerprint])? ": HostKey,
		"something unexpected":                                                  Secret,
		// ssh's whole question, as it arrives.
		"The authenticity of host 'github.com (140.82.121.4)' can't be established.\nED25519 key fingerprint is SHA256:abc.\nAre you sure you want to continue connecting (yes/no/[fingerprint])? ": HostKey,
		"Are you sure you want to continue connecting (yes/no)? ": HostKey,
		// A server's own prompt that only borrows the words is a password field, never a "yes" button.
		"To continue connecting, enter your GitHub password: ": Secret,
	}
	for prompt, want := range cases {
		if got := Classify(prompt); got != want {
			t.Errorf("Classify(%q) = %s, want %s", prompt, got, want)
		}
	}
}

func TestClientServerRoundTrip(t *testing.T) {
	var got []string
	var mu sync.Mutex
	s, err := Listen(func(_ context.Context, prompt string, kind Kind, from any) (string, bool) {
		mu.Lock()
		got = append(got, fmt.Sprint(from)+":"+string(kind)+":"+prompt)
		mu.Unlock()
		if strings.Contains(prompt, "cancel") {
			return "", false
		}
		return "hunter2", true
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	open := func(from string) (map[string]string, func()) {
		vars, done, err := s.Open("exe", from)
		if err != nil {
			t.Fatal(err)
		}
		env := map[string]string{}
		for _, kv := range vars {
			k, v, _ := strings.Cut(kv, "=")
			env[k] = v
		}
		return env, done
	}
	env, done := open("fetch")
	other, otherDone := open("push")
	defer otherDone()
	if env[EnvToken] == other[EnvToken] {
		t.Fatal("two operations share a token")
	}

	ans, err := Client(env[EnvAddr], env[EnvToken], "Password: ")
	if err != nil || ans != "hunter2" {
		t.Fatalf("answer = %q, %v", ans, err)
	}
	if _, err := Client(env[EnvAddr], env[EnvToken], "please cancel"); !errors.Is(err, ErrCancelled) {
		t.Fatalf("cancel err = %v", err)
	}
	if _, err := Client(env[EnvAddr], "wrong-token", "Password: "); err == nil {
		t.Fatal("a wrong token must not get an answer")
	}
	// Each prompt arrives with what its own operation was opened as.
	if _, err := Client(other[EnvAddr], other[EnvToken], "Password: "); err != nil {
		t.Fatal(err)
	}
	// An operation that has ended gets no more answers, and the other one still does.
	done()
	if _, err := Client(env[EnvAddr], env[EnvToken], "Password: "); err == nil {
		t.Fatal("a finished operation's token must not get an answer")
	}
	if _, err := Client(other[EnvAddr], other[EnvToken], "Password: "); err != nil {
		t.Fatalf("the other operation stopped being answered: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"fetch:secret:Password: ", "fetch:secret:please cancel", "push:secret:Password: ", "push:secret:Password: "}
	if !slices.Equal(got, want) {
		t.Fatalf("handler saw %v, want %v", got, want)
	}
}

// End to end: git asks for HTTPS credentials through GIT_ASKPASS, our helper relays them to
// the server, and git receives the answers.
func TestGitUsesHelper(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	helper := filepath.Join(t.TempDir(), "helper")
	if runtime.GOOS == "windows" {
		helper += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", helper, "./testhelper").CombinedOutput(); err != nil {
		t.Fatalf("building helper: %v\n%s", err, out)
	}

	// Generated per run so the source holds no credential-looking literals.
	user, pass := "user-"+randomHex(t), "pass-"+randomHex(t)
	s, err := Listen(func(_ context.Context, _ string, kind Kind, _ any) (string, bool) {
		if kind == Text {
			return user, true
		}
		return pass, true
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	// git credential fill prompts for anything the input doesn't provide.
	cmd := exec.Command(gitBin, "-c", "credential.helper=", "credential", "fill")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=example.com\n\n")
	global := filepath.Join(t.TempDir(), "gitconfig")
	_ = os.WriteFile(global, nil, 0o600)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+global, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	env, done, err := s.Open(helper, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("credential fill: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "username="+user) || !strings.Contains(string(out), "password="+pass) {
		t.Fatalf("git got: %s", out)
	}
}

func randomHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
