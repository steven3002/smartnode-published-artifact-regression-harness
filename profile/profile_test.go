package profile

import "testing"

// TestRepairExpectationsMatchTheStartScript pins the finding this harness
// exists to demonstrate. If either expectation is edited without evidence from
// the published start script, the fixtures stop meaning anything.
func TestRepairExpectationsMatchTheStartScript(t *testing.T) {
	tests := []struct {
		profile string
		ec      string
		cc      string
		repairs JWTRepair
	}{
		{"geth-lighthouse", "geth", "lighthouse", DoesNotRepair},
		{"besu-teku", "besu", "teku", Repairs},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			p, err := Lookup(tt.profile)
			if err != nil {
				t.Fatalf("Lookup(%q): %v", tt.profile, err)
			}
			if p.ExecutionClient != tt.ec || p.ConsensusClient != tt.cc {
				t.Errorf("clients: got %s/%s, want %s/%s",
					p.ExecutionClient, p.ConsensusClient, tt.ec, tt.cc)
			}
			if p.JWTRepairs != tt.repairs {
				t.Errorf("JWTRepairs = %v, want %v", p.JWTRepairs, tt.repairs)
			}
			if p.RepairEvidence == "" {
				t.Error("repair expectation carries no evidence")
			}
		})
	}
}

// TestLookupRejectsUnknownProfiles checks the error names what is supported.
// The previous parser split any hyphenated string, so a typo produced a run
// against clients that do not exist.
func TestLookupRejectsUnknownProfiles(t *testing.T) {
	for _, name := range []string{"", "besu", "geth", "geth-teku", "nethermind-teku"} {
		if _, err := Lookup(name); err == nil {
			t.Errorf("Lookup(%q) was accepted; want an error", name)
		}
	}
}
