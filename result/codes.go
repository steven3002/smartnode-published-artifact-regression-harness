package result

// FailureClass categorizes the type of failure.
type FailureClass string

const (
	ClassSuccess        FailureClass = "SUCCESS"
	ClassProduct        FailureClass = "PRODUCT"
	ClassInfrastructure FailureClass = "INFRASTRUCTURE"
	ClassHarness        FailureClass = "HARNESS"
	ClassTimeout        FailureClass = "TIMEOUT"
)

// FixtureKind distinguishes fixtures that expect a product failure from those
// that expect a successful repair. The Geth empty-JWT fixture expects failure
// (the bug is unrecoverable); the Besu empty-JWT fixture expects repair
// (start-ec.sh:303 regenerates the secret).
type FixtureKind int

const (
	FixtureNone   FixtureKind = iota // not a fixture
	FixtureDefect                    // expects ClassProduct (observed product failure)
	FixtureRepair                    // expects ClassSuccess (repair confirmed)
)

// ExitCode maps a failure class to a distinct exit code.
// For fixture runs, 0 means "the expected failure was observed".
func ExitCode(class FailureClass, isFixture bool) int {
	if isFixture {
		if class == ClassProduct {
			return 0 // Expected failure observed
		}
		if class == ClassSuccess {
			return 1 // Harness failed to observe expected failure
		}
	} else if class == ClassSuccess {
		return 0
	}
	switch class {
	case ClassProduct:
		return 1
	case ClassInfrastructure:
		return 2
	case ClassHarness:
		return 3
	case ClassTimeout:
		return 4
	}
	return 1
}

// FixtureExitCode maps outcome to exit code using the fixture's expected kind.
// Exit 0 means the expected outcome was observed:
//   - FixtureDefect: ClassProduct is expected (failure observed)
//   - FixtureRepair: ClassSuccess is expected (repair confirmed)
//   - FixtureNone: delegates to regular ExitCode
func FixtureExitCode(class FailureClass, kind FixtureKind) int {
	switch kind {
	case FixtureDefect:
		return ExitCode(class, true)
	case FixtureRepair:
		if class == ClassSuccess {
			return 0
		}
		if class == ClassProduct {
			return 1 // Expected repair but got failure
		}
		return ExitCode(class, false)
	default:
		return ExitCode(class, false)
	}
}
