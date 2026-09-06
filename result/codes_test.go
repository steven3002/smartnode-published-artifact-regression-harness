package result

import "testing"

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		res  Result
		want int
	}{
		{"pass", Pass(), ExitOK},
		{"product", Fail(ClassProduct, ""), ExitProduct},
		{"infrastructure", Fail(ClassInfrastructure, ""), ExitInfrastructure},
		{"harness", Fail(ClassHarness, ""), ExitHarness},
		{"timeout", Fail(ClassTimeout, ""), ExitTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.res); got != tt.want {
				t.Errorf("ExitCode(%+v) = %d, want %d", tt.res, got, tt.want)
			}
		})
	}
}

// TestFixtureExitCode covers the inverted semantics: a fixture exits zero when
// it observes what it predicted, so a defect fixture that suddenly passes must
// be reported as a failure rather than a green run.
func TestFixtureExitCode(t *testing.T) {
	tests := []struct {
		name string
		res  Result
		kind FixtureKind
		want int
	}{
		{"defect: predicted product failure observed", Fail(ClassProduct, ""), FixtureDefect, ExitOK},
		{"defect: stopped reproducing", Pass(), FixtureDefect, ExitUnexpected},
		{"defect: infrastructure says nothing", Fail(ClassInfrastructure, ""), FixtureDefect, ExitInfrastructure},
		{"defect: harness says nothing", Fail(ClassHarness, ""), FixtureDefect, ExitHarness},
		{"defect: timeout says nothing", Fail(ClassTimeout, ""), FixtureDefect, ExitTimeout},

		{"repair: predicted recovery observed", Pass(), FixtureRepair, ExitOK},
		{"repair: did not recover", Fail(ClassProduct, ""), FixtureRepair, ExitUnexpected},
		{"repair: infrastructure says nothing", Fail(ClassInfrastructure, ""), FixtureRepair, ExitInfrastructure},
		{"repair: timeout says nothing", Fail(ClassTimeout, ""), FixtureRepair, ExitTimeout},

		{"none: delegates to run semantics on pass", Pass(), FixtureNone, ExitOK},
		{"none: delegates to run semantics on failure", Fail(ClassProduct, ""), FixtureNone, ExitProduct},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FixtureExitCode(tt.res, tt.kind); got != tt.want {
				t.Errorf("FixtureExitCode(%+v, %v) = %d, want %d", tt.res, tt.kind, got, tt.want)
			}
		})
	}
}

// TestFailureClassIsNotAnOutcome guards the model rule that pass and fail are
// recorded in exactly one place. A class that could also mean success would let
// the two fields disagree.
func TestFailureClassIsNotAnOutcome(t *testing.T) {
	if got := Pass().FailureClass; got != "" {
		t.Errorf("a passing result carries failure class %q; want empty", got)
	}

	var fc FailureClass
	if err := fc.UnmarshalJSON([]byte(`"SUCCESS"`)); err == nil {
		t.Error("SUCCESS was accepted as a failure class; it must be rejected")
	}
}
