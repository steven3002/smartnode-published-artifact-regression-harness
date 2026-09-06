package result

import (
	"encoding/json"
	"fmt"
)

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

func (fc *FailureClass) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch FailureClass(s) {
	case ClassSuccess, ClassProduct, ClassInfrastructure, ClassHarness, ClassTimeout, "":
		*fc = FailureClass(s)
		return nil
	default:
		return fmt.Errorf("invalid failure_class: %q", s)
	}
}
