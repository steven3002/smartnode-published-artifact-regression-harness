package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// DigestResult holds the outcome of a SHA-256 digest check.
type DigestResult struct {
	Actual   string // hex-encoded SHA-256 of the file on disk
	Expected string // hex-encoded SHA-256 from the API, empty if unavailable
	Match    bool   // true only when Expected is non-empty and equals Actual
}

// ComputeSHA256 returns the lowercase hex-encoded SHA-256 of a file.
func ComputeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open for digest: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read for digest: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyDigest computes the SHA-256 of binaryPath and compares it against
// the expected hex digest. Returns a DigestResult indicating whether they
// match. A mismatch is a PRODUCT failure — the binary is not what was published.
func VerifyDigest(binaryPath, expectedHex string) (*DigestResult, error) {
	actual, err := ComputeSHA256(binaryPath)
	if err != nil {
		return nil, err
	}

	result := &DigestResult{
		Actual:   actual,
		Expected: expectedHex,
	}

	if expectedHex != "" {
		result.Match = actual == expectedHex
	}

	return result, nil
}
