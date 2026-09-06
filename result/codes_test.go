package result

import (
	"testing"
)

func TestExitCodeMatrix(t *testing.T) {
	tests := []struct {
		name      string
		class     FailureClass
		isFixture bool
		want      int
	}{
		// run cases
		{"run PASS", ClassSuccess, false, 0},
		{"run PRODUCT fail", ClassProduct, false, 1},
		{"run INFRASTRUCTURE fail", ClassInfrastructure, false, 2},
		{"run HARNESS fail", ClassHarness, false, 3},
		{"run TIMEOUT fail", ClassTimeout, false, 4},

		// fixture cases (inverted semantics)
		// fixture → zero only when the expected detection or repair behaviour occurs.
		{"fixture expected failure observed (PRODUCT)", ClassProduct, true, 0},

		// fixture → non-zero when the harness fails to observe the expected result.
		{"fixture unexpected PASS", ClassSuccess, true, 1},

		// Infrastructure/harness/timeout pass through as invalid runs.
		{"fixture INFRASTRUCTURE fail", ClassInfrastructure, true, 2},
		{"fixture HARNESS fail", ClassHarness, true, 3},
		{"fixture TIMEOUT fail", ClassTimeout, true, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.class, tt.isFixture)
			if got != tt.want {
				t.Errorf("ExitCode(%v, %v) = %v, want %v", tt.class, tt.isFixture, got, tt.want)
			}
		})
	}
}

func TestFixtureExitCode(t *testing.T) {
	tests := []struct {
		name  string
		class FailureClass
		kind  FixtureKind
		want  int
	}{
		// Defect fixtures (e.g. geth empty-jwt): expect ClassProduct
		{"defect: product failure observed", ClassProduct, FixtureDefect, 0},
		{"defect: unexpected success", ClassSuccess, FixtureDefect, 1},
		{"defect: infrastructure", ClassInfrastructure, FixtureDefect, 2},
		{"defect: timeout", ClassTimeout, FixtureDefect, 4},

		// Repair fixtures (e.g. besu empty-jwt): expect ClassSuccess
		{"repair: success confirmed", ClassSuccess, FixtureRepair, 0},
		{"repair: unexpected product failure", ClassProduct, FixtureRepair, 1},
		{"repair: infrastructure", ClassInfrastructure, FixtureRepair, 2},
		{"repair: timeout", ClassTimeout, FixtureRepair, 4},

		// FixtureNone delegates to regular ExitCode
		{"none: success", ClassSuccess, FixtureNone, 0},
		{"none: product", ClassProduct, FixtureNone, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FixtureExitCode(tt.class, tt.kind)
			if got != tt.want {
				t.Errorf("FixtureExitCode(%v, %v) = %v, want %v", tt.class, tt.kind, got, tt.want)
			}
		})
	}
}
