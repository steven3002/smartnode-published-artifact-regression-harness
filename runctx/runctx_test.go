package runctx

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRunStep_TimeoutAndOutputCap(t *testing.T) {
	// Create a test script that spawns a child, ignores SIGTERM, and floods output.
	script := `#!/bin/sh
trap '' TERM
(
  trap '' TERM
  while true; do
    echo "Please answer 'y' or 'n'"
  done
) &
wait
`
	scriptFile, err := os.CreateTemp("", "flood-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(scriptFile.Name())

	scriptFile.Write([]byte(script))
	scriptFile.Close()
	os.Chmod(scriptFile.Name(), 0755)

	rc, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Cleanup()

	cmd := exec.Command(scriptFile.Name())

	// Byte cap: 1MB. We expect it to drop bytes because the flood is fast.
	byteCap := int64(1024 * 1024)

	start := time.Now()
	res, err := rc.RunStep(context.Background(), cmd, 500*time.Millisecond, byteCap, nil)
	duration := time.Since(start)

	if duration < 500*time.Millisecond || duration > 3*time.Second { // 500ms + 2s grace
		t.Errorf("expected duration around 2.5s, got %v", duration)
	}

	if res.FailureClass != "TIMEOUT" {
		t.Errorf("expected failure class TIMEOUT, got %v", res.FailureClass)
	}

	if int64(len(res.Output)) > byteCap {
		t.Errorf("output exceeded cap: %d", len(res.Output))
	}

	if res.TruncatedBytes == 0 {
		t.Error("expected truncated bytes > 0")
	}

	if !strings.Contains(string(res.Output), "Please answer") {
		t.Error("expected output to contain flood message")
	}
}
