package result

import (
	"encoding/json"
	"fmt"
)

// UnmarshalJSON rejects any outcome outside the enum, so a typo in a stored
// report fails loudly rather than being read back as a valid state.
func (o *Outcome) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch Outcome(s) {
	case OutcomePass, OutcomeFail:
		*o = Outcome(s)
		return nil
	default:
		return fmt.Errorf("invalid outcome: %q", s)
	}
}

// UnmarshalJSON rejects any class outside the enum. The empty string is
// accepted because a passing run has no failure to classify.
func (fc *FailureClass) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch FailureClass(s) {
	case ClassProduct, ClassInfrastructure, ClassHarness, ClassTimeout, "":
		*fc = FailureClass(s)
		return nil
	default:
		return fmt.Errorf("invalid failure_class: %q", s)
	}
}
