package redact

import (
	"testing"
)

func TestRedact(t *testing.T) {
	r := New()
	r.AddLiteral("my-super-secret-password")

	cases := []struct {
		in, expected string
	}{
		{
			"Here is a jwt: 1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			"Here is a jwt: [REDACTED]",
		},
		{
			"Here is a jwt: 0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			"Here is a jwt: [REDACTED]",
		},
		{
			"My pass is my-super-secret-password!",
			"My pass is [REDACTED]!",
		},
		{
			"Mnemonic: abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
			"Mnemonic: [REDACTED]",
		},
	}

	for _, c := range cases {
		out := r.Redact([]byte(c.in))
		if string(out) != c.expected {
			t.Errorf("expected %q, got %q", c.expected, out)
		}
	}
}
