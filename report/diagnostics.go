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

// WriteDiagnostics copies files from srcDir to destDir, applying the given redactor to file contents.
// Only files are copied, preserving relative paths.
func WriteDiagnostics(srcDir, destDir string, redactor Redactor) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create diagnostics dir: %w", err)
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Cannot access path
		}
		if info.IsDir() {
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
			// Best effort, skip if we can't read
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
