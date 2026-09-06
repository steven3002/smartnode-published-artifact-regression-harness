package redact

import (
	"regexp"
)

// Redactor holds the patterns and literal strings to redact from streams.
type Redactor struct {
	patterns []*regexp.Regexp
	literals [][]byte
}

func New() *Redactor {
	r := &Redactor{}
	
	// JWT, private keys (64 hex chars, optional 0x prefix)
	r.patterns = append(r.patterns, regexp.MustCompile(`(?:0x)?[0-9a-fA-F]{64}`))
	
	// BIP-39 mnemonic (24 lowercase words separated by single spaces)
	// We'll use a word boundary to avoid partial matches
	r.patterns = append(r.patterns, regexp.MustCompile(`\b([a-z]{3,8}(?: [a-z]{3,8}){23})\b`))
	
	// Passwords if they are in standard logs, though normally they aren't logged.
	// We can also add known passwords as literals.
	
	return r
}

func (r *Redactor) AddLiteral(secret string) {
	if secret != "" {
		r.literals = append(r.literals, []byte(secret))
	}
}

func (r *Redactor) Redact(in []byte) []byte {
	out := in
	for _, p := range r.patterns {
		out = p.ReplaceAll(out, []byte("[REDACTED]"))
	}
	for _, l := range r.literals {
		out = replaceAll(out, l, []byte("[REDACTED]"))
	}
	return out
}

func replaceAll(s, old, new []byte) []byte {
	// Simple non-overlapping replacement
	return regexp.MustCompile(regexp.QuoteMeta(string(old))).ReplaceAll(s, new)
}
