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
