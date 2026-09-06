package ui

import (
	"testing"
)

func TestFormatStatus(t *testing.T) {
	// Unicode + Color
	cap := Capability{Unicode: true, Color: true}
	if out := FormatStatus(cap, StatusPass, "done"); out != "\033[32m✓\033[0m done" {
		t.Errorf("unexpected out: %q", out)
	}

	// ASCII + No Color
	cap = Capability{Unicode: false, Color: false}
	if out := FormatStatus(cap, StatusPass, "done"); out != "ok done" {
		t.Errorf("unexpected out: %q", out)
	}
	if out := FormatStatus(cap, StatusFail, "err"); out != "x err" {
		t.Errorf("unexpected out: %q", out)
	}
}
