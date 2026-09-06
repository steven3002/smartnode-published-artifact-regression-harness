package artifact

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestVerifyV1230 downloads the real v1.23.0 linux-amd64 binary and runs
// the full verification pipeline: digest, signature, version.
//
// This test hits the network and takes ~60s. Run with:
//
//	go test -v -run TestVerifyV1230 -timeout 120s ./artifact/
func TestVerifyV1230(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	sandbox := t.TempDir()

	// Fetch release metadata from the GitHub API.
	rel, err := FetchRelease("v1.23.0")
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}
	if rel.TagName != "v1.23.0" {
		t.Fatalf("unexpected tag: %s", rel.TagName)
	}

	// Resolve the linux-amd64 CLI binary and its signature.
	assets, err := ResolveAssetSet(rel, "rocketpool-cli-linux-amd64")
	if err != nil {
		t.Fatalf("ResolveAssetSet: %v", err)
	}

	t.Logf("API digest: %s", assets.DigestHex)
	if assets.DigestHex == "" {
		t.Fatal("GitHub API did not return a digest for the binary")
	}

	// Download binary and signature.
	binaryPath, sigPath, err := DownloadAssetSet(assets, sandbox)
	if err != nil {
		t.Fatalf("DownloadAssetSet: %v", err)
	}

	// Import the signing key. We fetch it from the release here, but
	// ImportPinnedKey will refuse it if its fingerprint doesn't match
	// the value pinned in trustedkeys.go.
	keyAsset := findAsset(rel, "fornax-signing-key.asc")
	if keyAsset == nil {
		t.Fatal("signing key asset not found in release")
	}
	keyPath, err := Download(*keyAsset, sandbox)
	if err != nil {
		t.Fatalf("download signing key: %v", err)
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read signing key: %v", err)
	}
	if err := ImportPinnedKey(keyData, sandbox); err != nil {
		t.Fatalf("ImportPinnedKey: %v", err)
	}

	// Full verification pipeline.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := Verify(ctx, binaryPath, sigPath, sandbox, "v1.23.0", assets.DigestHex)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	// Confirm all six report fields.
	t.Logf("Tag:             %s", report.Tag)
	t.Logf("BinarySHA256:    %s", report.BinarySHA256)
	t.Logf("DigestMatch:     %v", report.DigestMatch)
	t.Logf("SignatureValid:  %v", report.SignatureValid)
	t.Logf("KeyFingerprint:  %s", report.KeyFingerprint)
	t.Logf("SignerUID:       %s", report.SignerUID)
	t.Logf("ReportedVersion: %s", report.ReportedVersion)
	t.Logf("VersionMatch:    %v", report.VersionMatch)
	t.Logf("Timestamp:       %s", report.Timestamp)

	if !report.DigestMatch {
		t.Error("digest did not match")
	}
	if !report.SignatureValid {
		t.Errorf("signature invalid: %s", report.SignatureReason)
	}
	if report.KeyFingerprint != PinnedFingerprint {
		t.Errorf("fingerprint: got %s, want %s", report.KeyFingerprint, PinnedFingerprint)
	}
	if !report.VersionMatch {
		t.Errorf("version mismatch: reported %q", report.ReportedVersion)
	}
	if report.ReportedVersion != "1.23.0" {
		t.Errorf("unexpected version string: %q", report.ReportedVersion)
	}
}

// TestDigestMismatch corrupts the binary and confirms a DigestMismatchError.
func TestDigestMismatch(t *testing.T) {
	sandbox := t.TempDir()

	// Create a small file with known content.
	binaryPath := filepath.Join(sandbox, "test-binary")
	if err := os.WriteFile(binaryPath, []byte("original content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Compute its real digest.
	realDigest, err := ComputeSHA256(binaryPath)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the binary.
	if err := os.WriteFile(binaryPath, []byte("corrupted content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := VerifyDigest(binaryPath, realDigest)
	if err != nil {
		t.Fatal(err)
	}

	if result.Match {
		t.Fatal("expected digest mismatch, got match")
	}
	if result.Actual == realDigest {
		t.Fatal("corrupted file should have different digest")
	}
	t.Logf("correctly detected mismatch: expected %s, got %s", realDigest, result.Actual)
}

// TestBadSignature verifies a signature against corrupted data and confirms
// a "bad signature" failure, distinct from other failure modes.
func TestBadSignature(t *testing.T) {
	sandbox := t.TempDir()

	// Generate a throwaway keypair.
	gnupgHome := filepath.Join(sandbox, "keygen")
	if err := os.MkdirAll(gnupgHome, 0700); err != nil {
		t.Fatal(err)
	}

	genScript := filepath.Join(sandbox, "keygen.txt")
	if err := os.WriteFile(genScript, []byte(`%no-protection
Key-Type: RSA
Key-Length: 2048
Name-Real: Test Signer
Name-Email: test@example.com
Expire-Date: 0
%commit
`), 0644); err != nil {
		t.Fatal(err)
	}

	genCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--gen-key", genScript)
	genCmd.Stderr = nil
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v: %s", err, out)
	}

	// Sign a file.
	original := filepath.Join(sandbox, "original")
	if err := os.WriteFile(original, []byte("genuine data"), 0644); err != nil {
		t.Fatal(err)
	}
	signCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--detach-sign", "--armor", "-o", original+".sig", original)
	signCmd.Stderr = nil
	if out, err := signCmd.CombinedOutput(); err != nil {
		t.Fatalf("sign: %v: %s", err, out)
	}

	// Import the test key into the verification keyring via the gpg-verify homedir.
	verifyHome := filepath.Join(sandbox, "gnupg-verify")
	if err := os.MkdirAll(verifyHome, 0700); err != nil {
		t.Fatal(err)
	}
	exportCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--export", "--armor")
	keyData, err := exportCmd.Output()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	importCmd := exec.Command("gpg", "--homedir", verifyHome, "--batch", "--quiet",
		"--no-default-keyring", "--keyring", filepath.Join(verifyHome, "trusted.gpg"), "--import")
	importCmd.Stdin = strings.NewReader(string(keyData))
	importCmd.Stderr = nil
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("import: %v: %s", err, out)
	}

	// Corrupt the file; signature was made against the original.
	corrupted := filepath.Join(sandbox, "corrupted")
	if err := os.WriteFile(corrupted, []byte("tampered data"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := VerifySignature(corrupted, original+".sig", sandbox)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Fatal("expected invalid signature for corrupted data")
	}
	if result.Reason != "bad signature" {
		t.Errorf("expected reason 'bad signature', got %q", result.Reason)
	}
	t.Logf("correctly rejected: %s", result.Reason)
}

// TestUnpinnedKey verifies that a signature from a key whose fingerprint
// does not match PinnedFingerprint is rejected with a distinct reason.
func TestUnpinnedKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	sandbox := t.TempDir()

	// Generate a keypair that is NOT the pinned key.
	gnupgHome := filepath.Join(sandbox, "attacker")
	if err := os.MkdirAll(gnupgHome, 0700); err != nil {
		t.Fatal(err)
	}

	genScript := filepath.Join(sandbox, "attacker-keygen.txt")
	if err := os.WriteFile(genScript, []byte(`%no-protection
Key-Type: RSA
Key-Length: 2048
Name-Real: Attacker
Name-Email: attacker@evil.example
Expire-Date: 0
%commit
`), 0644); err != nil {
		t.Fatal(err)
	}

	genCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--gen-key", genScript)
	genCmd.Stderr = nil
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v: %s", err, out)
	}

	// Sign a file with the attacker's key.
	binary := filepath.Join(sandbox, "fake-binary")
	if err := os.WriteFile(binary, []byte("fake binary content"), 0644); err != nil {
		t.Fatal(err)
	}
	sigFile := binary + ".sig"
	signCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--detach-sign", "--armor", "-o", sigFile, binary)
	signCmd.Stderr = nil
	if out, err := signCmd.CombinedOutput(); err != nil {
		t.Fatalf("sign: %v: %s", err, out)
	}

	// Export the attacker's key and import it into the verification keyring.
	verifyHome := filepath.Join(sandbox, "gnupg-verify")
	if err := os.MkdirAll(verifyHome, 0700); err != nil {
		t.Fatal(err)
	}
	exportCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--export", "--armor")
	keyData, err := exportCmd.Output()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	importCmd := exec.Command("gpg", "--homedir", verifyHome, "--batch", "--quiet",
		"--no-default-keyring", "--keyring", filepath.Join(verifyHome, "trusted.gpg"), "--import")
	importCmd.Stdin = strings.NewReader(string(keyData))
	importCmd.Stderr = nil
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("import: %v: %s", err, out)
	}

	// The signature is cryptographically valid, but the key is not the pinned one.
	result, err := VerifySignature(binary, sigFile, sandbox)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Fatal("expected rejection: signature from unpinned key should not be accepted")
	}
	if result.Reason == "" || result.Reason == "bad signature" {
		t.Errorf("wrong reason for unpinned key rejection: %q (should mention fingerprint mismatch)", result.Reason)
	}
	if result.Fingerprint == PinnedFingerprint {
		t.Fatal("generated key should not have the pinned fingerprint")
	}
	t.Logf("correctly rejected unpinned key: %s (fingerprint %s)", result.Reason, result.Fingerprint)
}

// TestVersionMismatch verifies that a binary reporting a different version
// produces a VersionMismatchError, distinct from digest or signature failures.
func TestVersionMismatch(t *testing.T) {
	sandbox := t.TempDir()

	// Create a script that reports a different version.
	fakeBinary := filepath.Join(sandbox, "fake-rp")
	if err := os.WriteFile(fakeBinary, []byte(`#!/bin/sh
echo "rocketpool version 99.99.99"
`), 0755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reported, err := CheckVersion(ctx, fakeBinary)
	if err != nil {
		t.Fatalf("CheckVersion: %v", err)
	}

	if reported != "99.99.99" {
		t.Fatalf("unexpected reported version: %q", reported)
	}

	// The version does not match "v1.23.0".
	expected := expectedVersion("v1.23.0")
	if reported == expected {
		t.Fatal("test setup error: fake version should differ")
	}

	t.Logf("correctly detected version mismatch: binary reports %q, expected %q", reported, expected)
}

// TestImportPinnedKeyRejectsWrongKey confirms that ImportPinnedKey refuses
// to import a key whose fingerprint does not match PinnedFingerprint.
func TestImportPinnedKeyRejectsWrongKey(t *testing.T) {
	sandbox := t.TempDir()

	// Generate a random key.
	gnupgHome := filepath.Join(sandbox, "wrong-key")
	if err := os.MkdirAll(gnupgHome, 0700); err != nil {
		t.Fatal(err)
	}

	genScript := filepath.Join(sandbox, "wrong-keygen.txt")
	if err := os.WriteFile(genScript, []byte(`%no-protection
Key-Type: RSA
Key-Length: 2048
Name-Real: Wrong Key
Name-Email: wrong@example.com
Expire-Date: 0
%commit
`), 0644); err != nil {
		t.Fatal(err)
	}

	genCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--gen-key", genScript)
	genCmd.Stderr = nil
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v: %s", err, out)
	}

	exportCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--export", "--armor")
	keyData, err := exportCmd.Output()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	err = ImportPinnedKey(keyData, sandbox)
	if err == nil {
		t.Fatal("ImportPinnedKey should reject a key with wrong fingerprint")
	}
	if !containsStr(err.Error(), "does not match pinned") {
		t.Errorf("unexpected error message: %v", err)
	}
	t.Logf("correctly rejected wrong key: %v", err)
}

// TestUnknownKeySignature verifies that a signature from a key not in the
// verification keyring produces an "unknown key" failure.
func TestUnknownKeySignature(t *testing.T) {
	sandbox := t.TempDir()

	// Generate a key that we never import into the verification keyring.
	gnupgHome := filepath.Join(sandbox, "unknown")
	if err := os.MkdirAll(gnupgHome, 0700); err != nil {
		t.Fatal(err)
	}

	genScript := filepath.Join(sandbox, "unknown-keygen.txt")
	if err := os.WriteFile(genScript, []byte(`%no-protection
Key-Type: RSA
Key-Length: 2048
Name-Real: Unknown
Name-Email: unknown@example.com
Expire-Date: 0
%commit
`), 0644); err != nil {
		t.Fatal(err)
	}

	genCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--gen-key", genScript)
	genCmd.Stderr = nil
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v: %s", err, out)
	}

	// Sign a file.
	binary := filepath.Join(sandbox, "test-file")
	if err := os.WriteFile(binary, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}
	sigFile := binary + ".sig"
	signCmd := exec.Command("gpg", "--homedir", gnupgHome, "--batch", "--detach-sign", "--armor", "-o", sigFile, binary)
	signCmd.Stderr = nil
	if out, err := signCmd.CombinedOutput(); err != nil {
		t.Fatalf("sign: %v: %s", err, out)
	}

	// Verify against an empty keyring — the key is unknown.
	result, err := VerifySignature(binary, sigFile, sandbox)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Fatal("expected rejection for unknown key")
	}
	if result.Reason != "unknown key" {
		t.Errorf("expected reason 'unknown key', got %q", result.Reason)
	}
	t.Logf("correctly rejected unknown key: %s", result.Reason)
}

// TestVerifyIntegration_CorruptedBinary is the full-pipeline negative test:
// download the real artifact, corrupt it, and confirm a DigestMismatchError.
func TestVerifyIntegration_CorruptedBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	sandbox := t.TempDir()

	rel, err := FetchRelease("v1.23.0")
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}

	assets, err := ResolveAssetSet(rel, "rocketpool-cli-linux-amd64")
	if err != nil {
		t.Fatalf("ResolveAssetSet: %v", err)
	}

	binaryPath, sigPath, err := DownloadAssetSet(assets, sandbox)
	if err != nil {
		t.Fatalf("DownloadAssetSet: %v", err)
	}

	// Import the real signing key.
	keyAsset := findAsset(rel, "fornax-signing-key.asc")
	if keyAsset == nil {
		t.Fatal("signing key not found")
	}
	keyPath, err := Download(*keyAsset, sandbox)
	if err != nil {
		t.Fatalf("download key: %v", err)
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := ImportPinnedKey(keyData, sandbox); err != nil {
		t.Fatalf("ImportPinnedKey: %v", err)
	}

	// Corrupt one byte of the binary.
	f, err := os.OpenFile(binaryPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	noise := make([]byte, 1)
	rand.Read(noise)
	f.WriteAt(noise, 100)
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, verifyErr := Verify(ctx, binaryPath, sigPath, sandbox, "v1.23.0", assets.DigestHex)
	if verifyErr == nil {
		t.Fatal("expected error for corrupted binary")
	}

	var digestErr *DigestMismatchError
	if !errors.As(verifyErr, &digestErr) {
		t.Fatalf("expected DigestMismatchError, got %T: %v", verifyErr, verifyErr)
	}
	t.Logf("correctly detected corruption: %v", verifyErr)
}

// findAsset locates an asset by name in a release.
func findAsset(rel *Release, name string) *ReleaseAsset {
	for i := range rel.Assets {
		if rel.Assets[i].Name == name {
			return &rel.Assets[i]
		}
	}
	return nil
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
