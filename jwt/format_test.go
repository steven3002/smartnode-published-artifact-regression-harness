package jwt

import "testing"

// TestValid covers both encodings the stack actually produces. A validator that
// accepts only bare hex rejects Geth's own output, which would fail the
// baseline profile rather than find a defect in it.
func TestValid(t *testing.T) {
	const hex64 = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	tests := []struct {
		name   string
		secret string
		want   bool
	}{
		{"bare 64 hex, as the start scripts write", hex64, true},
		{"0x-prefixed, as geth writes", "0x" + hex64, true},
		{"uppercase hex", "0xABCDEF7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", true},
		{"trailing newline", hex64 + "\n", true},
		{"surrounding whitespace", "  " + hex64 + "\t", true},

		{"empty", "", false},
		{"whitespace only", "   \n", false},
		{"too short", "0x1234567890abcdef", false},
		{"one character short", hex64[:63], false},
		{"one character long", hex64 + "a", false},
		{"non-hex character", "0x1234567890abcdeg1234567890abcdef1234567890abcdef1234567890abcdef", false},
		{"wrong prefix", "0X" + hex64, false},
		{"embedded in other text", "secret=" + hex64, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.secret); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.secret, got, tt.want)
			}
		})
	}
}

// TestStateDiagnosis checks that only an unusable secret raises JWT-001, and
// that an absent secret does not: a stack that has not yet written one is not
// the same as a stack that wrote a broken one.
func TestStateDiagnosis(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  string
	}{
		{"well formed", State{Present: true, Valid: true}, ""},
		{"zero byte", State{Present: true, Empty: true}, CodeMalformed},
		{"malformed", State{Present: true, Valid: false}, CodeMalformed},
		{"absent", State{Present: false}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.Diagnosis(); got != tt.want {
				t.Errorf("Diagnosis() = %q, want %q", got, tt.want)
			}
		})
	}
}
