package result

// FailureClass says who is at fault when a run fails.
//
// Misclassification is what kills a release gate: a harness that blames the
// product for a checkpoint provider outage teaches its readers to ignore it.
type FailureClass string

const (
	// ClassProduct means the software under test did the wrong thing.
	ClassProduct FailureClass = "PRODUCT"
	// ClassInfrastructure means the world did — a provider, network or registry.
	ClassInfrastructure FailureClass = "INFRASTRUCTURE"
	// ClassHarness means the harness did.
	ClassHarness FailureClass = "HARNESS"
	// ClassTimeout means a deadline expired, which is distinct from a defect.
	ClassTimeout FailureClass = "TIMEOUT"
)

// Exit codes. Distinct per failure class so a script can branch on the reason.
const (
	ExitOK             = 0
	ExitProduct        = 1
	ExitInfrastructure = 2
	ExitHarness        = 3
	ExitTimeout        = 4
	// ExitUnexpected reports that a fixture did not observe what it predicted.
	ExitUnexpected = 5
)

// FixtureKind is what a fixture predicts will happen.
type FixtureKind int

const (
	// FixtureNone is a normal run, not a fixture.
	FixtureNone FixtureKind = iota
	// FixtureDefect predicts a product failure, as on an execution client whose
	// start script cannot repair a zero-byte secret.
	FixtureDefect
	// FixtureRepair predicts the stack recovers, as on a client whose start
	// script rewrites an empty secret.
	FixtureRepair
)

// ExitCode maps a run's result to a process exit status.
func ExitCode(r Result) int {
	if r.Outcome == OutcomePass {
		return ExitOK
	}
	switch r.FailureClass {
	case ClassInfrastructure:
		return ExitInfrastructure
	case ClassHarness:
		return ExitHarness
	case ClassTimeout:
		return ExitTimeout
	default:
		return ExitProduct
	}
}

// FixtureExitCode maps a fixture's result to a process exit status.
//
// The semantics are inverted relative to a normal run: zero means the fixture
// observed what it predicted. A fixture that exits zero because its regression
// stopped reproducing would be a green light that means nothing.
func FixtureExitCode(r Result, kind FixtureKind) int {
	switch kind {
	case FixtureDefect:
		// The prediction is a product failure. Anything else — including a pass
		// — means the harness did not observe what it came to observe.
		if r.Outcome == OutcomeFail && r.FailureClass == ClassProduct {
			return ExitOK
		}
		if r.Outcome == OutcomeFail {
			// An infrastructure, harness or timeout failure says nothing about
			// the defect, so it is reported on its own terms.
			return ExitCode(r)
		}
		return ExitUnexpected
	case FixtureRepair:
		// The prediction is that the stack recovers.
		if r.Outcome == OutcomePass {
			return ExitOK
		}
		if r.FailureClass == ClassProduct {
			return ExitUnexpected
		}
		return ExitCode(r)
	default:
		return ExitCode(r)
	}
}
