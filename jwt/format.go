// Package jwt holds the engine-API secret's format rules and the fault
// injection used to exercise them.
package jwt

import "regexp"

// Pattern accepts both encodings the Smartnode stack produces on disk.
//
// Geth writes the secret itself from --authrpc.jwtsecret as 0x-prefixed, 66
// bytes. The Smartnode start scripts write bare 64-character hex for the
// clients they generate a secret for. A validator accepting only one of these
// rejects a healthy stack.
var Pattern = regexp.MustCompile(`^(0x)?[0-9a-fA-F]{64}$`)

// Valid reports whether a secret is well formed. Leading and trailing space is
// tolerated because the file is newline-terminated by some writers.
func Valid(secret string) bool {
	return Pattern.MatchString(trimSpace(secret))
}

// trimSpace removes surrounding ASCII whitespace without pulling in strings for
// one call.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
