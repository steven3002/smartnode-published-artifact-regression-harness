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
		// wait, if the fixture didn't fail at all (ClassSuccess), that's a failure of the fixture to detect.
		// So it should exit non-zero!
		{"fixture unexpected PASS", ClassSuccess, true, 1}, 
		
		// What if it failed for infrastructure reasons? The fixture didn't observe the PRODUCT failure.
		// So it should probably just pass through the INFRASTRUCTURE/HARNESS/TIMEOUT exit codes
		// to indicate the run was invalid.
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
