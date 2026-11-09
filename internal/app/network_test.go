package app

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/askpass"
	"github.com/git-revenge/gitrevenge/internal/git"
	"github.com/git-revenge/gitrevenge/internal/gittest"
)

// isolateGit keeps the user's git config (and credential helpers such as Git Credential
// Manager, which could pop up a window) out of these tests.
func isolateGit(t *testing.T) {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = Test\n\temail = t@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func TestPushCurrentPublishesNewBranch(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	tab, _ := a.OpenRepo(local.Dir)
	local.Branch("feature")
	local.CommitFile("f.txt", "f", "feature")

	if err := a.PushCurrent(tab.Path, false); err != nil {
		t.Fatal(err)
	}
	if remote.Git("rev-parse", "feature") != local.Head() {
		t.Fatal("branch not pushed")
	}
	if got := local.Git("rev-parse", "--abbrev-ref", "feature@{upstream}"); got != "origin/feature" {
		t.Fatalf("upstream = %q", got)
	}
}

func TestPushRejectedIsExplained(t *testing.T) {
	isolateGit(t)
	a := newTestApp(t)
	local, remote := gittest.WithRemote(t, 1)
	other := gittest.Clone(t, remote)
	other.CommitFile("o.txt", "o", "other")
	other.Git("push", "-q", "origin", "main")
	tab, _ := a.OpenRepo(local.Dir)
	local.CommitFile("l.txt", "l", "mine")

	err := a.PushCurrent(tab.Path, false)
	if err == nil || !strings.Contains(err.Error(), "Pull first") {
		t.Fatalf("err = %v", err)
	}
}

// buildHelper builds a stand-in for the gitrevenge executable's askpass mode.
func buildHelper(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "askpass-helper")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", exe, "../askpass/testhelper").CombinedOutput(); err != nil {
		t.Fatalf("building helper: %v\n%s", err, out)
	}
	return exe
}

// End to end: fetching over HTTP with basic auth asks for a username and password through
// the askpass bridge.
func TestFetchAsksForCredentials(t *testing.T) {
	isolateGit(t)
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	bare := gittest.NewBare(t)
	seed := gittest.New(t)
	seed.Linear(2)
	seed.AddRemote("origin", bare)
	seed.Git("push", "-q", "origin", "main")

	// Generated per run so the source holds no credential-looking literals.
	user, pass := "user-"+randomToken(t), "pass-"+randomToken(t)
	backend := &cgi.Handler{
		Path: gitBin,
		Args: []string{"http-backend"},
		Env:  []string{"GIT_PROJECT_ROOT=" + filepath.Dir(bare.Dir), "GIT_HTTP_EXPORT_ALL=1"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	defer srv.Close()

	a := newTestApp(t)
	var mu sync.Mutex
	var asked []string
	a.promptHook = func(_ string, kind askpass.Kind) (string, bool) {
		mu.Lock()
		asked = append(asked, string(kind))
		mu.Unlock()
		if kind == askpass.Text {
			return user, true
		}
		return pass, true
	}
	a.net.exe = buildHelper(t) // set before askEnv runs
	local := gittest.New(t)
	local.Git("remote", "add", "origin", srv.URL+"/"+filepath.Base(bare.Dir))
	tab, err := a.OpenRepo(local.Dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := a.Fetch(tab.Path, "origin"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if local.Git("rev-parse", "origin/main") != seed.Head() {
		t.Fatal("fetch did not bring origin/main")
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(asked, ",") != "text,secret" {
		t.Fatalf("prompts = %v", asked)
	}
}

func TestNetErrHTTPStatus(t *testing.T) {
	t.Parallel()
	err := netErr(&git.Error{Stderr: "fatal: unable to access 'http://x/repo.git/': The requested URL returned error: 403"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("err = %v", err)
	}
}

func randomToken(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
