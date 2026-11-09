package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/git-revenge/gitrevenge/internal/gittest"
)

func TestStatusOf(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]string{"G": "good", "B": "bad", "U": "unknown", "E": "unknown", "X": "expired", "Y": "expired", "R": "revoked", "N": "none", "": "none"} {
		if got := statusOf(code); got != want {
			t.Errorf("statusOf(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestVerifySignaturesUnsigned(t *testing.T) {
	t.Parallel()
	fx := gittest.New(t)
	shas := fx.Linear(2)
	repo := OpenRepo(newTestRunner(t), fx.Dir)
	got, err := repo.VerifySignatures(context.Background(), shas)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range shas {
		if got[id].Status != "none" || got[id].Format != "" {
			t.Errorf("%s = %+v, want unsigned", id, got[id])
		}
	}
	if got, err := repo.VerifySignatures(context.Background(), nil); err != nil || len(got) != 0 {
		t.Fatalf("no ids = %v, %v", got, err)
	}
}

func TestVerifySignaturesSSH(t *testing.T) {
	t.Parallel()
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("ssh-keygen not installed")
	}
	fx := gittest.New(t)
	fx.Linear(1)
	key := filepath.Join(t.TempDir(), "id_ed25519")
	if out, err := exec.Command(keygen, "-q", "-t", "ed25519", "-N", "", "-C", "test@example.com", "-f", key).CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen: %v %s", err, out)
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	fx.Git("config", "gpg.format", "ssh")
	// No allowed signers, whatever the machine's global config says.
	fx.Git("config", "gpg.ssh.allowedSignersFile", "")
	fx.Git("config", "user.signingkey", key)
	fx.WriteFile("signed.txt", "x")
	fx.Git("add", "signed.txt")
	if _, err := fx.TryGit("commit", "-S", "-m", "signed"); err != nil {
		t.Skipf("ssh signing unavailable here: %v", err)
	}
	signed := fx.Head()
	repo := OpenRepo(newTestRunner(t), fx.Dir)

	// Without an allowed signers file git can't vouch for the key.
	got, err := repo.VerifySignatures(context.Background(), []string{signed})
	if err != nil {
		t.Fatal(err)
	}
	if got[signed].Status != "unknown" {
		t.Fatalf("unverifiable signature = %+v, want unknown", got[signed])
	}

	allowed := filepath.Join(t.TempDir(), "allowed_signers")
	if err := os.WriteFile(allowed, []byte("test@example.com "+string(pub)), 0o600); err != nil {
		t.Fatal(err)
	}
	fx.Git("config", "gpg.ssh.allowedSignersFile", allowed)
	got, err = repo.VerifySignatures(context.Background(), []string{signed})
	if err != nil {
		t.Fatal(err)
	}
	if c := got[signed]; c.Status != "good" || c.Format != "ssh" || c.Signer != "test@example.com" {
		t.Fatalf("signature = %+v, want good ssh by test@example.com", c)
	}
}
