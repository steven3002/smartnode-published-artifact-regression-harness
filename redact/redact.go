package redact

import (
	"bytes"
	"regexp"
)

// Redactor holds the patterns and literal strings to redact from streams.
type Redactor struct {
	patterns []*regexp.Regexp
	literals [][]byte
}

func New() *Redactor {
	r := &Redactor{}

	// Engine-API secrets and private keys: 64 hex characters, optionally 0x
	// prefixed. Both encodings appear on disk, so both must be matched.
	r.patterns = append(r.patterns, regexp.MustCompile(`(?:0x)?[0-9a-fA-F]{64}`))

	// A 24-word BIP-39 mnemonic. Word boundaries keep this from matching an
	// arbitrary run of lowercase prose.
	r.patterns = append(r.patterns, regexp.MustCompile(`\b([a-z]{3,8}(?: [a-z]{3,8}){23})\b`))

	return r
}

// AddLiteral registers a known secret to remove verbatim, for values the
// patterns cannot describe.
func (r *Redactor) AddLiteral(secret string) {
	if secret != "" {
		r.literals = append(r.literals, []byte(secret))
	}
}

// placeholder replaces every redacted value, so a reader can see that something
// was removed rather than that nothing was there.
var placeholder = []byte("[REDACTED]")

// Redact removes every known secret shape from a buffer.
func (r *Redactor) Redact(in []byte) []byte {
	out := in
	for _, p := range r.patterns {
		out = p.ReplaceAll(out, placeholder)
	}
	for _, l := range r.literals {
		out = bytes.ReplaceAll(out, l, placeholder)
	}
	return out
}
