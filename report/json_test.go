package report

import (
	"testing"

	"github.com/rocket-pool/smartnode/rp-regress/result"
)

func TestJSONRoundTrip(t *testing.T) {
	orig := &result.Report{
		ArtifactIdentity: "sha256:1234",
		ImageIdentities:  map[string]string{"besu": "sha256:abcd"},
		Profile:          "besu",
		Network:          "hoodi",
		ReproductionCmd:  "rp-regress run --profile besu",
		Result: result.Result{
			Outcome:      result.OutcomeFail,
			FailureClass: result.ClassProduct,
			KnownIssue:   "issue-123",
		},
	}

	data, err := ToJSON(orig)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	parsed, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if parsed.ArtifactIdentity != orig.ArtifactIdentity {
		t.Errorf("Mismatch in ArtifactIdentity")
	}
	if parsed.Result.Outcome != orig.Result.Outcome {
		t.Errorf("Mismatch in Outcome: got %v want %v", parsed.Result.Outcome, orig.Result.Outcome)
	}
}

func TestJSONRejectsInvalidEnums(t *testing.T) {
	badOutcomeJSON := []byte(`{"result": {"outcome": "MAYBE"}}`)
	_, err := FromJSON(badOutcomeJSON)
	if err == nil {
		t.Error("Expected error for invalid outcome MAYBE, got nil")
	}

	badClassJSON := []byte(`{"result": {"outcome": "FAIL", "failure_class": "UNKNOWN_CLASS"}}`)
	_, err = FromJSON(badClassJSON)
	if err == nil {
		t.Error("Expected error for invalid failure_class UNKNOWN_CLASS, got nil")
	}

	knownRegressionJSON := []byte(`{"result": {"outcome": "KNOWN_REGRESSION"}}`)
	_, err = FromJSON(knownRegressionJSON)
	if err == nil {
		t.Error("Expected error for KNOWN_REGRESSION as outcome, got nil")
	}
}
