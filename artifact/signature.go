package artifact

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SignatureResult holds the outcome of a GPG signature verification.
type SignatureResult struct {
	// Valid is true only when the signature is cryptographically good AND
	// the signing key's fingerprint matches PinnedFingerprint.
	Valid       bool
	Fingerprint string
	SignerUID   string
	// Reason describes why verification failed, when Valid is false.
	// Distinct reasons: "digest mismatch" (from digest.go),
	// "bad signature", "unknown key", "fingerprint mismatch: got <fpr>".
	Reason string
}

// VerifySignature checks a detached GPG signature against the binary,
// accepting only the fingerprint pinned in trustedkeys.go.
//
// GPG runs inside an ephemeral GNUPGHOME under sandboxDir so it never
// touches the operator's real keyring and cannot be influenced by keys
// already trusted on the host.
//
// The signing key is NOT imported from the release under test. Instead,
// the key must already be importable from a source the caller controls.
// ImportPinnedKey must be called first to populate the ephemeral keyring
// with a key whose fingerprint matches PinnedFingerprint.
//
// --status-fd is parsed instead of human-readable output because GPG's
// status protocol is stable across versions and locales.
func VerifySignature(binaryPath, sigPath, sandboxDir string) (*SignatureResult, error) {
	gnupgHome := filepath.Join(sandboxDir, "gnupg-verify")
	if err := os.MkdirAll(gnupgHome, 0700); err != nil {
		return nil, fmt.Errorf("create GNUPGHOME: %w", err)
	}

	// gpg --status-fd 3 --verify <sig> <binary>
	// status output goes to fd 3, which we capture via a pipe.
	cmd := exec.Command("gpg",
		"--homedir", gnupgHome,
		"--batch",
		"--no-default-keyring",
		"--keyring", filepath.Join(gnupgHome, "trusted.gpg"),
		"--status-fd", "1",
		"--verify", sigPath, binaryPath,
	)

	// Suppress stderr (human-readable GPG output we don't parse).
	cmd.Stderr = nil

	var statusBuf bytes.Buffer
	cmd.Stdout = &statusBuf

	err := cmd.Run()

	return parseStatusOutput(statusBuf.String(), err)
}

// ImportPinnedKey imports a public key into the ephemeral keyring, but only
// if its fingerprint matches PinnedFingerprint. This is the sole import path;
// there is no function that imports an arbitrary key.
func ImportPinnedKey(keyData []byte, sandboxDir string) error {
	gnupgHome := filepath.Join(sandboxDir, "gnupg-verify")
	if err := os.MkdirAll(gnupgHome, 0700); err != nil {
		return fmt.Errorf("create GNUPGHOME: %w", err)
	}

	// Import into a temporary keyring first, then check the fingerprint.
	tmpHome := filepath.Join(sandboxDir, "gnupg-import-check")
	if err := os.MkdirAll(tmpHome, 0700); err != nil {
		return fmt.Errorf("create temp GNUPGHOME: %w", err)
	}
	defer os.RemoveAll(tmpHome)

	// Import the key into the temporary keyring.
	importCmd := exec.Command("gpg",
		"--homedir", tmpHome,
		"--batch",
		"--quiet",
		"--import",
	)
	importCmd.Stdin = bytes.NewReader(keyData)
	importCmd.Stderr = nil
	if out, err := importCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("import key for checking: %w: %s", err, out)
	}

	// Read the fingerprint of the imported key.
	fprCmd := exec.Command("gpg",
		"--homedir", tmpHome,
		"--batch",
		"--with-colons",
		"--fingerprint",
	)
	fprOut, err := fprCmd.Output()
	if err != nil {
		return fmt.Errorf("read fingerprint: %w", err)
	}

	fpr := extractFingerprint(string(fprOut))
	if fpr == "" {
		return fmt.Errorf("no fingerprint found in imported key")
	}

	if fpr != PinnedFingerprint {
		return fmt.Errorf(
			"imported key fingerprint %s does not match pinned %s: refusing to trust",
			fpr, PinnedFingerprint,
		)
	}

	// Fingerprint verified — import into the real verification keyring.
	realImport := exec.Command("gpg",
		"--homedir", gnupgHome,
		"--batch",
		"--quiet",
		"--no-default-keyring",
		"--keyring", filepath.Join(gnupgHome, "trusted.gpg"),
		"--import",
	)
	realImport.Stdin = bytes.NewReader(keyData)
	realImport.Stderr = nil
	if out, err := realImport.CombinedOutput(); err != nil {
		return fmt.Errorf("import verified key: %w: %s", err, out)
	}

	return nil
}

// extractFingerprint finds the first fpr: record in --with-colons output.
func extractFingerprint(colonOutput string) string {
	scanner := bufio.NewScanner(strings.NewReader(colonOutput))
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) >= 10 && fields[0] == "fpr" {
			return fields[9]
		}
	}
	return ""
}

// parseStatusOutput interprets gpg --status-fd output.
//
// Key status lines (confirmed against GPG 2.4.8):
//   [GNUPG:] GOODSIG <keyid> <uid>
//   [GNUPG:] VALIDSIG <fpr> <date> <ts> ... <primary-fpr>
//   [GNUPG:] BADSIG <keyid> <uid>
//   [GNUPG:] ERRSIG <keyid> <algo> ... <fpr>
//   [GNUPG:] NO_PUBKEY <keyid>
func parseStatusOutput(status string, gpgErr error) (*SignatureResult, error) {
	result := &SignatureResult{}

	scanner := bufio.NewScanner(strings.NewReader(status))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "[GNUPG:] ") {
			continue
		}
		payload := strings.TrimPrefix(line, "[GNUPG:] ")
		fields := strings.Fields(payload)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "GOODSIG":
			if len(fields) >= 3 {
				result.SignerUID = strings.Join(fields[2:], " ")
			}
		case "VALIDSIG":
			if len(fields) >= 2 {
				result.Fingerprint = fields[1]
			}
		case "BADSIG":
			result.Reason = "bad signature"
			if len(fields) >= 3 {
				result.SignerUID = strings.Join(fields[2:], " ")
			}
			return result, nil
		case "NO_PUBKEY":
			result.Reason = "unknown key"
			return result, nil
		case "ERRSIG":
			// ERRSIG includes the full fingerprint as the last field
			// when GPG 2.4 has it available.
			if len(fields) >= 8 {
				result.Fingerprint = fields[7]
			}
			result.Reason = "unknown key"
			return result, nil
		}
	}

	if result.Fingerprint == "" {
		if gpgErr != nil {
			return result, fmt.Errorf("gpg verification failed: %w", gpgErr)
		}
		return result, fmt.Errorf("gpg produced no VALIDSIG in status output")
	}

	// Enforce the pinned fingerprint. A valid signature from any other key
	// is rejected with a distinct reason.
	if result.Fingerprint != PinnedFingerprint {
		result.Reason = fmt.Sprintf("fingerprint mismatch: got %s", result.Fingerprint)
		return result, nil
	}

	result.Valid = true
	return result, nil
}
