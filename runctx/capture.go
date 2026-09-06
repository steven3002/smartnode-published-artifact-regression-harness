package runctx

import (
	"bytes"
)

// overlap is how many trailing bytes are held back between writes so a secret
// spanning a chunk boundary is still matched.
//
// It exceeds the longest pattern the redactor recognises: a 24-word BIP-39
// mnemonic at the maximum word length. Without this, a JWT split across two
// reads matches neither half and reaches the buffer in clear text.
const overlap = 256

// BoundedWriter caps how much output a step may retain, redacting as it goes
// and recording what was dropped rather than truncating silently.
type BoundedWriter struct {
	Buf     *bytes.Buffer
	Limit   int64
	Written int64
	Dropped int64
	Redact  func([]byte) []byte

	// pending holds bytes withheld from the previous write so that redaction
	// can span a chunk boundary.
	pending []byte
}

func (b *BoundedWriter) Write(p []byte) (int, error) {
	n := len(p)

	if b.Redact == nil {
		b.append(p)
		return n, nil
	}

	buf := append(b.pending, p...)

	// Hold back the tail; it may be the start of a secret completed by the
	// next write. Everything before it is safe to redact and emit now.
	keep := overlap
	if keep > len(buf) {
		keep = len(buf)
	}
	flush, hold := buf[:len(buf)-keep], buf[len(buf)-keep:]

	b.pending = append(b.pending[:0], hold...)
	b.append(b.Redact(flush))

	return n, nil
}

// Flush emits any withheld tail. It must be called once the step has exited,
// or the final bytes of its output are lost.
func (b *BoundedWriter) Flush() {
	if len(b.pending) == 0 {
		return
	}
	out := b.pending
	if b.Redact != nil {
		out = b.Redact(out)
	}
	b.append(out)
	b.pending = nil
}

// append writes within the cap, counting anything beyond it as dropped.
func (b *BoundedWriter) append(p []byte) {
	if b.Written >= b.Limit {
		b.Dropped += int64(len(p))
		return
	}
	if space := b.Limit - b.Written; int64(len(p)) > space {
		b.Buf.Write(p[:space])
		b.Written += space
		b.Dropped += int64(len(p)) - space
		return
	}
	written, _ := b.Buf.Write(p)
	b.Written += int64(written)
}
