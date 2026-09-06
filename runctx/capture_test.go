package runctx

import (
	"bytes"
	"strings"
	"testing"
)

// redactHex stands in for the real redactor: it removes 64-character hex runs,
// which is the shape of an engine-API secret.
func redactHex(p []byte) []byte {
	const hexLen = 64
	out := p
	for {
		idx := indexHexRun(out, hexLen)
		if idx < 0 {
			return out
		}
		out = append(append(append([]byte{}, out[:idx]...), []byte("[REDACTED]")...), out[idx+hexLen:]...)
	}
}

func indexHexRun(p []byte, n int) int {
	run := 0
	for i := 0; i < len(p); i++ {
		if isHex(p[i]) {
			run++
			if run == n {
				return i - n + 1
			}
			continue
		}
		run = 0
	}
	return -1
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// TestRedactionSpansChunkBoundaries is the case a per-write redactor misses. A
// process writes in arbitrary chunks, so a secret split across two writes
// matches neither half and would reach the buffer in clear text.
func TestRedactionSpansChunkBoundaries(t *testing.T) {
	secret := strings.Repeat("ab", 32)

	var buf bytes.Buffer
	w := &BoundedWriter{Buf: &buf, Limit: 1 << 20, Redact: redactHex}

	// Split the secret across two writes, as a pipe would.
	w.Write([]byte("token=" + secret[:20]))
	w.Write([]byte(secret[20:] + " done\n"))
	w.Flush()

	got := buf.String()
	if strings.Contains(got, secret) {
		t.Fatalf("secret survived a split write: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("output was not redacted: %q", got)
	}
	if !strings.Contains(got, "done") {
		t.Errorf("surrounding output was lost: %q", got)
	}
}

// TestFlushEmitsWithheldTail checks that holding bytes back for boundary
// matching does not silently drop the end of a step's output.
func TestFlushEmitsWithheldTail(t *testing.T) {
	var buf bytes.Buffer
	w := &BoundedWriter{Buf: &buf, Limit: 1 << 20, Redact: func(p []byte) []byte { return p }}

	w.Write([]byte("short output"))
	if buf.Len() != 0 {
		t.Fatalf("tail was emitted before Flush: %q", buf.String())
	}

	w.Flush()
	if got := buf.String(); got != "short output" {
		t.Errorf("after Flush got %q, want %q", got, "short output")
	}
}

// TestCapRecordsWhatItDropped keeps truncation visible. A report that hides
// truncation is worse than one that admits it.
func TestCapRecordsWhatItDropped(t *testing.T) {
	var buf bytes.Buffer
	w := &BoundedWriter{Buf: &buf, Limit: 10}

	w.Write(bytes.Repeat([]byte("x"), 100))
	w.Flush()

	if buf.Len() != 10 {
		t.Errorf("buffered %d bytes, want 10", buf.Len())
	}
	if w.Dropped != 90 {
		t.Errorf("Dropped = %d, want 90", w.Dropped)
	}
}
