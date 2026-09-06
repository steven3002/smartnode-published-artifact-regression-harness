package cli

import (
	"regexp"
	"testing"
)

func TestJWTValidation(t *testing.T) {
	regex := `^(0x)?[0-9a-fA-F]{64}$`
	
	validBare := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	validPrefixed := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	
	if matched, _ := regexp.MatchString(regex, validBare); !matched {
		t.Errorf("Expected bare 64-hex to validate")
	}
	
	if matched, _ := regexp.MatchString(regex, validPrefixed); !matched {
		t.Errorf("Expected 0x-prefixed to validate")
	}
	
	invalidShort := "0x1234567890abcdef"
	if matched, _ := regexp.MatchString(regex, invalidShort); matched {
		t.Errorf("Expected short JWT to fail")
	}
	
	invalidChars := "0x1234567890abcdeg1234567890abcdef1234567890abcdef1234567890abcdef" // contains 'g'
	if matched, _ := regexp.MatchString(regex, invalidChars); matched {
		t.Errorf("Expected invalid hex characters to fail")
	}
}
