package jwt

import (
	"fmt"
	"os"
	"path/filepath"
)

// PlantEmpty creates a zero-byte secret at path, replacing any existing file.
//
// This reproduces the state a partially completed write leaves behind, which is
// the condition the start scripts disagree about repairing.
func PlantEmpty(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create secret directory: %w", err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return fmt.Errorf("plant empty secret: %w", err)
	}
	return nil
}
