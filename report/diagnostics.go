package report

import (
	"fmt"
	"os"
	"path/filepath"
)

// Redactor defines the interface for redacting sensitive data.
type Redactor interface {
	Redact([]byte) []byte
}

// maxDiagnosticBytes caps a single captured file. Diagnostics exist to explain a
// failure, and an unbounded copy turns a published artifact into a liability.
const maxDiagnosticBytes = 4 << 20

// skipNames are files in the run directory that are not diagnostics.
//
// The verified binary, its signature and the throwaway GPG keyrings all live
// alongside the logs. Copying them produces a multi-megabyte bundle of binary
// content that no reader can use, and passing that content through a text
// redactor does not make it safe to publish.
var skipNames = map[string]bool{
	"rocketpool":                     true,
	"rocketpool-cli-linux-amd64":     true,
	"rocketpool-cli-linux-amd64.sig": true,
	"fornax-signing-key.asc":         true,
}

// skipDirs are directories in the run directory that hold no diagnostics.
var skipDirs = map[string]bool{
	"gnupg-verify":       true,
	"gnupg-import-check": true,
}

// WriteDiagnostics copies the run's textual diagnostics into destDir, redacting
// as it goes and preserving relative paths.
func WriteDiagnostics(srcDir, destDir string, redactor Redactor) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create diagnostics dir: %w", err)
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Cannot access path
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if skipNames[info.Name()] {
			return nil
		}
		if info.Size() > maxDiagnosticBytes {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, rel)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		srcData, err := os.ReadFile(path)
		if err != nil {
			// A file the host user cannot read is not worth failing the report
			// over; container-owned state is expected here.
			return nil
		}
		if isBinary(srcData) {
			return nil
		}

		var redactedData []byte
		if redactor != nil {
			redactedData = redactor.Redact(srcData)
		} else {
			redactedData = srcData
		}

		if err := os.WriteFile(destPath, redactedData, 0644); err != nil {
			return fmt.Errorf("write redacted file: %w", err)
		}
		return nil
	})
}

// isBinary reports whether content looks like a binary file. A NUL byte in the
// first block is the same heuristic diff and grep use.
func isBinary(data []byte) bool {
	head := data
	if len(head) > 8000 {
		head = head[:8000]
	}
	for _, b := range head {
		if b == 0 {
			return true
		}
	}
	return false
}
