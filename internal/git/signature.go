package git

import (
	"context"
	"strings"
)

// SignatureCheck is git's verdict on a commit's signature (GPG, SSH or X.509), as
// `git log --format=%G?` reports it, with the signer and key when there's a signature.
type SignatureCheck struct {
	ID string `json:"id"`
	// Status is "good", "bad", "unknown" (valid but the key isn't trusted or can't be
	// checked), "expired" (expired signature or key), "revoked" or "none".
	Status string `json:"status"`
	Signer string `json:"signer"`
	Key    string `json:"key"`
	// Format is "gpg", "ssh" or "x509" when known.
	Format string `json:"format"`
}

// statusOf maps git's %G? letter to a SignatureCheck status.
func statusOf(code string) string {
	switch code {
	case "G":
		return "good"
	case "B":
		return "bad"
	case "U", "E":
		return "unknown"
	case "X", "Y":
		return "expired"
	case "R":
		return "revoked"
	}
	return "none"
}

// VerifySignatures checks the signatures of the given commits with git's own verification,
// which uses the user's gpg, gpg.ssh.allowedSignersFile and gpgsm configuration.
func (r *Repo) VerifySignatures(ctx context.Context, ids []string) (map[string]SignatureCheck, error) {
	if err := CheckRevs(ids...); err != nil {
		return nil, err
	}
	out := make(map[string]SignatureCheck, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	args := append([]string{"log", "--no-walk=unsorted", "--format=%H%x1f%G?%x1f%GS%x1f%GK%x1f%GF%x1f%GG%x1e"}, ids...)
	text, err := r.ReadString(ctx, append(args, "--")...)
	if err != nil {
		return nil, err
	}
	for rec := range strings.SplitSeq(text, "\x1e") {
		f := strings.Split(strings.TrimLeft(rec, "\n"), "\x1f")
		if len(f) < 6 || f[0] == "" {
			continue
		}
		c := SignatureCheck{ID: f[0], Status: statusOf(f[1]), Signer: f[2], Key: f[3]}
		// git says "no signature" (N) when it can't run the check at all, e.g. an SSH
		// signature with no gpg.ssh.allowedSignersFile; its verification output then holds
		// the complaint. The commit is signed, just not checkable.
		if c.Status == "none" && strings.TrimSpace(f[5]) != "" {
			c.Status = "unknown"
		}
		if c.Status != "none" {
			c.Format = formatOf(f[3], f[4])
		}
		out[f[0]] = c
	}
	return out, nil
}

// formatOf guesses the signature format from the key and fingerprint git reports: SSH
// fingerprints start with "SHA256:", X.509 keys are long hex serials, the rest are GPG.
func formatOf(key, fingerprint string) string {
	switch {
	case strings.HasPrefix(fingerprint, "SHA256:") || strings.HasPrefix(key, "SHA256:"):
		return "ssh"
	case fingerprint == "" && len(key) > 16:
		return "x509"
	}
	return "gpg"
}
