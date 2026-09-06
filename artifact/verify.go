package artifact

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// VerificationReport is the complete result of verifying a release artifact.
type VerificationReport struct {
	Tag             string
	BinarySHA256    string
	ExpectedSHA256  string
	DigestMatch     bool
	SignatureValid  bool
	SignerUID       string
	KeyFingerprint  string
	SignatureReason string // non-empty when SignatureValid is false
	ReportedVersion string
	VersionMatch    bool
	Timestamp       time.Time
}

// VersionMismatchError signals a distinct failure: the binary's self-reported
// version differs from the requested release tag.
type VersionMismatchError struct {
	Requested string
	Reported  string
}

func (e *VersionMismatchError) Error() string {
	return fmt.Sprintf("version mismatch: requested %s, binary reports %q", e.Requested, e.Reported)
}

// DigestMismatchError signals that the binary's SHA-256 does not match the
// digest published with the release.
type DigestMismatchError struct {
	Expected string
	Actual   string
}

func (e *DigestMismatchError) Error() string {
	return fmt.Sprintf("digest mismatch: expected %s, got %s", e.Expected, e.Actual)
}

// SignatureError signals that GPG signature verification failed.
type SignatureError struct {
	Reason string
}

func (e *SignatureError) Error() string {
	return fmt.Sprintf("signature verification failed: %s", e.Reason)
}

// CheckVersion executes the binary with --version and returns the
// reported version string. The binary must have been verified (digest and
// signature) before this is called — it runs an unverified binary.
//
// The execute bit is set here because the binary arrives without it.
// Execution is bounded by the provided context deadline.
func CheckVersion(ctx context.Context, binaryPath string) (string, error) {
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return "", fmt.Errorf("chmod: %w", err)
	}

	cmd := exec.CommandContext(ctx, binaryPath, "--version")
	// Isolate from the host environment.
	cmd.Env = []string{"PATH=/usr/bin:/bin"}

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("execute --version: %w", err)
	}

	return parseVersionOutput(string(out)), nil
}

// parseVersionOutput extracts the version from output like
// "rocketpool version 1.23.0\n".
func parseVersionOutput(output string) string {
	// The binary outputs "rocketpool version X.Y.Z".
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "rocketpool version ") {
			return strings.TrimPrefix(line, "rocketpool version ")
		}
	}
	return strings.TrimSpace(output)
}

// expectedVersion strips the leading "v" from a tag to match the binary's
// output format (e.g., "v1.23.0" → "1.23.0").
func expectedVersion(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// Verify performs the complete verification pipeline for a release artifact:
//  1. SHA-256 digest check (PRODUCT failure on mismatch)
//  2. GPG signature check (PRODUCT failure on bad/unknown key)
//  3. Version self-report (PRODUCT failure on mismatch)
//
// The order is deliberate: the binary is executed only after digest and
// signature verification pass, because until then it is unverified.
func Verify(ctx context.Context, binaryPath, sigPath, sandboxDir, tag, expectedDigest string) (*VerificationReport, error) {
	report := &VerificationReport{
		Tag:       tag,
		Timestamp: time.Now().UTC(),
	}

	// Step 1: Digest verification.
	digestResult, err := VerifyDigest(binaryPath, expectedDigest)
	if err != nil {
		return report, fmt.Errorf("digest computation: %w", err)
	}
	report.BinarySHA256 = digestResult.Actual
	report.ExpectedSHA256 = digestResult.Expected
	report.DigestMatch = digestResult.Match

	if expectedDigest != "" && !digestResult.Match {
		return report, &DigestMismatchError{
			Expected: expectedDigest,
			Actual:   digestResult.Actual,
		}
	}

	// Step 2: Signature verification.
	sigResult, err := VerifySignature(binaryPath, sigPath, sandboxDir)
	if err != nil {
		return report, fmt.Errorf("signature verification: %w", err)
	}
	report.SignatureValid = sigResult.Valid
	report.SignerUID = sigResult.SignerUID
	report.KeyFingerprint = sigResult.Fingerprint
	report.SignatureReason = sigResult.Reason

	if !sigResult.Valid {
		return report, &SignatureError{Reason: sigResult.Reason}
	}

	// Step 3: Version assertion — only after digest and signature pass.
	versionCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reported, err := CheckVersion(versionCtx, binaryPath)
	if err != nil {
		return report, fmt.Errorf("version check: %w", err)
	}
	report.ReportedVersion = reported
	report.VersionMatch = reported == expectedVersion(tag)

	if !report.VersionMatch {
		return report, &VersionMismatchError{
			Requested: tag,
			Reported:  reported,
		}
	}

	return report, nil
}
