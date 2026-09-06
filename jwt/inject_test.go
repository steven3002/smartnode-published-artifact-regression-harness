package jwt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestPlantEmptyIsDetected is the fixture's own contract: the planted state must
// be the state Inspect reports as JWT-001, or the fixture proves nothing.
func TestPlantEmptyIsDetected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets", "jwtsecret")

	if err := PlantEmpty(path); err != nil {
		t.Fatalf("PlantEmpty: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("planted secret is not present: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("planted secret is %d bytes, want 0", info.Size())
	}

	state, err := Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !state.Present || !state.Empty {
		t.Fatalf("planted secret reported as %+v", state)
	}
	if state.Diagnosis() != CodeMalformed {
		t.Errorf("Diagnosis() = %q, want %s", state.Diagnosis(), CodeMalformed)
	}
}

// TestPlantEmptyReplacesAValidSecret guards against a fixture that silently
// leaves a healthy secret in place, which would make the run pass for the wrong
// reason.
func TestPlantEmptyReplacesAValidSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jwtsecret")
	valid := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := PlantEmpty(path); err != nil {
		t.Fatalf("PlantEmpty: %v", err)
	}

	state, err := Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !state.Empty {
		t.Error("an existing valid secret survived the fixture")
	}
}

// TestInspectAbsentSecret confirms a missing file is distinguishable from a
// broken one.
func TestInspectAbsentSecret(t *testing.T) {
	state, err := Inspect(context.Background(), filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("Inspect on an absent secret returned an error: %v", err)
	}
	if state.Present {
		t.Error("absent secret reported as present")
	}
	if state.Diagnosis() != "" {
		t.Errorf("absent secret raised %q", state.Diagnosis())
	}
}
