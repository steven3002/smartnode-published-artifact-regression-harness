package ui

import (
	"os"
	"testing"
)

func TestDetectCapability_NoColor(t *testing.T) {
	f, _ := os.CreateTemp("", "")
	defer os.Remove(f.Name())

	env := map[string]string{
		"NO_COLOR": "0",
	}
	getEnv := func(k string) string { return env[k] }

	cap := DetectCapability(f, ColorAuto, getEnv)
	if cap.Color {
		t.Error("expected color to be disabled due to NO_COLOR")
	}
}

func TestDetectCapability_Unicode(t *testing.T) {
	f, _ := os.CreateTemp("", "")
	defer os.Remove(f.Name())

	// No env -> UTF-8
	env := map[string]string{}
	getEnv := func(k string) string { return env[k] }
	cap := DetectCapability(f, ColorAuto, getEnv)
	if !cap.Unicode {
		t.Error("expected unicode to be true with no locale set")
	}

	// TERM=dumb -> ASCII
	env["TERM"] = "dumb"
	cap = DetectCapability(f, ColorAuto, getEnv)
	if cap.Unicode {
		t.Error("expected unicode to be false with TERM=dumb")
	}
}
